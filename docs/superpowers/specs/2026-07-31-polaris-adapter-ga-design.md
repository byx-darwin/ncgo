# 设计：Polaris Canary Adapter GA 加固

> **类型**：设计文档（brainstorming 交付物）
> **Workflow**：wf-2026-07-31-002 · **Phase**：1 (Clarification) · **Mode**：full
> **Issue**：[#34](https://github.com/byx-darwin/ncgo/issues/34)
> **前置**：MVP 已合入（PR #33，Closes #32，commit eceda71）；多角色分析 `docs/superpowers/specs/2026-07-31-real-sdk-adapter-multirole-analysis.md`

---

## 1. 目标与范围

让 opt-in `ncgo add infra polaris_adapter`（Polaris-first、kitex-only）达到**可承接真实生产流量**的可靠性与可运维性，满足 SRE/Security 角色在 GO-WITH-CONDITIONS 时提出的 GA 前置条件，**且不改变** opt-in / SDK-neutral 核心 / kitex-only 的既定边界。

### 锁定的交付范围（用户拍板，2026-07-31）

- **全部 5 条 AC** 在一个 plan / PR 中交付：
  1. 客户端缓存 + TTL（stale-while-revalidate + 刷新抖动，可配置）
  2. 显式 discovery-error fail 语义（fail-open / fail-fast，可配置、可测试）
  3. 决策级可观测性（结构化日志 + metrics，接入既有 OTel）
  4. dry-run / shadow 模式
  5. 运行时 canary harness（假实例，不依赖活 Polaris）
- **resolver 可见性验证先行**，作为设计输入（见 §3，已完成初步验证）。

### 三个架构决策（用户拍板）

| 决策点 | 选择 |
|---|---|
| GA 装饰器落点 | **新增独立 SDK-neutral 文件** `release_ops.go`（`package release`，仅 stdlib）；canonical `release_canary.go` 保持字节级不变（沿用 PR#33 不变式） |
| 可观测依赖策略 | **stdlib `Observer` 接口 + slog 结构化日志在 ops 文件**；OTel/Prometheus 实现放 opt-in adapter（复用生成项目既有 go-framework OTLP） |
| 范围 / 前置验证 | 全部 5 条 AC + resolver 可见性验证先行 |

### Out of Scope

- Nacos adapter（另开 Issue）。
- 改变 opt-in / kitex-only / SDK-neutral 核心边界。
- 活 Polaris 集成测试（CI 仅编译级 + 单测 + 假实例 harness）。

---

## 2. 现状（已核实）

| 资产 | 路径 | 说明 |
|---|---|---|
| canonical seam | `internal/assets/_data/optional/release_canary.go` | SDK-neutral **且零外部依赖**（仅 stdlib）：`Discoverer`/`RuleProvider`/`Selector`/`Select`/`Decide`/`SplitInstances`/`SelectInstance`、`Static*` 测试替身。PR#33 保持字节级不变。**契约敏感**，emit 入下游项目。 |
| kitex canary 适配 | `internal/assets/_data/kitex/optional/release_canary.go` | `KitexCanaryLoadBalancer` + `kitexCanaryPicker`；`KitexResultDiscoverer` 把 `discovery.Result` 映射为 `release.Instance`。 |
| MVP polaris adapter | `internal/assets/_data/kitex/optional/polaris_canary_adapter.go` | `package release`，唯一 SDK 耦合面；`polarisAPI` 接口隔离所有 polaris-go 调用；`sdkClient.listInstances` 用 **`GetAllInstances`（全量）**；env-only 凭证。 |
| 可观测（生成项目） | go-framework OTLP（OTel，kitex base 内置）+ `go-common/log`（slog 分类） | 无独立 Prometheus add-on。 |
| harness 范式 | `internal/scaffold/test/ratelimit/`（run/seed/e2e/attack + tests） | vegeta/grpcurl + docker compose。 |
| 编译级门槛 | `scripts/verify-polaris-adapter.sh` → `tools/verifyexamples/polaris-adapter/` | 拷贝 asset 进独立模块 `go build`，钉 polaris-go。 |
| infra 注册 | `internal/scaffold/infra/infra.go` | `KindPolarisAdapter`（kitex-only，`goGetDeps`: polaris-go + yaml.v3 + go-common）。 |
| 文档边界 | `internal/assets/_data/docs/kitex/design-doc.{en,zh-CN}.md` | 契约敏感。 |

---

## 3. 前置验证：Kitex resolver 实例可见性（已完成，结论改变设计）

### 结论（从源码核实）

polaris-go `api/consumer.go` 接口注释（v1.2.0-beta / v1.7.1 一致）：

| API | 语义（源码原文） |
|---|---|
| `GetInstances` | 「获取可用的服务列表（**会执行路由链**，默认去掉隔离以及不健康的服务实例）」→ **路由过滤后的子集** |
| `GetAllInstances` | 「获取**完整**的服务列表（包括隔离及不健康的服务实例）」→ **全量** |

`kitex-contrib/polaris@v0.0.0-20220811095956` 的 `polarisResolver.Resolve()` 调用 **`consumer.GetInstances(...)`** → 返回**路由过滤后**的实例池。

### 影响

当前 kitex canary LB（`kitexCanaryPicker`）通过 `KitexResultDiscoverer{Result: p.result}` 从 **resolver 的 `discovery.Result`** 构建 canary 池。若 Polaris 配置了按 `release.track`（或任意 metadata）过滤的路由规则，**canary 池可能从 LB 视野中消失，canary 分流静默失败**。这正是架构角色 Top 风险，**已证实为真**。

MVP adapter 自身的 `PolarisDiscoverer` 正确使用 `GetAllInstances`（全量），是唯一可靠的全量来源。

### 验证证据（2026-07-31 复核，可重跑）

```text
$ grep -n "获取可用的服务列表\|获取完整的服务列表" "$(go env GOMODCACHE)/github.com/polarismesh/polaris-go@*/api/consumer.go"
100:	// GetInstances 获取可用的服务列表（会执行路由链，默认去掉隔离以及不健康的服务实例）
102:	// GetAllInstances 获取完整的服务列表（包括隔离及不健康的服务实例）

$ grep -n "GetInstances(getInstances)" "$(go env GOMODCACHE)/github.com/kitex-contrib/polaris@*/resolver.go"
157:	InstanceResp, err := pr.consumer.GetInstances(getInstances)   # Resolve() 主路径
```

结论：kitex resolver 的 `Resolve()`（kitex LB `discovery.Result` 的来源）走 `GetInstances`（路由过滤子集）。canary LB 必须基于 adapter 全量 `Discoverer`（`GetAllInstances`），与 resolver 过滤结果解耦（见 §6）。该语义在 polaris-go v1.2.0-beta 与 v1.7.1 一致。

### 设计决议

canary 路由必须基于 **adapter 的全量 `Discoverer`（GetAllInstances-backed）**，与 kitex resolver 的过滤结果**解耦**（见 §6 LB 改造）。该结论作为硬约束输入 Phase 2 plan 与 Phase 3 实现；Phase 3 首个任务仍保留一次正式验证（含对 polaris-go v1.7.1 的复核与文档化），以关闭 Issue 的前置勾选项。

---

## 4. 架构总览 — 装饰器分层

新增**一个 SDK-neutral 文件** `internal/assets/_data/optional/release_ops.go`（`package release`，**仅 stdlib**），与 `release_canary.go` 同包同目录，随 `add infra canary` / `polaris_adapter` 一并 emit。canonical `release_canary.go` **字节级不变**。

GA 关注点全部实现为**装饰器**，包裹既有 seam，可被未来 Nacos adapter 复用、可无 SDK 单测：

```
PolarisDiscoverer (SDK, GetAllInstances 全量)         ← MVP 已有
  └─ CachingDiscoverer (TTL + SWR + jitter + single-flight)  ← AC1
       └─ FailPolicy (fail-open / fail-fast)                 ← AC2
            └─ Engine.Select (决策 + 观测 + dry-run)          ← AC3 / AC4
```

**设计原则**：每个装饰器单一职责、接口清晰、可独立测试；零外部依赖保持核心 dependency-neutral；OTel 等重依赖只在 opt-in adapter 面出现。

---

## 5. 组件明细（release_ops.go，stdlib only）

| 组件 | 职责 | 关键设计 |
|------|------|----------|
| `CacheOptions` | 缓存配置 | `TTL`（默认 30s）、`StaleTTL`（默认 5min）、`Jitter`（默认 ±20%）、`Now func() time.Time`（可注入，便于测试） |
| `CachingDiscoverer` | 包裹 `Discoverer` | 按 serviceName 本地缓存；**stale-while-revalidate**（过期但在 stale 窗内→返回 stale 并异步/后台刷新）；**抖动刷新**防注册中心重连惊群；`sync` single-flight 防并发重复拉取；刷新失败在 stale 窗内→返回 stale + 记错误 |
| `CachingRuleProvider` | 包裹 `RuleProvider` | 同 caching 模式缓存 `RuleSet` |
| `FailPolicy` | `FailOpen`（默认）/ `FailFast` | FailOpen：discovery/rule 错误时返回 last-known-good（无缓存则空池 → 上游走默认加权 LB，保可用）；FailFast：传播错误拒绝。可配置、路径可测试 |
| `Observer` 接口 | 决策级可观测（stdlib） | `ObserveDecision(Decision, Pools)`、`ObserveDiscovery(service string, n int, err error)`、`ObserveRules(service string, version int, err error)`、`ObserveFallback(reason string)`；`NopObserver` 为默认 |
| `slogObserver` | 结构化日志实现 | 用 `log/slog` 输出决策日志（命中规则 / 选中 track / 池大小 stable·canary·unknown / fallback / discovery·rule error），零新依赖 |
| `Engine` | 组合编排 | 持有 `ServiceName` + `Discoverer` + `Rules` + `Observer` + `DryRun`；调用 canonical `Decide`/`SplitInstances`/`SelectInstance`；集中做决策观测与 dry-run |

### 5.1 AC3 — metrics 接入

- `Observer` 接口定义在 ops 文件（零依赖）；默认 `slogObserver`（结构化日志）+ `NopObserver`。
- **OTel 实现放 adapter**：新增独立文件 `internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go`（opt-in），复用生成项目既有 go-framework OTLP。
- 指标集（命名待 plan 细化，语义固定）：
  - `canary.decision`（counter，labels: `track`/`reason`/`rule`）
  - `canary.pool_size`（gauge，label: `track`=stable/canary/unknown）
  - `canary.fallback`（counter，label: `reason`）
  - `canary.discovery_error` / `canary.rule_error`（counter）

### 5.2 AC2 — fail 语义

- 默认 **FailOpen**（SRE 可用性优先）；可配置 `FailFast`。
- FailOpen 且无 last-known-good → 返回空池，使上游（kitex picker fallback / 默认加权 LB）继续可用，而非拒绝。
- 该路径由单测显式覆盖（注入返回 error 的 fake Discoverer/RuleProvider）。

### 5.3 AC4 — dry-run / shadow

- `Engine.DryRun bool`（构造选项配置）。
- 开启时：照常计算真实 `Decision` + `Selection` 并**观测/记录「将如何路由」**，但**实际返回 stable/default 选路**（不真实路由到 canary）。
- 运维首次下发新 RuleSet 时先 shadow 观察，规则 bug 不波及真实用户。

---

## 6. Kitex LB 改造（源自 §3 验证，必需）

扩展 `kitex/optional/release_canary.go` 的 `KitexCanaryLoadBalancer`，新增**可选**字段：

- `Discoverer release.Discoverer` — 接装饰后的 adapter discoverer（全量，GetAllInstances-backed）。
- `Observer release.Observer` — 决策级观测（可选）。

行为：

- `Discoverer != nil` → picker 用**全量池**选路（经 Engine），再把选中的 `release.Instance` 映射回 kitex endpoint；若该实例不在 resolver `discovery.Result` 中，则**构造**一个 kitex `discovery.Instance`（保证可路由）。
- `Discoverer == nil` → 维持现状（`KitexResultDiscoverer` 从 resolver result 取池），**向后兼容**。

属性：**追加式、向后兼容**，改动在 kitex adapter 面（非 canonical seam），需重录相关 golden。

---

## 7. AC5 — 运行时 canary harness

新增 `internal/scaffold/test/canary/`，仿 `test/ratelimit`（run/seed/e2e + tests），**用假实例、不依赖活 Polaris**：

- 注册 2+ 个带 `release.track` metadata 的实例（`StaticDiscoverer` / 假 `polarisAPI`）。
- 驱动流量经 `Engine`/`Selector`，**断言 stable/canary 分流符合加权规则**（统计大量请求的命中率 ≈ 权重比）。
- 断言 discovery 错误下 **fail-open vs fail-fast** 行为。
- 断言 **dry-run 不真实路由到 canary**（shadow 记录 vs 实际选路）。
- 断言 **缓存 TTL / stale-while-revalidate** 行为（注入 `CacheOptions.Now`）。

现有 `scripts/verify-polaris-adapter.sh` 编译级门槛保留；新增 `polaris_canary_observer_otel.go` 需纳入验证模块编译（verify `go.mod` 增 OTel 依赖）。

---

## 8. 契约 / 测试 / 文档影响

| 面 | 影响 | 测试级别 |
|----|------|----------|
| 新 asset `release_ops.go` | 新 golden（infra-polaris-adapter + infra-canary）、asset parse 测试 | 单测 + golden |
| 改 `kitex/optional/release_canary.go` | golden 重录（追加式、向后兼容） | golden + 集成 |
| 新 asset `polaris_canary_observer_otel.go` | verify 脚本编译它（verify go.mod + OTel） | 编译级 |
| `infra.go` | 若 OTel observer 随 adapter emit，`polaris_adapter` goGetDeps 需加 OTel dep | 集成 |
| 文档 EN+ZH | design-doc canary 节 + troubleshooting（cache/TTL、fail policy、dry-run、resolver-visibility 说明） | markdown 诊断 |
| MCP | 无新 tool；enum 不变（`polaris_adapter` 已在） | — |

### 验证顺序（小→大）

1. 单测：ops 装饰器（注入 `Now` / fake Discoverer·RuleProvider）—— cache/SWR/jitter、fail policy、dry-run、observer。
2. asset parse 测试 + golden（`-update-golden` 刻意重录）。
3. infra 集成测试（`add infra polaris_adapter` / `canary`）。
4. `scripts/verify-polaris-adapter.sh`（编译级，含 OTel observer）。
5. `go build ./... && go build . && go vet ./... && go test ./... -count=1`。
6. `./scripts/smoke.sh`。

---

## 9. 约束复核（沿用 MVP，全部保持）

- ncgo 本体不引入 polaris-go（`release_ops.go` 仅 stdlib，OTel 仅在 opt-in adapter）。
- SDK-neutral 核心 `release_canary.go` 保持 canonical、字节级不变。
- opt-in、kitex-only。
- 凭证仅环境变量（`POLARIS_TOKEN`/`POLARIS_NAMESPACE`），错误/日志绝不泄露（observer 日志禁止输出 token）。
- CI 仍编译级 + 单测；运行时 harness 用假实例。

---

## 10. Plan 阶段需细化的开放项

1. `release_ops.go` 的精确 API 签名与 functional-options 形态。
2. OTel observer 是随 `polaris_adapter` 默认 emit，还是独立子命令/可选文件（影响 goGetDeps 与 golden）。
3. kitex `discovery.Instance` 构造的具体实现（从 `release.Instance` 还原 Address/Weight/Tags）。
4. metrics 命名与 label 基数控制（避免高基数 `rule` label 爆炸）。
5. harness 目录的 vegeta/grpcurl 复用程度 vs 纯 Go 内存驱动。
6. design-doc EN+ZH canary 节的最小必要增量（避免契约文档膨胀）。

---

## 11. 决策记录

| 决策点 | 选择 | 依据 |
|---|---|---|
| 范围 | 全部 5 条 AC + resolver 验证先行 | 用户拍板 |
| 装饰器落点 | 新增独立 SDK-neutral `release_ops.go` | 用户拍板；保 PR#33 不变式、可复用、可无 SDK 单测 |
| 可观测依赖 | stdlib Observer + slog；OTel 放 adapter | 用户拍板；保核心零依赖 |
| fail 默认 | FailOpen | SRE 可用性优先（本设计推荐，plan 确认） |
| LB 全量来源 | adapter `Discoverer`（GetAllInstances），解耦 resolver | §3 源码验证结论 |
