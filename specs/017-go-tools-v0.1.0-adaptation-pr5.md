# 017 — 生成项目适配 go-tools v0.1.0（PR5：observability + registry 适配 go-framework）

- 状态：设计已确认（待实施）
- 日期：2026-07-24
- 关联：Issue #10；工作流 `wf-2026-07-24-002`
- 父设计：`specs/017-go-tools-v0.1.0-adaptation.md`
- 前置：PR1（go.mod 基础 + 错误机制）；建议在 PR3（go-middleware 引入）之后

## 1. 背景与目标

「生成项目全面适配 go-tools v0.1.0」多 PR 分解的 **PR5**，覆盖两类基础设施：

- **observability**：可观测性接入方式
- **registry**：kitex 服务注册 / 发现后端

目标：让生成项目的这两块基础设施与 go-tools v0.1.0 对齐，去除过渡期遗留实现。

## 2. 探索发现（修正 Issue #10 的两处前提）

核对 `go-framework@v0.1.0` 源码与当前模板后，发现 Issue #10 的两处前提与实际代码不符，本设计据实修正：

### 2.1 observability 前提修正

| Issue 前提 | 实际情况 |
|------------|----------|
| 「当前基于 LoongSuite」 | 仅**独立 add-on** `optional/observability_otel.go` 是 LoongSuite。**kitex 基础模板早已用 go-framework OTLP**（`kitex-template/server.yaml` import `go-framework/kitex/observability`，由 `cfg.Jaeger` 驱动，commit `283392e` 已并入 origin/main）。 |
| — | **hertz 基础模板完全没有 observability 接线**（`hertz-template/server_go.yaml` 无任何 trace/otel 代码）。 |

go-framework 同时提供 `kitex/observability` 与 `hertz/observability`：运行时 `NewProvider(ctx, config.ObservabilityConfig)`（OTLP gRPC → collector/Jaeger，W3C + B3 propagator，含 runtime metrics）+ `ServerSuite/ClientSuite`（kitex）/ `ServerMiddleware/ServerTracer`（hertz）+ `Shutdown()`。

### 2.2 registry 前提修正

| Issue 前提 | 实际情况 |
|------------|----------|
| 「registry → go-framework/kitex/option（注册/发现已合并进 option）」 | **`go-framework/kitex/option` 不提供 registry/discovery 构建器**。`option.go` 只构建地址/codec/超时/连接池/重试/负载均衡，完全忽略 `Registry`/`Resolver` 配置。其包注释提到的「Polaris 注册发现 + `//go:build ignore`」是**过时/误导注释**（实测该包可正常编译，`//go:build ignore` 仅出现在注释文字中，无实际 build tag）。 |
| — | go-framework 仅有 registry **配置类型** `config.RegistryOption`（Space/Name/Version/Env/Network/Address，偏 Polaris 风格），无构建器。 |
| — | 现状 `registry_etcd` add-on 用 `kitex-contrib/registry-etcd`，提供 `NewEtcdRegistry/NewEtcdResolver/NewRegistryInfo`，但**未接入基础 server**（需用户手动 `WithRegistry`）。 |

**结论**：registry 无法「迁移到 option」；observability 的 kitex 基础层早已切换。据此重新决策（见 §3）。

### 2.3 可行性验证

- `kitex-contrib/polaris@v0.0.0-20220811095956-d405002eaeaf` 已**实测对 kitex v0.16.1（生成项目所用版本）干净编译并生成二进制**（`go build` exit 0）。MVS 保持 kitex v0.16.1，引入 polaris-go v1.2.0-beta。
- 其 API：`NewPolarisRegistry(ServerOptions, configFile...) → Registry`（扩展 kitex `registry.Registry`）、`NewPolarisResolver(ClientOptions, configFile...) → Resolver`（扩展 kitex `discovery.Resolver`），可分别接入 `server.WithRegistry` / `client.WithResolver`。配置经 `polaris.yaml`（configFile 参数）。
- `internal/mcp/tools.go` 的 `ncgo_add_infra` 用 `enumField("kind", infra.SupportedKinds())` **自动拾取** kind 枚举 —— 增删 kind 自动同步 MCP schema，无需手改枚举（但文档/测试需对齐）。

## 3. 已确认的决策

| 维度 | 决策 |
|------|------|
| observability 路线 | **统一到 go-framework OTLP**。kitex 基础保持现状（已是 OTLP）；hertz 基础补 go-framework OTLP 接线；**完全移除** LoongSuite `observability_otel`/`otel` add-on。 |
| LoongSuite add-on | **完全移除**（从 `SupportedKinds` 删 `observability_otel`/`otel`，删资产与 setup steps，同步 golden/文档/MCP）。`ncgo add infra otel` 将返回 invalid kind（**CLI 契约变更**）。 |
| registry 后端 | **切换到 Polaris**（`kitex-contrib/polaris`），弃用 etcd（`kitex-contrib/registry-etcd`）。 |
| registry 形态 | 新增 `registry_polaris` kind、**移除 `registry_etcd` kind**；经 `ncgo:wire` 锚点把 `WithRegistry`/`WithResolver` 接入 kitex 基础 server/client。 |
| 依赖 | hertz observability 补 `go-framework v0.1.0`（hertz go.mod 已 require，确认即可）；registry 补 `kitex-contrib/polaris`（+ `go-common`）。 |

> **归档方案对照**：`specs/archive/plans/2026-05-11-nacos-polaris-infra-addons.md` 是 v1 旧方案（samber/oops、把 polaris 当**数据层 client wrapper** 放 `internal/base/data/`、配置中心风格），已被 go-tools v0.1.0 重规划取代。本 PR 的 polaris 是 **kitex 注册/发现后端**（放 `internal/base/registry/`、goerror、kitex-contrib/polaris），与旧方案不同，勿混用。

## 4. 设计

### A. 移除 LoongSuite `observability_otel` add-on

- `internal/scaffold/infra/infra.go`：
  - 删常量 `KindObservabilityOtel`、`KindOtelAlias`
  - 从 `SupportedKinds()`、`commonKinds()`、`commonAssetKinds`、`outputRelPaths`、`setupSteps`、`normalizeKind`（alias 分支）清除
- 删资产 `internal/assets/_data/optional/observability_otel.go`
- 删 LoongSuite setup steps（install.sh / `otel go build` / OTEL_* env）
- `ncgo add infra otel` / `observability_otel` 返回 invalid kind
- 相关 golden、文档、MCP 快照同步（MCP enum 自动同步）

### B. hertz 基础 observability（新增，镜像 kitex）

- `hertz-template/conf_go.yaml`：config struct 增 `Jaeger *config.JaegerOption` 段 + `Default()`/`Validate()` 处理
- `hertz-template/conf_dev.yaml`（及 dev conf 样例）：增 `jaeger:` 段（`enable: false`、`endpoint`）
- `hertz-template/server_go.yaml`：
  - import `github.com/byx-darwin/go-tools/go-framework/config`、`hertzobs "github.com/byx-darwin/go-tools/go-framework/hertz/observability"`
  - `cfg.Jaeger != nil && cfg.Jaeger.Enable` 时 `hertzobs.NewProvider(ctx, config.ObservabilityConfig{Enabled, Endpoint, ServiceName})`，`h.Use(provider.ServerMiddleware())`，`defer provider.Shutdown()`（镜像 kitex server.yaml 的写法）
- 确认 hertz 静态 go.mod 已 require `go-framework v0.1.0`（父设计 §5 已钉死）
- **影响 mono golden**（基础模板变更，须 `-update-golden` 并逐 diff 审查）

### C. Polaris registry add-on（替换 etcd）

- `internal/scaffold/infra/infra.go`：
  - 加常量 `KindRegistryPolaris = "registry_polaris"`；删 `KindRegistryEtcd`
  - `SupportedKinds()` / `kitexOnlyKinds()`：`registry_etcd` → `registry_polaris`
  - `outputRelPaths[KindRegistryPolaris] = internal/base/registry/polaris.go`
  - `goGetDeps[KindRegistryPolaris] = {kitex-contrib/polaris, go-common}`（删 etcd 条目）
  - 处理 `polaris.yaml` 作为 add-on 附带文件（`assetFiles` 增第二个 addOnFile，输出到**项目根 `polaris.yaml`**——kitex-contrib/polaris 默认从工作目录读取 `polaris.yaml`）
- 新资产 `internal/assets/_data/kitex/optional/registry_polaris.go`（`package registry`）：
  - `PolarisConfig`（addresses/namespace/protocol/timeout 等）+ `Validate()`（goerror，沿用 `registry_config_invalid` 段）
  - `NewRegistry(cfg) (kitexregistry.Registry, error)` → `polaris.NewPolarisRegistry(...)`
  - `NewResolver(cfg) (discovery.Resolver, error)` → `polaris.NewPolarisResolver(...)`
- 新资产 `internal/assets/_data/kitex/optional/registry_polaris.yaml`（polaris.yaml 模板：server 地址 / namespace）
- 删 `internal/assets/_data/kitex/optional/registry_etcd.go`

### D. registry 接入 kitex 基础（wire）

- `internal/scaffold/infra/wire.go`：
  - `wireSupportedKind` 增 `KindRegistryPolaris`
  - `wireKitex` 增 `KindRegistryPolaris` case：加 import（registry 包 + polaris），在 server opts 处插入 `kitexserver.WithRegistry(registry.NewRegistry(...))`（新锚点 `// ncgo:wire:registry:server`）
  - `wireKitexClient` 增 case：插入 `kitexclient.WithResolver(registry.NewResolver(...))`（新锚点 `// ncgo:wire:registry:client`）
  - 沿用现有幂等插入助手（`insertOnceMarkerOrAnchor*`、`addGoImport*`）
- `kitex-template/server.yaml`：opts 构造处加 `// ncgo:wire:registry:server` 锚点 + 注释提示
- `kitex-template/client.yaml`（生成 `pkg/client/*/client.go`）：加 `// ncgo:wire:registry:client` 锚点
- **影响 kitex golden**

### E. 错误码

沿用 PR1 建立的 goerror + 项目段方案。Polaris 配置校验错误沿用 `registry_config_invalid`（与 etcd 一致）；如需细分，用 middleware 段码并以 `goerror.RegisterHTTPStatuses` 注册 HTTP 映射（避免兜底 500）。具体码值在 Phase 2 计划中钉死。

## 5. 数据 / 配置流

- **observability（hertz）**：conf `jaeger.enable/endpoint` → `hertz/observability.NewProvider(config.ObservabilityConfig{...})` → `h.Use(provider.ServerMiddleware())` → `defer provider.Shutdown()`。kitex 基础已同此模式（不变）。
- **registry（kitex）**：`polaris.yaml`/conf → `registry.NewRegistry(cfg)` → `server.WithRegistry`；client → `registry.NewResolver(cfg)` → `client.WithResolver`。仅在 `ncgo add infra registry_polaris --wire` 后接线（默认 add-on 不带接线，保持 opt-in）。

## 6. 受影响契约面（contract-sensitive）

| 面 | 变更 |
|----|------|
| CLI | `add infra` kind 列表：去 `otel`/`observability_otel`、`registry_etcd` → `registry_polaris` |
| MCP | `ncgo_add_infra` kind enum 自动同步（`SupportedKinds()`）；相关快照/测试对齐 |
| 模板 | hertz `conf_go.yaml`/`conf_dev.yaml`/`server_go.yaml`（observability）；kitex `server.yaml`/`client.yaml`（registry 锚点） |
| add-on 资产 | 删 `optional/observability_otel.go`、`kitex/optional/registry_etcd.go`；增 `kitex/optional/registry_polaris.go` + `.yaml` |
| 生成布局 | hertz 基础含 observability；kitex registry 输出 `internal/base/registry/polaris.go` |
| golden | infra golden（去 otel/etcd、增 polaris）+ mono golden（hertz observability、kitex 锚点） |
| 文档 | design-doc（hertz/kitex 中英）、README 中英、docs/examples 中英 |

## 7. 测试策略

- **单元**：infra kind/`goGetDeps`/`outputRelPaths`/`normalizeKind`（确认 otel/etcd 移除、polaris 加入）；polaris wire 插入幂等性。
- **golden**：`-update-golden` 重生成 infra golden（registry_polaris，删 otel/etcd）+ mono golden（hertz observability、kitex 锚点），**逐 diff 审查**避免误 bless。
- **e2e 编译**（父设计 §6）：生成 hertz（+kitex）项目 → `go mod tidy` → `go build ./...`，验证 hertz observability 与 kitex polaris registry 可编译。需 go 1.26.5 工具链 + 网络（解析 go-tools、kitex-contrib/polaris）。
- **验证链**：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`。

## 8. 文档计划

- design-doc（`internal/assets/_data/docs/{hertz,kitex}/design-doc.{en,zh-CN}.md`）：observability 去 LoongSuite、描述双框架 go-framework OTLP；registry etcd → polaris（含 kitex-contrib/polaris、polaris.yaml、wire 锚点）。
- `README.md`/`README.zh-CN.md`、`docs/examples.md`/`docs/examples.zh-CN.md`：`add infra` kind 列表与示例（去 otel、registry_etcd → registry_polaris）。
- 中英对齐。

## 9. 风险

- **基础模板变更影响 mono/kitex golden**：hertz observability、kitex registry 锚点都会改变 golden，须有意重生成并逐 diff 审查。
- **kitex-contrib/polaris 较老**（2022、钉 polaris-go v1.2.0-beta）：已验证**编译**通过 kitex v0.16.1，但 CI 无 Polaris server，仅能做编译级验证，运行期行为不在本 PR 验证范围。
- **CLI 契约变更**：移除 `otel`/`registry_etcd` kind 会影响既有用户脚本；docs + MCP 快照须同步，smoke 须过。
- **polaris.yaml 是 add-on 新文件类型**：`assetFiles` 目前对多数 kind 只产出一个文件，需支持 registry_polaris 产出 `.go` + `polaris.yaml` 两个文件。
- **e2e 编译依赖网络与工具链**：若 CI 不具备 go 1.26.5 + proxy，先落为本地/可选检查并标注（沿用父设计 §8 处理）。

## 10. 与其他 PR 的关系

- 依赖 PR1（go.mod 基础 + goerror 错误机制）。
- 建议在 PR3（go-middleware 引入）之后，避免 go.mod / 依赖解析冲突。
- PR4（数据类 add-on：kafka/es/clickhouse → go-middleware，Issue #9，PR #14 已合并）已完成，本 PR 沿用其「库替换 + golden 更新」模式。
