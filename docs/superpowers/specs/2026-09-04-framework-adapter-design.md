# FrameworkAdapter 设计（Issue #101）

## Context

`manifest.KindHertz`/`manifest.KindKitex` 目前是硬编码两值枚举，围绕它的 `if`/`switch` 分支散布在
`internal/scaffold/mono/{mono.go,files.go}`、`bff/bff.go`、`rpc/rpc.go`、`infra/{infra.go,wire.go}`、
`shared/container.go` 等文件中。模板目录 `internal/assets/_data/{hertz,kitex}/` 也是并列硬编码，
没有统一的"框架适配器"抽象层。新增第三种框架（如 gin/grpc-go）需要在每处 switch-case 都加分支，
改动面大、遗漏风险高。

本 issue 的目标：抽象一个 `FrameworkAdapter` 接口封装 Hertz/Kitex 的差异点，消除
`mono/bff/rpc/infra/shared` 中重复的 Kind 分支。**本次是纯接口收敛重构**，不新增第三种框架，
不改变生成产物内容。

## 现状调研

通过对 `internal/manifest/manifest.go`、`internal/scaffold/mono/{mono.go,files.go}`、
`internal/scaffold/bff/bff.go`、`internal/scaffold/rpc/rpc.go`、
`internal/scaffold/infra/{infra.go,wire.go}`、`internal/scaffold/shared/container.go` 的调研，
找到的 Kind 分支点按差异类型归类如下：

### (a) 资产/模板路径选择
- `mono/files.go:94` `writeTemplate` → 分发到 `writeHertzTemplate`/`writeKitexTemplate`
- `mono/files.go:527` `overlayTemplatePackage` → Hertz-only 分支
- `mono/files.go:634` `idlNameToken` → Kitex 去横线小写 vs Hertz 小写带横线
- `mono/files.go:651` `writeIDLPlaceholder` → Hertz-only：写 proto 支持文件
- `mono/files.go:706,722` `writeHertzProtoSupportFiles` → 读取 `hertz/openapi/*`、`hertz/validate/validate.proto`，Kitex 无对应
- `mono/files.go:748` `renderIDLPlaceholder` → Kitex/Hertz proto 内容不同
- `infra/infra.go:383` `planHertzConfigWrite` → 读取 `hertz/optional-config/<kind>.yaml`，Hertz-only
- `infra/infra.go:824` `frameworkAssetFiles` → 选择 `hertz/optional/<kind>.go` 或 `kitex/optional/<kind>.go`

### (b) config 结构合并/注入
- `infra/infra.go:376` `planHertzConfigWrite` → 仅 Hertz 合并 YAML config（Kitex 返回 nil, nil）
- `infra/infra.go:582` `nextSteps` → 仅 Hertz + 非空 HertzConfigKey 时追加"检查 conf.yaml"步骤
- `infra/infra.go:679` `planKitexRateLimitConfigWrite` → 仅 Kitex + RateLimit 时合并 kitex conf.yaml 限流块

### (c) wire/DI 代码生成
- `infra/wire.go:52-55` `wire` → 分发 `wireHertz`/`wireKitex`（Kitex 下还有 client/server 子分支）
- `infra/wire.go:83-84` → 本地重复定义 `manifestKindHertz`/`manifestKindKitex` 字符串常量，未复用 `manifest` 包

### (d) docker/dockerfile 生成
- `shared/container.go:251` `loadWorkspaceComposeApps` → Kitex/Hertz 不同宿主机端口段
- `shared/container.go:508,510` `renderServiceDockerConfig` → 分发 `renderHertzDockerConfigBlocks`/`renderKitexDockerConfigBlocks`
- `shared/container.go:594,599` `composeFeaturesForApp` → Hertz-only 开启 nacos+polaris；Hertz+WithDatabase 开启 vegeta
- `shared/container.go:707,709` `servicePort` → Hertz/Kitex 不同容器端口

### (f) IDL/生成器工具调用
- `mono/mono.go:239-242` `defaultIDL` → Kitex `idl/<name>.proto` vs Hertz `idl/app/<name>.proto`
- `mono/mono.go:254-260` `runGenerator` → Kitex → `exec.Kitex(...)`，默认 → `exec.HZ(...)`
- `mono/files.go:634,748,898,939` → `idlNameToken`/`renderIDLPlaceholder`/`generatorCommand`/`requiresSQLCBeforeTidy`
- `mono/mono.go:177,190,197` `Generate` → Kitex-only 预写 go.mod、更新 manifest domains；Hertz-only 模板覆盖回填

### 不在本次收敛范围内
- `manifest.go:138` `Validate` 的 3 路 switch（合法值校验）——这是 Schema 层的"允许枚举值"检查，
  不是生成逻辑分支，且本 issue 明确不新增第三种框架，没必要现在让它动态可扩展。
- `bff/bff.go:95,108`、`rpc/rpc.go:95,108` 的固定 Kind 字面量——bff 恒为 Hertz、rpc 恒为 Kitex，
  这不是"分支"，是各自包的语义决定。

## 方案

### 包位置：`internal/scaffold/framework`

新增包而非放入 `internal/manifest`。理由：所有差异点都是"脚手架生成期"关注点，`internal/manifest`
是 Schema 层。`internal/scaffold/framework` 可以安全 `import internal/manifest`（取
`KindHertz`/`KindKitex` 常量），不会产生循环依赖（`manifest` 不会反向 import `scaffold/framework`）。

### 接口按职责拆分

```go
package framework

type AssetResolver interface {
    OptionalAssetPath(infraKind string) (string, bool)
    HertzConfigAssetPath(infraKind string) (string, bool) // Kitex 实现返回 "", false
}

type ConfigMerger interface {
    MergeServiceConfig(plugin Plugin, ctx MergeContext) (WriteOp, error)
}

type Wirer interface {
    Wire(ctx WireContext) error
}

type ContainerRenderer interface {
    DockerConfigBlocks(ctx DockerContext) (string, error)
    ContainerPort() int
    ComposeFeatures(withDatabase bool) []string
}

type Generator interface {
    IDLPath(opts mono.Options) string
    RunGenerator(ctx context.Context, r exec.Runner, dir string, opts mono.Options, idl string) (exec.Result, error)
    RequiresSQLCBeforeTidy(withDatabase bool) bool
}

type Adapter interface {
    Kind() string
    AssetResolver
    ConfigMerger
    Wirer
    ContainerRenderer
    Generator
}
```

按职责拆分而非单一胖接口，符合 Go 接口隔离惯例：调用方只需依赖自己需要的子接口，
测试时也更容易 mock 单一职责。`Adapter` 作为组合接口供 Registry 使用。

### Registry：与 `internal/scaffold/infra` 的 Plugin 模式一致

```go
package framework

func Register(a Adapter)
func Get(kind string) (Adapter, bool)
func MustGet(kind string) Adapter
```

`hertzAdapter`/`kitexAdapter` 分别在 `adapter_hertz.go`/`adapter_kitex.go` 内通过 `init()` 注册，
与 `infra` 包 `plugin_*.go` 的命名习惯一致（最近一次重构见 commit `2f33e8b`）。

### 迁移范围（7 个文件，纯替换调用点）

| 文件 | 改动 |
|---|---|
| `mono/mono.go` | `defaultKind`/`runGenerator`/`defaultIDL`/`validate` → `framework.Get(kind)` |
| `mono/files.go` | `writeTemplate`/`idlNameToken`/`writeIDLPlaceholder`/`renderIDLPlaceholder`/`requiresSQLCBeforeTidy` → adapter 方法 |
| `infra/infra.go` | `planHertzConfigWrite`/`planKitexRateLimitConfigWrite`/`frameworkAssetFiles` → adapter 方法 |
| `infra/wire.go` | `wireHertz`/`wireKitex` 分支 → `adapter.Wire(...)`；删除重复的本地 Kind 常量 |
| `shared/container.go` | `renderServiceDockerConfig`/`servicePort`/`composeFeaturesForApp` → adapter 方法 |
| `bff/bff.go`、`rpc/rpc.go` | 不涉及分支，不改动 |

`manifest.go` 的 Validate switch 保持不变。

## 验证

- **Golden test 零 diff**：`internal/scaffold/mono/golden_test.go` 全量跑一遍，确认生成产物字节级不变。
- **单元测试**：为 `framework.Register`/`Get`/`MustGet` 补充测试；为每个 adapter 方法对照现有
  mono/infra/container 测试用例的输入输出补充单元测试，确保迁移前后行为一致。
- **回归**：`go build ./... && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`

## 范围声明

本 issue 仅做接口收敛与两个现有实现（Hertz/Kitex）的迁移，**不新增第三种框架**。是否新增
gin/grpc-go 等第三种框架支持，留待后续 issue 单独评估。
