# AI Agent 评估报告

> 评估角色：AI Agent（文档消费者），刚接手一个 ncgo 生成的 Go 微服务项目，需要从 design doc 理解
> `observability_logging` 与 `polaris_adapter` 的使用方法。
>
> 评估对象（提交 `2e5a152`，Closes #62）：
> - `internal/assets/_data/docs/hertz/design-doc.en.md` §`#### Structured Logging`
> - `internal/assets/_data/docs/kitex/design-doc.en.md` §`#### Structured Logging` 与 §`#### Polaris Canary Adapter`
>
> 交叉核对了实际模板源码：`internal/scaffold/infra/infra.go`、`internal/assets/_data/{hertz,kitex,optional}/` 下的
> `observability_logging.go`、`release_canary.go`、`release_ops.go`、`polaris_canary_adapter.go`、`polaris_canary_observer_otel.go`、
> `kitex/kitex-template/main.yaml`。

## 评分（1-5）

- 可发现性: **4/5**
- 可操作性: **3/5**
- 完整性: **3/5**
- 代码示例质量: **3/5**
- 交叉引用: **2/5**

## 优点

- **章节标题清晰、位置合理**。两个文档的 `#### Structured Logging` 都挂在 `3.7 Optional Infra Snippets`
  下，标题即关键字，`grep`/`Ctrl-F` 可直达；`#### Polaris Canary Adapter` 紧邻 `#### Polaris Registry / Discovery`
  与 `#### Release Canary GA Hardening`，构成"注册发现 → 灰度适配 → 生产加固"的叙事链。
- **Structured Logging 章节可操作性最强**。files / middleware / init / config / handler usage / deps
  六个子块齐全；Hertz 版的注册顺序注释（`// extract/generate X-Request-ID...`）和 YAML 配置（含 `file:` 块）
  与模板 `conf.LoggingConfig` 完全一致；`HertzRequestIDFromContext(c)` 的 handler 内取用示例是加分项。
- **依赖声明与模板基本吻合**。`logging` 章节的 Deps（`go-common`、`otel`、`bytedance/gopkg metainfo`）
  与模板 import 一致；共享助手（`WithRequestID`、`WithTrafficLane`、`SinceMS`、8 个 Category 常量 re-export）
  经核对 `optional/observability_logging.go` 全部存在，文档无虚构 API。
- **Polaris 凭据安全边界写得好**。明确"environment only — never hardcode"、`POLARIS_TOKEN`/`POLARIS_NAMESPACE`
  语义、以及"凭证不会出现在日志/metric label"的说明，是给消费方的重要安全契约。
- **自述清晰**。`polaris_adapter` 章节明确说明"ncgo 本身不依赖 polaris-go SDK，该文件是唯一 SDK 耦合面"，
  并给出测试版本（v1.7.1），降低消费者对 API 漂移的担忧。

## 问题

### A. 悬空的 `release.Version`（两个框架、EN/CN 四个文档都有）

初始化示例（hertz `design-doc.en.md:421-425`、kitex `design-doc.en.md:346-350`，zh-CN 同）使用
`Version: release.Version`，但：

1. 示例代码块只 `import goclog "github.com/byx-darwin/go-tools/go-common/log"`，没有 `release` 的 import；
2. 生成的工程里**不存在**提供包级 `Version` 变量的 `release` 包。Kitex base 的 `main.go`
   （`kitex-template/main.yaml:25-33`）初始化用的是 `cfg.Server.Registry.Name` + `cfg.Env`；
   canary add-on 的 `release` 包只有 `ReleaseInfo.Version`（来自 `SERVICE_VERSION` 环境变量），
   没有包级 `Version`。

AI Agent 直接复制该代码块必然编译失败，必须自行发明/猜测 `release` 的来源——这是"可直接复制"契约的破坏点。

### B. Kitex 章节没有说明与 base 模板既有日志初始化的关系

Kitex base 的 `main.go` **已经**调用 `goclog.Init(cfg.Log, ...)`（`kitex-template/main.yaml`）。
文档却把 `logging.InitFromConf(cfg.Log, ...)` 描述为标准初始化方式（"in `main.go` or `server.go`"），
且两者初始化内容重叠（Kitex 版 `InitFromConf` 也只吃 Level/Format/Mode）。Agent 在已有工程上接入时，
会面临"重复初始化 / 该替换 base 哪段"的歧义，文档未给出取舍说明。Hertz base 没有 goclog 初始化
（`hertz/layout.yaml` 无 goclog 引用），所以 Hertz 侧无此问题，但文档也没有利用这个差异。

### C. 文件名引用用的是模板源文件名，不是生成后落盘文件名

- 文档称 canary 接缝在 `release_canary.go`（kitex `design-doc.en.md:395`），实际落盘为
  `internal/base/release/canary.go`（`infra.go:108`）；模板 `polaris_canary_adapter.go` 头注释
  也写 `canary.go`——与文档不一致。
- `#### Release Canary GA Hardening (`release_ops.go`)`（kitex `design-doc.en.md:439`）落盘为
  `internal/base/release/ops.go`（`infra.go:328`）。
- 另：`polaris_adapter` add-on 实际会同时落 `internal/base/release/polaris_observer_otel.go`
  （`infra.go:308`，需 `go.opentelemetry.io/otel/metric`），文档只字未提。

Agent 在生成工程里按文档文件名 `grep` 会找不到目标，需自行推断"模板名 vs 落盘名"的映射。

### D. `NewPolarisSelector` 示例是断头路

`design-doc.en.md:419-434` 展示了构造 `sel, err := release.NewPolarisSelector(...)`，
但**没有展示返回值如何使用**。而紧邻的 "Typical wiring"（`design-doc.en.md:468-478`）走的是另一条路
（`NewPolarisInstanceLister` → `PolarisDiscoverer` → `NewCachingDiscoverer` → `NewKitexCanaryLoadBalancer`），
且其中的 `rulesProvider`、`fallbackLB`、`obs` 三个变量均为未定义占位。两条 API 路径（Selector 高层便捷构造
vs 手动拼 Discoverer/LB）之间的关系、各自适用场景、`Selector.Select` 与 Kitex LB 的衔接，文档都没有说明。
消费者无法判断该照抄哪一段，也补不全缺的变量。

### E. Enable 依赖列表不完整

`polaris_adapter` 的 Enable 块（`design-doc.en.md:413-417`）只列了
`go get github.com/polarismesh/polaris-go` 和 `go get gopkg.in/yaml.v3`，
但 `infra.go:74` 的 KindPolarisAdapter 依赖还包含 `go-tools/go-common`、`go.opentelemetry.io/otel`、
`go.opentelemetry.io/otel/metric`（后者被同包落盘的 observer 文件 import）。observer 模板注释声称
otel 已由 go-framework OTLP 提供，可辩护，但文档未交代这一事实，Agent 会少装/多猜。

### F. 交叉引用薄弱

- `polaris_adapter` 与 `registry_polaris` 共享 Polaris 概念（地址、namespace、polaris.yaml），
  但两节互不引用；adapter 实际通过 `config.NewDefaultConfiguration(addresses)` 自建 SDK 配置、
  不读 `polaris.yaml`，这个"不需要前置装 registry_polaris"的结论是隐含的，Agent 容易误以为有依赖。
- Hertz 与 Kitex 的配置键不对称（Hertz `cfg.Logging` / `logging:`，Kitex `cfg.Log` / `log:`）是真实代码差异，
  文档未以任何"与 Hertz 的差异"提示点出，跨框架维护的 Agent 极易踩坑。
- canary 接缝（`canary.go` 的 `Selector`/`Discoverer`/`RuleProvider` 类型）没有独立章节，
  adapter 章节是唯一入口，但又不解释这些类型的用法。

### G. 次要问题

- Hertz handler 示例（`design-doc.en.md:446-450`）用 `goclog.Biz(ctx)` 但该代码块未给 import
  （可从上方 init 块推断，可接受但不够自足）。
- Hertz 文档存在结构编号倒挂：`## 3.7 Rule-Center Client` 排在 `## 6. hz Invocation Mapping` 之后
  （`design-doc.en.md:508`），是既有问题，但与本次新增章节同文档，拖累导航体验。
- 两个框架的日志章节都缺"常见错误/故障排查"小节（如：忘了先注册 `HertzRequestID()` 导致 access log
  无 request_id；`X-Traffic-Lane` 需 RequestID + TrafficLane 中间件配合才下传）。

## 改进建议

1. **修正 `release.Version`**：改为真实存在的来源并补 import，或加一行说明——"若工程没有 `release` 包，
   用 `os.Getenv("SERVICE_VERSION")`，或沿用 base 模板的 `cfg.Server.Registry.Name` + `cfg.Env`"。
2. **Kitex 日志章节**：加一句"base `main.go` 已调用 `goclog.Init(cfg.Log, ...)`，本 add-on 的
   `InitFromConf` 仅在你想用 add-on 统一接管（含 Category 常量）时替换之"；同时用一行说明为何 Kitex 配置块
   没有 Hertz 的 `add_source`/`file` 字段（`conf.LogConfig` 结构差异）。
3. **统一文件名口径**：给每节补"模板源 → 落盘路径"映射（如 `optional/release_canary.go` → `canary.go`，
   `optional/release_ops.go` → `ops.go`），或在标题中直接用落盘名；`polaris_adapter` 节补一句
   "同次生成 `polaris_observer_otel.go`"。
4. **补齐 wiring 示例的未定义变量**：给出
   `loader, _ := release.NewPolarisRuleLoader(ruleCfg)`、
   `rulesProvider := release.NewCachingRuleProvider(release.PolarisRuleProvider{Config: ruleCfg, LoadRules: loader}, release.CacheOptions{}, release.FailOpen, obs)`、
   `fallbackLB := loadbalance.NewWeightedBalancer()`、`obs := release.NewSlogObserver(slog.Default())`。
5. **打通 Selector 与 LB 两条路径**：一句话说明 `NewPolarisSelector` 返回的 `Selector` 面向
   需要显式决策（`sel.Select(ctx, traffic)`）的场景，而 Kitex LB 侧用实例化 lister/loader 拼装；
   否则删除该死代码示例。
6. **补 Enable 依赖**：注明 otel/otel-metric 已由 go-framework OTLP 提供（与 observer 模板注释对齐），
   或把 `infra.go` 的 GoGet 列表完整列出。
7. **加交叉引用与故障排查**：每节底部加 `Related:` 行（链接 canary 接缝、`registry_polaris`、GA Hardening）；
   在 "Differences from Hertz" 表中补一行 `cfg.Logging/logging:` vs `cfg.Log/log:`。
8. **顺带修 Hertz 编号倒挂**（`3.7 Rule-Center Client` 挪回 3.x 或改编号），这是与本文档同文件的导航瑕疵。

---

## 附：证据索引

| 发现 | 位置 |
|---|---|
| `release.Version` 悬空引用 | hertz `design-doc.en.md:421-425`；kitex `design-doc.en.md:346-350`；zh-CN `hertz:432`、`kitex:331` |
| Kitex base 已有 goclog 初始化 | `internal/assets/_data/kitex/kitex-template/main.yaml:25-33` |
| 落盘名 `canary.go` / `ops.go` / `polaris_observer_otel.go` | `internal/scaffold/infra/infra.go:108, 328, 308` |
| 文档称接缝 `release_canary.go` | kitex `design-doc.en.md:395` |
| Enable 缺 otel 依赖 | `infra.go:74` vs kitex `design-doc.en.md:413-417` |
| `NewPolarisSelector` 无消费示例 | kitex `design-doc.en.md:419-434`；wiring 缺变量 `468-478` |
| `Selector` 类型与 Select 方法 | `optional/release_canary.go:253` |
| adapter 自建 SDK 配置（不读 polaris.yaml） | `kitex/optional/polaris_canary_adapter.go:99-118` |
| Hertz 文档编号倒挂（3.7 在 6 之后） | hertz `design-doc.en.md:508` |
