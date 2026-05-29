# ncgo Hertz i18n Hybrid 方案设计

- 状态：Draft v1
- 适用范围：`ncgo` Hertz 模板
- 目标：在保留 `locales/*.json + make i18n` 的基础上，引入 Agent + 工具链协作的 Hybrid 方案

## 1. 背景

当前 Hertz 模板已经具备以下能力：

- 使用 `internal/pkg/i18n/locales/*.json` 管理多语言文案
- 使用 `make i18n` 生成 `internal/pkg/i18n/catalog_gen.go`
- 使用 `internal/pkg/i18n/i18n.go` 完成语言注册、归一化与 `Accept-Language` 协商
- 默认语言与扩展语言统一由 locale JSON 驱动

随着业务消息增多，单纯手工维护多语言会出现以下问题：

- 新增消息 key 后，需要手工同步到多个语言文件
- 容易漏 key、漏翻译、漏更新
- 错误消息通常较短，纯机器翻译缺少上下文，质量不稳定
- 希望利用 AI code agent 提高翻译效率，但不希望把不确定的生成式行为直接耦合进 CI 或 `make generate`

## 2. 设计目标

本方案希望实现：

- 自动发现新增消息 key
- 自动同步所有 locale 的 key 集合
- 通过 Agent 对缺失项进行智能补译
- 通过工具链完成结构治理、状态治理与生成治理
- 保持 CI 可重复、可审计、可稳定运行
- 不破坏现有 `locales/*.json + make i18n` 架构

本方案不追求：

- 运行时动态翻译
- 无人值守全自动 AI 翻译
- 在 CI 中依赖外部模型或翻译 API
- 不经 review 即默认机器生成文案为最终正式译文

## 3. 总体设计

采用 Hybrid 模式，将 i18n 流程拆为两层：

- **确定性层**：由仓库内工具负责，结果稳定、可重复
- **智能补译层**：由 Agent 人工触发执行，负责上下文相关的翻译内容处理

### 3.1 确定性层

由工具链负责：

- 扫描消息 key
- 同步 locale key 集合
- 输出缺失与状态报告
- 校验结构、完整性、占位符和治理状态
- 生成 `catalog_gen.go`

涉及命令：

- `make i18n-sync`
- `make i18n-report`
- `make i18n-check`
- `make i18n`

### 3.2 智能补译层

由 Agent 负责：

- 读取 `i18n-report`
- 读取 source locale 文案和 glossary
- 结合代码上下文补全多语言初稿
- 统一术语
- 修正 stale 翻译
- 更新 locale 文件和翻译状态

## 4. 关键约定

### 4.1 source locale

固定 `zh-CN` 为 source locale：

- 所有新增消息 key，必须先在 `zh-CN.json` 中提供正式源文案
- 其他语言以 `zh-CN` 为翻译基准
- 如果 source locale 缺文案，则不允许进入正式生成与发布流程

### 4.2 正式翻译源

正式翻译源继续使用 `internal/pkg/i18n/locales/*.json`，保持现有结构不变：

- `language`
- `aliases`
- `messages`

### 4.3 元数据与术语表

翻译治理状态单独存放在：

- `internal/pkg/i18n/.meta/status.json`

统一术语表单独存放在：

- `internal/pkg/i18n/glossary.json`

## 5. 文件结构设计

建议生成项目包含以下结构：

- `internal/pkg/i18n/i18n.go`
- `internal/pkg/i18n/catalog_gen.go`
- `internal/pkg/i18n/locales/*.json`
- `internal/pkg/i18n/.meta/status.json`
- `internal/pkg/i18n/.meta/report.md`
- `internal/pkg/i18n/.meta/report.json`
- `internal/pkg/i18n/glossary.json`
- `tools/i18n/util/i18nutil.go`
- `tools/i18n/util/i18nutil_test.go`
- `tools/i18n/gen/main.go`
- `tools/i18n/sync/main.go`
- `tools/i18n/report/main.go`
- `tools/i18n/check/main.go`

## 6. 命令设计

### 6.1 `make i18n-sync`

职责：

- 扫描代码中定义的消息 key
- 同步 source locale 与 target locales 的 key 集合
- 自动补齐缺失 key
- 初始化或更新 `status.json`
- 根据 source 文案变化标记 stale 状态

约束：

- 不自动翻译
- 不覆盖非空翻译
- 不覆盖已人工确认内容

### 6.2 `make i18n-report`

职责：输出当前 i18n 缺口报告，建议生成：

- `internal/pkg/i18n/.meta/report.md`
- `internal/pkg/i18n/.meta/report.json`

报告建议包含：

- summary
- missing source messages
- missing translations
- stale translations
- draft translations
- extra keys
- glossary hints

说明：

- `summary` 用于给开发者和 Agent 提供快速概览
- `glossary hints` 用于提示哪些目标语言文案可能没有使用推荐术语

### 6.3 `make i18n-check`

职责：校验 locale 和 metadata 是否满足工程约束，范围包括：

- JSON 结构合法性
- source/target locale 完整性
- 占位符一致性
- 状态约束
- glossary 弱提示

建议支持两种模式：

- `dev`：允许部分 `draft`
- `release`：禁止 `draft` 和 `stale`

说明：

- glossary 不一致在当前阶段只输出 warning，不直接阻断流程
- glossary warning 的目标是辅助 review，而不是替代人工判断

### 6.4 `make i18n`

职责保持不变：从 `locales/*.json` 生成 `internal/pkg/i18n/catalog_gen.go`。

### 6.5 `make generate`

建议默认串联：

- `i18n-sync`
- `i18n-check`
- `i18n`

不建议默认串入 Agent 补译或外部翻译服务。

## 7. 消息扫描策略

仅扫描明确注册消息的代码位置，v1 推荐支持：

- `response.MustRegister(response.Definition{ Msg: "..." })`
- `response.Definition{ Msg: "..." }`
- `oops.Public("...")`

建议采用 Go AST 扫描，而不是简单 regex。当前阶段建议只支持静态字符串消息 key，不支持运行时拼接 key，因此以下形式不纳入自动同步范围：

- `Public(dynamicValue)`
- 运行时拼接后的 `Msg`

## 8. metadata 与 report 设计

### 8.1 状态模型

建议每个 `locale + key` 维护以下状态之一：

- `draft`
- `reviewed`
- `stale`

`status.json` 中每条记录建议包含：

- `status`
- `source_locale`
- `source_hash`
- `updated_at`
- `updated_by`
- `note`

### 8.2 report 结构

`report.md` 面向开发者，`report.json` 面向 Agent 和后续工具。建议输出以下清单：

- `summary`
- `missing_source`
- `missing_translations`
- `stale_translations`
- `draft_translations`
- `extra_keys`
- `glossary_hints`

其中：

- `summary` 至少包含 locale 数量、message key 数量和各类问题计数
- `glossary_hints` 至少包含 `language`、`key`、`term`、`recommended`、`current_text`

## 9. Agent 协作规范

Agent 只负责智能内容处理，不负责工程规则判定。

建议 Agent 补译时优先参考：

- `internal/pkg/i18n/.meta/report.md`
- `internal/pkg/i18n/.meta/report.json`
- `internal/pkg/i18n/locales/zh-CN.json`
- 目标 locale 文件
- `internal/pkg/i18n/glossary.json`
- 涉及该 key 的业务代码上下文

建议默认限制 Agent 只修改：

- `internal/pkg/i18n/locales/*.json`
- `internal/pkg/i18n/.meta/status.json`

每次 Agent 补译后，必须重新执行：

- `make i18n-check`
- `make i18n`

## 10. 工作流设计

### 10.1 新增消息 key

1. 在业务代码中新增消息 key
2. 在 `zh-CN.json` 中补 source 文案
3. 执行 `make i18n-sync`
4. 执行 `make i18n-report`
5. 人工触发 Agent 补齐其他语言
6. 执行 `make i18n-check`
7. 执行 `make i18n`

### 10.2 修改 source locale 文案

1. 修改 `zh-CN.json`
2. 执行 `make i18n-sync`
3. 被影响的目标翻译标记为 `stale`
4. 执行 `make i18n-report`
5. Agent 处理 stale 项
6. 执行 `make i18n-check`
7. 执行 `make i18n`

### 10.3 新增一种语言

1. 新建 locale 文件
2. 填写 `language` 和 `aliases`
3. 执行 `make i18n-sync`
4. 执行 `make i18n-report`
5. Agent 批量补译
6. 执行 `make i18n-check`
7. 执行 `make i18n`

## 11. 校验与 CI 策略

`i18n-check` 需要校验：

- JSON 结构合法
- target locale 覆盖 source locale 全部 key
- source locale 不缺原文
- 占位符如 `%s`、`%d`、`%v`、`{id}`、`{name}` 一致
- `dev` 模式允许 `draft`
- `release` 模式禁止 `draft` / `stale`

同时建议输出：

- glossary 相关 warning

这些 warning 默认不阻断流程，但应进入 review 视野。

CI 默认建议执行：

- `make i18n-sync`
- `make i18n-check`
- `make i18n`

不建议在 CI 中默认执行 Agent 补译或外部翻译 API。

## 12. glossary 治理

第一阶段建议至少纳入以下术语：

- token
- signature
- session
- permission
- route
- idempotency
- rate limit

用途：

- Agent 补译时统一术语
- 人工 review 时作为基线
- 后续 `i18n-check` 可增加弱提示能力

## 13. 实施计划

### Phase 1：最小闭环

实现：

- `i18n-sync`
- `i18n-report`
- `i18n-check`
- `status.json`
- `glossary.json`
- Makefile 新目标
- 基础测试

### Phase 2：模板接入

实现：

- 将新工具接入 Hertz 模板
- 更新 `layout.yaml`
- 更新 golden
- 更新 mono 测试
- 验证生成项目可执行 `sync/check/i18n`

### Phase 2.5：增强确定性层

实现：

- `i18n-sync` 额外扫描 `oops.Public("...")`
- `i18n-report` 输出 `summary` 与 `glossary_hints`
- `i18n-check` 输出 glossary 弱提示 warning
- 生成项目新增 `tools/i18n/util/i18nutil_test.go`
- 模板集成验证覆盖 `go test ./tools/...`

### Phase 3：文档与 Agent 规范

实现：

- 主方案文档
- 开发工作流文档
- Agent 协作文档

### Phase 4：可选增强

未来若有需要，再考虑：

- provider 抽象
- 外部翻译 API
- 缓存机制
- 无人值守自动补译

## 14. 最终建议

当前阶段建议固定采用以下约定：

- source locale 为 `zh-CN`
- 正式翻译源为 `locales/*.json`
- metadata 独立存储
- `make generate` 默认串 `i18n-sync + i18n-check + i18n`
- Agent 在 `report` 后人工触发
- CI 只做校验与生成，不做自动补译

## 15. 相关文档

- [i18n 工作流](i18n-workflow.zh-CN.md)
- [i18n Agent 协作](i18n-agent-workflow.zh-CN.md)
- [i18n Agent 协议](i18n-agent-protocol.zh-CN.md)
- [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)
- [i18n CLI / MCP Payload](i18n-payload.zh-CN.md)
