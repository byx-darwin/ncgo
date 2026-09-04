# infra Plugin 注册接口重构设计

- Issue: [#100](https://github.com/byx-darwin/ncgo/issues/100)
- Workflow: `wf-2026-09-04-005`
- Date: 2026-09-04

## Context

`internal/scaffold/infra/infra.go`（约 848 行）用 `Kind` 常量 + map（`goGetDeps`/`setupSteps`/`outputRelPaths`/`hertzConfigSnippetKeys`/`commonAssetKinds`）+ switch（`assetFiles`/`assetPath`/`frameworkAdapterName`）组织各 infra 插件；`internal/scaffold/infra/wire.go`（约 530 行）另有三处 per-kind switch（`wireHertz`/`wireKitex` server 分支/`wireKitexClient`）实现 `--wire` 场景下把生成代码的 marker/anchor 拼接到启动文件中。新增一个 infra kind 目前平均要改 5-6 个独立编辑点，是明显的耦合热点。

范围确认（brainstorming 阶段与用户对齐）：
- **纳入**：`infra.go` 内全部 switch/map 组织逻辑 + `wire.go` 的三处 per-kind switch。
- 目标是纯重构：外部行为、生成产物内容、公开 API 签名全部不变，golden test 零 diff。

## Goals / Non-Goals

**Goals**
- 用接口式 `Plugin` 把现有 12 个 infra kind（含别名共 8 个 canonical kind）的元数据和行为收敛到各自独立文件。
- 新增一个插件的改动面从"infra.go/wire.go 内 5-6 处独立编辑点"降到"实现一个 `Plugin` 类型 + 注册一次"。
- `infra.KindXxx` 常量、`SupportedKinds()`、`Add()`/`Wire()`/`PreviewWire()`/`PreviewWirePlan()`、`Options`/`Result`/`PlanItem` 等被 `internal/cli`、`internal/mcp`、`internal/orchestrator`、`internal/scaffold/mono` 依赖的导出符号签名保持不变。
- `internal/scaffold/infra/testdata/` 下全部 golden 输出零 diff；`infra_test.go`/`golden_test.go`/`render_test.go` 全部通过，无需修改断言内容（除非测试直接引用了被移除的私有 helper，如 `commonKinds()`，那部分改为等价的动态过滤，行为不变）。

**Non-Goals**
- 不引入外部（包外）插件注册能力；`Plugin` 仍是包内已知的封闭集合，不做插件动态加载。
- 不改变任何生成模板文件的内容（`internal/assets/_data/**`）。
- 不改变 CLI/MCP 层面对 infra 的调用方式或输出文案。

## Design

### Plugin 接口

```go
// Plugin describes one `ncgo add infra <kind>` add-on. Concrete plugin
// types live in plugin_<kind>.go and self-register via Register() in an
// init() function.
type Plugin interface {
	Kind() string
	Aliases() []string
	// ServiceScope reports which manifest.Kind values this plugin supports:
	// "common" (both), "hertz", or "kitex".
	ServiceScope() string
	GoGetDeps() []string
	// SetupSteps returns an explicit next-steps override, or nil to derive
	// the default (go get GoGetDeps + hertz config note + "go mod tidy").
	SetupSteps() []string
	// HertzConfigKey returns the conf/dev/conf.yaml top-level key this
	// plugin's Hertz config snippet writes under, or "" if it has none.
	HertzConfigKey() string
	AssetFiles(serviceKind string) ([]addOnFile, error)
}
```

可选能力通过类型断言检测（未实现 = 该 hook 不适用，等价于现有 switch 里没有对应 case）：

```go
type extraFilesPlugin interface {
	// ExtraFiles returns additional files to append after AssetFiles,
	// conditioned on project state (e.g. redis's Hertz shared helper is
	// only added if missing). Only redis implements this today.
	ExtraFiles(root, serviceKind string) ([]addOnFile, error)
}

type hertzServerWirer interface {
	WireHertzServer(src, module string, plan *[]PlanItem) (string, error)
}

type kitexServerWirer interface {
	WireKitexServer(src, module string, plan *[]PlanItem) (string, error)
}

type kitexClientWirer interface {
	WireKitexClient(src, module string, plan *[]PlanItem) (string, error)
}
```

`WireXxx` 方法签名对应现有 `wireHertz`/`wireKitex`/`wireKitexClient` switch 分支体：输入当前源码字符串 + module，返回替换后的字符串；通过 `plan` 指针追加 `PlanItem`（复用现有 `wirePlan`/`wirePlanWithAnchor` 等 helper，不变）。

### 注册表

```go
// Register adds a plugin to the registry. Called from each plugin file's
// init(). Panics on duplicate Kind() — the plugin set is closed and known
// at compile time, so a collision is a programming error, not a runtime
// condition to recover from.
func Register(p Plugin) { ... }

var registry = map[string]Plugin{}   // Kind() -> Plugin
var aliasOf = map[string]string{}    // alias -> canonical Kind()
```

`SupportedKinds()` 的输出顺序**不依赖 `init()` 执行顺序**（虽然 Go 对同包多文件的 `init()` 顺序在实践中是确定的——按文件名字典序——但把展示顺序绑定到这个隐式顺序上很脆弱），而是维持一份独立的 `kindOrder []string` 常量表，逐字保留现有 `SupportedKinds()` 的返回顺序（含别名穿插位置）：

```go
var kindOrder = []string{
	KindRedis, KindKafka, KindES, KindClickHouse,
	KindObservabilityLog, KindLoggingAlias,
	KindReleaseCanary, KindCanaryAlias,
	KindRegistryPolaris,
	KindRateLimit, KindRateLimitAlias,
	KindPolarisAdapter, KindPolarisAdapterAlias,
}

func SupportedKinds() []string { return append([]string(nil), kindOrder...) }
```

`normalizeKind` 改为查 `aliasOf`/`registry` 而非硬编码 `if kind == KindLoggingAlias { ... }` 分支链；`commonKinds()`（现被 `infra_test.go` 用于遍历 6 个 common asset kind）改为从 `registry` 按 `ServiceScope() == "common"` 动态过滤，行为等价。

`wireSupportedKind(kind)` / `unsupportedWireError()` 改为：解析出 canonical kind → 查 `registry[kind]` → 用类型断言检查是否实现任一 `*Wirer` 接口；`unsupportedWireError` 的提示文案（列出支持 `--wire` 的 kind）动态遍历 `kindOrder` 中实现了任一 wirer 接口的 kind 生成，**文案内容需与现状字节级一致**（当前硬编码 `"infra: --wire is only supported for %s/%s/%s/%s"`，四个 kind 顺序为 `KindObservabilityLog/KindReleaseCanary/KindRegistryPolaris/KindRateLimit`）。

### 文件拆分

```
internal/scaffold/infra/
  infra.go                         # Add/Options/Result/PlanItem, Plugin 接口定义,
                                    # 注册表, normalizeKind, nextSteps,
                                    # planHertzConfigWrite（保留为通用 helper，
                                    # 按 plugin.HertzConfigKey() 驱动）
  wire.go                          # 通用原语不变；Wire/PreviewWire/PreviewWirePlan
                                    # 调度改为查插件 + 接口断言
  plugin_redis.go                  # + ExtraFiles（Hertz 共享 helper）
  plugin_kafka.go
  plugin_es.go
  plugin_clickhouse.go
  plugin_observability_logging.go  # + 三个 wire hook
  plugin_release_canary.go         # + 三个 wire hook
  plugin_registry_polaris.go       # kitex-only, + server/client wire hook
  plugin_rate_limit.go             # kitex-only, 自定义 AssetFiles
                                    # （含现有 rateLimitAssetFiles/conf.yaml 合并逻辑原样迁移）
                                    # + server wire hook
  plugin_polaris_adapter.go        # kitex-only
  render.go / golden_test.go / infra_test.go / render_test.go  # 不变
```

四个简单插件（redis/kafka/es/clickhouse）的 `AssetFiles` 复用一个共享 helper（对应现有 `assetPath()` 的 hertz/kitex 目录选择逻辑），避免重复样板：

```go
func frameworkAssetFiles(infraKind, outputRelPath string) func(serviceKind string) ([]addOnFile, error) {
	return func(serviceKind string) ([]addOnFile, error) {
		srcPath, err := frameworkAssetPath(infraKind, serviceKind) // hertz/kitex 目录选择, 不变
		if err != nil {
			return nil, err
		}
		return []addOnFile{{SourcePath: srcPath, OutputRelPath: outputRelPath}}, nil
	}
}
```

### 数据流（不变）

`Add()` 的整体流程不变：`normalizeKind` → 查插件 → `plugin.AssetFiles(serviceKind)` → 若实现 `extraFilesPlugin` 则追加 → `planHertzConfigWrite`（按 `plugin.HertzConfigKey()`）→ `planKitexRateLimitConfigWrite`（rate_limit 专属，保留为独立函数，不纳入 Plugin 接口——它不是"资产文件"而是 conf.yaml 合并，且只有一个 kind 用到，抽象成通用 hook 无收益）→ 写文件 → 更新 manifest → `Wire()`（若 `opts.Wire`）→ `nextSteps()`（按 `plugin.SetupSteps()`/`GoGetDeps()`)。

### 错误处理

- 插件重复注册（同 `Kind()` 或同 alias）→ `init()` 期间 `panic`，fail-fast，属编译期已知的编程错误，不需要运行时恢复路径。
- `normalizeKind` 找不到匹配 kind/alias → 保留现有报错文案 `infra: kind %q is invalid; want one of %v`。
- `AssetFiles` 内部的 "kind 仅支持 kitex 服务" 类错误保留在各插件实现内部，文案与现状逐字一致。

## Testing

- **单元测试**：不新增测试文件；现有 `infra_test.go` 逐条覆盖每个 kind 的 `Add`/`--wire`/`--force`/错误路径，迁移后应无需修改断言（仅 `commonKinds()` 私有 helper 的实现方式变化，签名和返回值不变）。
- **Golden 测试**：`golden_test.go` 覆盖的生成产物必须零 diff；迁移过程中每完成一个插件文件的拆分，跑一次 `go test ./internal/scaffold/infra/... -count=1` 做增量验证。
- **回归重点**：`wire.go` 三处 switch 迁移是本次改动风险最高的部分——原代码依赖具体的 marker/anchor 常量字符串（如 `markerLoggingInit`、`insertAfterMarkerOrAnyWithPlan` 的 anchor 列表顺序），这些常量和调用参数必须逐分支原样保留，只改变"由谁调用"（从 `wireHertz` 内联 switch case 改为 `plugin.WireHertzServer` 方法体）。

## Risks

- 接口式设计对简单插件有方法样板代码（`SetupSteps()`/`HertzConfigKey()` 等对多数插件只是返回 nil/""），可接受，是本次选定方案（接口式 Plugin）的已知取舍。
- 文件数从 2 个（`infra.go`/`wire.go`）增加到约 11 个，属预期的组织性变化，不影响外部行为。
