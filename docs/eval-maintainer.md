# 维护者评估报告

> 评估角色：ncgo 项目维护者（一致性与完整性审查）
>
> 评估对象（分支 `feat/62-infra-docs`，提交 `2e5a152`，Closes #62）：
> - `internal/assets/_data/docs/hertz/design-doc.en.md`
> - `internal/assets/_data/docs/kitex/design-doc.en.md`
>
> 交叉核对了实现与契约面：`internal/scaffold/infra/infra.go`（`SupportedKinds()` / `commonKinds()` /
> `kitexOnlyKinds()` / `rateLimitAssetFiles()`）、`internal/ai/sync.go`（`readDesignDoc()` /
> `listDocSpecs()` / `writeStandaloneDocs()` / `rewriteDocLinks()`）、`internal/ai/render.go`
> （design doc 如何被嵌入 CLAUDE.md/AGENTS.md）、`internal/assets/assets_test.go`、以及
> `internal/assets/_data/{hertz,kitex,optional}/` 下实际的 optional 模板文件。
>
> 对比基线：主分支 `ncgo` 上对应文档，diff 范围仅 design-doc en/zh 各两篇 + `docs/review-report.md`，
> 属 docs-only 变更；`_data/VERSION` 未动（保持 0.1.32）。

## 评分（1-5）

- 覆盖完整性: **3/5**
- ai sync 兼容性: **5/5**
- 项目约定遵循: **3/5**
- 遗漏风险评估: **4/5**（5=无风险）
- 可维护性: **4/5**

## 覆盖状态表

以 `SupportedKinds()` 的 canonical kind 为准（别名映射到 canonical，不单独计）：

| Infra Kind | Hertz 文档 | Kitex 文档 | 状态 |
|---|---|---|---|
| `redis` | ✅ §3.6 详述 | ✅ §3.7 简述 | OK |
| `kafka` | ✅ §3.6 详述 | ✅ §3.7 简述 | OK |
| `es` | ✅ §3.6 详述 | ✅ §3.7 简述 | OK |
| `clickhouse` | ✅ §3.6 详述 | ✅ §3.7 简述 | OK |
| `observability_logging`（别名 `logging`） | ✅ §3.6 新增详述 | ✅ §3.7 新增详述 | OK（本分支交付） |
| `release_canary`（别名 `canary`） | ⚠️ 仅 §7 "Currently shipped" 提及一行，**无专属章节** | ✅ §3.7 详述（含 release_ops.go GA 加固） | **缺口（Hertz）** |
| `registry_polaris` | n/a（kitex-only） | ✅ §3.7 详述 | OK |
| `rate_limit`（别名 `rate-limit`） | n/a（kitex-only） | ❌ **完全未文档化** | **缺口（Kitex，重大）** |
| `polaris_adapter`（别名 `polaris-adapter`） | n/a（kitex-only） | ✅ §3.7 新增详述 | OK（本分支交付） |

别名覆盖情况：`logging` 别名在两个文档中都有明确标注（"alias: `logging`"）；`canary` / `rate-limit` /
`polaris-adapter` 别名不需要单独文档，但 `rate_limit` 本体缺失意味着 `rate-limit` 别名同样无据可查。

## 优点

- **新增内容的事实准确性高**。逐一核对模板源码：
  - `observability_logging` 章节的 `InitFromConf(cfg.Logging / cfg.Log, ...)` 签名与
    `hertz/optional/observability_logging.go`（`conf.LoggingConfig`）和
    `kitex/optional/observability_logging.go`（`conf.LogConfig`）一致；配置块顶层键
    `logging:`（Hertz）与 `log:`（Kitex）与 `hertz/hertz-template/conf_go.yaml`、
    `kitex/kitex-template/conf.yaml` 一致；共享助手（`WithRequestID` / `WithTrafficLane` /
    `SinceMS` / 8 个 Category 常量）与 `optional/observability_logging.go` 逐项吻合。
  - `polaris_adapter` 章节的 `NewPolarisInstanceLister` / `NewPolarisRuleLoader` /
    `NewPolarisSelector`、`POLARIS_TOKEN` / `POLARIS_NAMESPACE`（env-only）、
    `polaris-go v1.7.1`、`ConsumerAPI` / `ConfigAPI` 均与
    `kitex/optional/polaris_canary_adapter.go` 一致，无虚构 API。
- **章节结构与既有模式一致**。新增章节沿用 Redis/Kafka/ES/ClickHouse 的
  Provides → Deps → Failure codes → Wiring 四段式，AI Agent 可复用既有检索路径。
- **ai sync 兼容性无异常**。design doc 路径与 `readDesignDoc("hertz"|"kitex", "en")` 匹配；
  文档内无 `<!-- ncgo:managed -->` 标记；代码围栏平衡（hertz 16/16、kitex 12/12）；
  `render.go` 以字符串拼接方式原样嵌入 CLAUDE.md/AGENTS.md（不经 Go template），文档中的
  `{{.GoModule}}` / `{{.Module}}` / `{{ToLower x}}` 不会被模板引擎求值，无注入风险。
- **zh-CN 同步完成**。`design-doc.zh-CN.md` 两份均含对应新增章节（`#### 结构化日志`、
  `#### Polaris 金丝雀适配器`），满足 Maintenance Contract 第 2 条的中英对齐要求。

## 风险点

1. **`rate_limit`（kitex-only infra kind）完全未文档化 —— 重大覆盖缺口。**
   `SupportedKinds()` 含 `KindRateLimit`，`kitexOnlyKinds()` 含 `rate_limit`；实现完备
   （`infra.go` 的 `rateLimitAssetFiles()` 落盘 `internal/pkg/ratelimit/{resolver,store}.go` 及测试、
   `kitex/kitex-template/ratelimit_middleware.yaml`，`planKitexRateLimitConfig` 向
   `conf/dev/conf.yaml` 追加 `rate_limit:` 块，`setupSteps` 有 shadow→enforce 上线流程）。
   但 Kitex design doc 仅在配置段提及 `rate_limit.rule.window_seconds`，§3.7 无任何
   rate_limit 小节。设计计划（`docs/superpowers/plans/2026-08-09-issue-62-infra-docs.md`）
   将其列为 Out of Scope，理由是"已在 `rate-limit-dynamic-design.*.md` 中单独记录"——该理由
   **事实不成立**：那份动态限流文档是 Hertz 专属的（Kitex doc 第 11-13 行自述
   "specific to the Hertz HTTP template and does not directly apply to the Kitex RPC
   template"），并不覆盖 kitex 的 `rate_limit` 基础设施 add-on。Acceptance Criteria 中的
   "All `ncgo add infra` supported components have corresponding docs" 因此被过度宣称。

2. **Hertz `release_canary` 无专属章节。** `release_canary` 属于 `commonKinds()`（Hertz 与 Kitex
   通用），Hertz 侧实际落盘 `hertz/optional/release_canary.go` →
   `internal/base/release/hertz.go`（`HertzTraffic` / `TrafficFromHertz` / `HertzDecision`）。
   Hertz doc 只在 §7 "Currently shipped" 列表中出现一次，Agent 无法从文档学到 Hertz 侧的
   注册方式与流量提取规则。设计计划中的覆盖表将 Hertz release_canary 标为 "N/A"，同样是
   事实错误——它不是 N/A，而是 common kind。

3. **新增内容与既有内容产生内部不一致（会导致 Agent 与维护者误读）：**
   - Kitex §6 "Currently shipped" 列表**遗漏 `release_canary`**，而其 §3.7 恰恰详述了
     Release Canary GA Hardening。
   - Kitex §7 "Differences from Hertz" 表格计数过时：仍写 "4 kinds … 5 kinds
     (adds `registry_polaris`)"。实际 Hertz 现支持 6 种（redis/kafka/es/clickhouse/
     observability_logging/release_canary），Kitex 现支持 9 种（上述 6 + registry_polaris/
     rate_limit/polaris_adapter）。
   - 两处 §4 Files 表未收录本分支新增文档化的可选文件：Hertz 表只列
     `{redis,kafka,es,clickhouse}.go`，缺 `observability_logging.go`、`release_canary.go`、
     `redis_shared.go`；Kitex 表只列 `{redis,kafka,es,clickhouse,registry_polaris}.go`，
     缺 `observability_logging.go`、`release_canary.go`、`polaris_canary_adapter.go`、
     `polaris_canary_observer_otel.go`。
   - §3.6 引言 "Data clients drop under `internal/base/data/`" 与 §3.7 引言
     "drops a Go file under `internal/base/{data,registry}/`" 未涵盖新目标包：
     logging → `internal/base/logging/`，canary / polaris_adapter → `internal/base/release/`。

4. **`_data/VERSION` 未随文档变更递增。** 严格按 Maintenance Contract 的字面（"Any change under
   `_data/hertz/`"），该条款针对模板树变更；`_data/docs/` 下的 docs-only 修改是否必须 bump 存在
   歧义。`ai sync` 不做版本比对（materialize 时总是覆写 managed 文件），因此不产生功能异常；
   但若未来在缓存/版本门控上复用 `assets.Version()`，缺失 bump 会引入陈旧内容。

5. **`assets_test.go` 的结构化资产清单未收录部分已存在的 optional 文件。** 现清单只列
   hertz/optional 的 redis/kafka/es/clickhouse 与 kitex/optional 的 registry_polaris*，
   缺 `observability_logging.go`、`release_canary.go`、`polaris_canary_adapter.go`、
   `polaris_canary_observer_otel.go` 等。本分支不新增文件，未触发 Maintenance Contract 第 5 条；
   但文档已将这些文件文档化，未来维护者可能误以为测试已覆盖。

6. **次要：ai sync 物化输出的链接瑕疵（仅影响物化后的独立文档，不影响嵌入 CLAUDE.md 的内容）。**
   - Kitex doc 链接 `../hertz/rate-limit-dynamic-design.en.md`，但 `listDocSpecs("kitex", …)`
     不物化该文档（仅 `profile == hertz` 时物化），故 `docs/ncgo/kitex/design-doc.*.md`
     中的此链接为悬空链接。属既有的 `listDocSpecs` 行为，非本分支引入，但 Kitex 文档新增了对它的引用。
   - `rewriteDocLinks()` 会把文档中 `docs/<profile>/…` 显示文本改写为 `../<profile>/…`
     （Hertz doc 第 9、580、585 行；Kitex doc 第 9、12、609 行），仅影响链接的显示文本，不影响目标路径。

## 改进建议

1. **补 Kitex `rate_limit` 章节（最高优先）。** 在 Kitex design doc §3.7 增加
   `#### Rate Limit (kitex-only)`：列出 `ncgo add infra rate_limit`（别名 `rate-limit`）落盘的
   `internal/pkg/ratelimit/{resolver,store}.go` 与 `conf/dev/conf.yaml` 的 `rate_limit:` 块，
   以及 setupSteps 中的 shadow → enforce 上线流程（与现有 `setupSteps[KindRateLimit]` 对齐）。
   同时修正设计计划 Out of Scope 中"已单独记录"的错误理由。
2. **补 Hertz `release_canary` 章节。** 在 Hertz design doc §3.6 增加
   `#### Release Canary (release_canary.go + hertz.go)`：`HertzTraffic` / `TrafficFromHertz` /
   `HertzDecision` 的注册方式与流量维度（X-Traffic-Lane / X-User-ID / X-Tenant-ID → StickyKey），
   落盘路径 `internal/base/release/hertz.go`。
3. **做一次一致性清扫（本次合并前建议完成）：**
   - Kitex §6 "Currently shipped" 补 `release_canary`；
   - Kitex §7 Differences 表更新为 "6 kinds … 9 kinds"（或改为动态描述，避免再次过时）；
   - 两份 §4 Files 表按实际 optional 目录补齐；
   - §3.6/§3.7 引言的目标包改为 `{data,logging,registry,release}/`。
4. **把文档化的 optional 文件补进 `internal/assets/assets_test.go` 结构化清单**，让文档内容与
   结构性测试互为印证。
5. **为未来维护提供"覆盖清单"钩子。** 在每份 design doc 的 Maintenance Contract 章节加一句：
   "新增 `SupportedKinds()` 条目时必须在本 doc 补对应小节，并同步本表的覆盖状态"，使
   kind → 文档的映射成为可勾选清单；可考虑在 §3.6/§3.7 开头加一行"当前支持 kind 索引"。

## 结论

本分支在计划范围内（`observability_logging` + `polaris_adapter` 中英四文档）**内容准确、
与模板/契约一致、ai sync 兼容无异常**，可直接合入。但从维护者视角看，它交付的只是 issue #62
覆盖矩阵的**子集**：Kitex `rate_limit` 完全缺档、Hertz `release_canary` 只有一行提及，
且新增章节暴露了多处既有表格/列表的过时计数。建议合入前补齐上述两项缺口并做一致性清扫，
否则文档作为"所有 infra 组件的权威入口"仍然不成立。
