# 多角色分析：真实 SDK adapter 接入（未来发展方向1）

> **类型**：战略方向多角色分析（brainstorming 交付物）
> **Workflow**：wf-2026-07-31-001 · **Phase**：1 (Clarification) · **Mode**：full
> **方向定义**：将 ncgo 当前 SDK-neutral 的服务发现 / 金丝雀规则加载 callback 接缝，接上真实 Nacos / Polaris SDK，使生成项目能从真实注册中心发现实例并加载 canary 规则（实例携带 `release.track` metadata）。
> **Issue**：待创建 · **状态**：待用户决策

---

## 1. 背景与现状（已核实）

ncgo 已完成 **SDK-neutral** 的 canary/release 模型（roadmap P1「Nacos / Polaris adapter skeleton」已交付）：

- **核心抽象** `internal/assets/_data/optional/release_canary.go`：`Discoverer` / `RuleProvider` / `Selector` / `Instance` / `Traffic` / `RuleSet` / `Pools` / `SelectInstance`，以及 `NacosInstance`/`PolarisInstance` DTO 和 callback 类型（`NacosInstanceLister` / `NacosRuleLoader` / `PolarisRuleLoader`）。**零 SDK 依赖**，文件头注释明确「intentionally SDK-neutral … Wire concrete Nacos/Polaris SDK clients in adapters later」。
- **Kitex 适配** `internal/assets/_data/kitex/optional/release_canary.go`：`KitexCanaryLoadBalancer` + `kitexCanaryPicker` 复用统一 Selector；`KitexResultDiscoverer` 把 `discovery.Result` 映射为 `release.Instance`；`kitexInstanceTags()` 已能读取 `release.track` tag。
- **Polaris registry 骨架** `internal/assets/_data/kitex/optional/registry_polaris.go`：已真正 import `github.com/kitex-contrib/polaris`（钉在 2022 年 commit），但只解决注册/resolver，与 canary `Discoverer` 未连通。这是现成的「opt-in 真实 SDK」范式。
- **依赖管理**：`internal/scaffold/infra/infra.go` `goGetDeps` 所有 optional infra 均为 OPT-IN；canary 当前在 `goGetDeps` 中无条目（零外部依赖）。
- **关键属性**：所有 canary 代码是**嵌入式模板**，会落入每个下游用户项目源码与 `go.mod` —— 属契约敏感面。

竞争方向（同一工程预算）：(b) `--plan` 补丁细度（from/to/insertAfter）；(c) release metadata 注入生成项目。

---

## 2. 五角色分析结论

| 角色 | 裁决 | 核心立场 |
|---|---|---|
| 架构师 Architect | **GO-WITH-CONDITIONS** | 混合策略：SDK-neutral 核心留在模板（稳定契约），真实 SDK 作为 opt-in 子包（`release/nacos`、`release/polaris`）实现核心接口。metadata 通道已打通。 |
| 产品 Product | **HOLD** | 增量价值窄（与 Kitex/Hertz 原生 registry 重叠，仅 canary 这一 niche 是增量）；采用摩擦高（用户须自带 Nacos/Polaris 集群）。建议**先做方向 (b) plan 补丁细度**（价值/工作量最高）。若推进：Polaris-first、opt-in、非默认依赖。 |
| SRE 可靠性 | **GO-WITH-CONDITIONS** | 当前选择逻辑健全，但缺可观测性/缓存/降级语义。GA 前置条件：决策级 metrics+结构化日志、客户端缓存+TTL（禁止每请求同步拉取）、显式 discovery-error fail 语义、dry-run/shadow、运行时 canary 测试 harness。 |
| 维护者 Maintainer | **HOLD（模板内）** / **GO-WITH-CONDITIONS（opt-in 示例）** | 真实 SDK 进模板会引发 golden 全量重录 + 与 kitex-contrib/hertz-contrib 重复维护 + CI 无法跑活注册中心。建议 seam 保持canonical，真实 SDK 作为独立版本化的 opt-in 示例（各自小 `go.mod`），CI 仅编译级验证。 |
| 安全/供应链 Security | **GO-WITH-CONDITIONS** | 真实 SDK 会给每个启用 canary 的项目增加 ~80–130 个传递依赖。强制护栏：严格 OPT-IN、凭证仅从环境变量读取、适配器独立包隔离、版本「tested with vX.Y.Z」文档化、`ncgo doctor` 增加 CVE 扫描。 |

### 各角色 Top 风险汇总

- **架构**：① Kitex resolver 可能在上游已按路由规则过滤实例，导致 canary LB 看不到 canary pool（**最关键约束，需验证 kitex-contrib/polaris 是否透传全量实例**）；② SDK 版本耦合传染下游 go.mod；③ 规则热加载的并发/缓存语义。
- **产品**：① 无明确真实用户需求（可能是「工程自然延伸」假设）；② 与原生 registry 功能重叠；③ 机会成本高于方向 (b)。
- **SRE**：① 注册中心不可用 → 静默降级且无可观测性；② 无缓存/TTL/限流，每请求同步拉取或依赖 SDK 自带缓存（不在 ncgo 契约内）；③ 健康/Enabled 语义完全委托 SDK，metadata 传播延迟可能导致跨版本调用。
- **维护**：① golden 全量重录成本；② 与上游生态重复维护；③ CI 覆盖缺口（仅编译级）。
- **安全**：① 依赖膨胀扩大 CVE 面；② 凭证泄露（日志/错误链/Git）；③ 钉住过时 SDK（polaris-go v1.2.0-beta）的安全/技术债。

---

## 3. 跨角色综合（Synthesis）

### 3.1 强共识（4/5 一致，Product 为战略层 HOLD）

> **真实 SDK 依赖绝不能进入默认模板或 ncgo 本体模块图；必须以 OPT-IN、隔离、独立版本化的方式交付。**

具体一致点：

1. **保留 SDK-neutral seam 作为 canonical 契约**（这是已交付的 P1 资产，不可侵蚀）。
2. **真实 SDK 适配作为 opt-in 胶水**：架构师主张 opt-in 子包（`release/nacos`、`release/polaris`），维护者主张 opt-in 示例文件（仿 `registry_polaris.go`），安全主张独立 `ncgo add infra <provider>_adapter` 命令 —— 三者本质同构：**用户显式启用 + 用户项目拥有 SDK 版本 pin + ncgo 本体零 SDK 依赖**。
3. **CI 仅做编译级验证**（无活注册中心），seam 用 StaticDiscoverer/StaticRuleProvider 单测。
4. **凭证仅从环境变量读取**，错误/日志禁止泄露。

### 3.2 战略分歧（需用户拍板）

- **Product 的优先级挑战**：在同一工程预算下，方向 (b) `--plan` 补丁细度对所有 ncgo 用户（含 AI agent）有乘数效应，价值/工作量明显更高；真实 SDK adapter 窄、重、需用户自带基础设施。→ **是否应推迟本方向，先做 (b)？是否存在真实用户需求？**
- **Nacos vs Polaris 优先级**：产品/维护倾向 Polaris（kitex-contrib 集成更成熟、已有骨架）；架构/安全提到 Nacos 用户基数更大。取决于目标用户注册栈。

### 3.3 推荐路径（综合裁决：GO-WITH-CONDITIONS，且建议降优先级 / 收窄为 opt-in）

若用户决定推进，唯一获得跨角色支持的形态是：

- **Phase A（低风险，高共识）**：不动核心模板。新增 opt-in adapter 胶水（仿 `registry_polaris.go`），实现既有 `NacosInstanceLister`/`PolarisRuleLoader` 等 callback 签名；用户项目各自持有 `go.mod` 与 SDK 版本；CI 加编译级验证脚本；补 EN+ZH 文档与 troubleshooting。
- **Phase B（需 SRE 条件）**：在 adapter 层补可观测性（决策 metrics/日志）、客户端缓存+TTL、discovery-error 显式降级语义、dry-run。
- **Phase C（架构验证）**：验证并解决 Kitex resolver 实例可见性约束（canary LB 是否需绕过 resolver 直接调 adapter `Discoverer`）。

**先决条件（任一不满足则维持 HOLD）**：
1. 有真实用户/团队明确表达需求；或用户明确将其置于方向 (b) 之前。
2. 选定单一 provider 作为首个 MVP（建议 Polaris，复用现有骨架）。
3. 接受「opt-in、非默认依赖、CI 仅编译级」的交付形态。

---

## 4. 待用户决策的开放问题

1. **优先级**：本方向 vs 方向 (b) `--plan` 补丁细度 —— 先做哪个？是否要求先有真实用户需求？
2. **Provider 优先级**：Polaris-first（推荐，骨架已存在）/ Nacos-first / 两者并行？
3. **交付形态**：opt-in 示例文件（`docs/examples/registry/`，零 scaffold 耦合）vs `ncgo add infra <provider>_adapter` 命令（可发现、与 `registry_polaris.go` 一致）vs opt-in 子包？
4. **Kitex LB 实例可见性**：canary LB 是否需看到全量实例（stable+canary），还是接受 resolver 过滤结果？（决定 adapter 落在 resolver 层还是 LB 层）
5. **凭证管理**：第一版仅环境变量是否足够，还是需 Vault/KMS？
6. **CVE/版本治理**：是否在 `ncgo doctor` 增加 go.sum CVE 扫描？是否维护「ncgo × SDK」兼容矩阵？

---

## 5. 结论

**综合裁决：GO-WITH-CONDITIONS（偏保守）/ 战略层 HOLD（Product）。**

技术上方向正确、抽象已预留正确接口；但 (a) 价值窄、机会成本高于方向 (b)，(b) 唯一获支持的形态是「opt-in、隔离、独立版本、CI 编译级」，(c) 存在 Kitex resolver 可见性这一未验证的架构约束。

**建议**：除非用户确认存在真实需求且优先级高于方向 (b)，否则本方向维持 HOLD；若推进，按 §3.3 Phase A→B→C 收窄为 Polaris-first 的 opt-in adapter，并以满足 SRE/Security 条件为 GA 门槛。

---

## 6. 决策记录（2026-07-31，用户拍板）

经多角色分析后，用户做出如下决策，**锁定本 workflow 的实施范围**：

| 决策点 | 选择 | 含义 |
|---|---|---|
| 方向处置 | **推进（opt-in 收窄）** | 不进入默认模板 / ncgo 本体模块图；以满足 SRE/Security 条件为 GA 门槛 |
| 首个 Provider | **Polaris-first** | 首个 MVP 接 `github.com/polarismesh/polaris-go`（采纳架构/产品/维护角色建议：已有 `registry_polaris.go` 骨架与 `kitex-contrib/polaris` 集成，复用成本最低；Nacos 后续单独 Issue） |
| 交付形态 | **`ncgo add infra` 命令** | 新增 `ncgo add infra nacos_adapter`（命名待定），与现有 `goGetDeps` opt-in 模式一致；用户显式启用 + 用户项目持有 SDK 版本 pin |

### 锁定的实施范围（Phase A MVP）

1. **核心契约不变**：`internal/assets/_data/optional/release_canary.go` 的 SDK-neutral seam 保持 canonical，不引入任何 SDK 依赖。
2. **新增 opt-in adapter**：以嵌入式模板提供真实 Polaris adapter 胶水，实现既有 `PolarisInstanceLister` / `PolarisRuleLoader` callback 签名（`release.Discoverer` / `release.RuleProvider`），import `polaris-go`；复用现有 `registry_polaris.go` 骨架与 `kitex-contrib/polaris`。
3. **新增 infra kind**：`internal/scaffold/infra` 增加 polaris-adapter kind（kitex/hertz 适用范围待定），纳入 `goGetDeps`（`go get polaris-go`，opt-in）。
4. **凭证**：仅从环境变量读取（`POLARIS_TOKEN` / `POLARIS_NAMESPACE` 等），禁止硬编码；错误/日志不泄露。
5. **版本**：ncgo 本体零 Polaris SDK 依赖；模板头注释标「tested with polaris-go vX.Y.Z」（注意现有骨架钉在 `polaris-go v1.2.0-beta`，需评估升级）；用户项目持有版本 pin。
6. **CI**：仅编译级验证（无活 Nacos）；seam 继续用 Static* 单测。
7. **文档**：EN+ZH 对齐，含 troubleshooting（凭证、连接失败、metadata 传播）。

### GA 门槛（来自 SRE/Security，后续 Phase B 处理，非本 MVP 阻塞项但需在建 Issue 中记录）

- 决策级 metrics + 结构化日志；客户端缓存 + TTL（禁止每请求同步拉取）；discovery-error 显式降级语义；dry-run/shadow。
- 运行时 canary harness（仿 `internal/scaffold/test/ratelimit`）。

### 遗留待验证（实施前需澄清）

- **Kitex resolver 实例可见性**（架构 Top 风险）：canary LB 是否需看到全量实例？若 kitex-contrib/nacos resolver 已过滤，adapter 需落在 LB 层直调 `Discoverer`。
- infra kind 命名与 hertz/kitex 适用边界。
- `--plan` 输出对该新 kind 的 plan item 设计。
