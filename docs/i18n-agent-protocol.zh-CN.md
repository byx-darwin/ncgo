# ncgo Hertz i18n Agent Prompt / Protocol

本文定义 Agent 消费 `internal/pkg/i18n/.meta/report.json` 时的固定输入、固定输出和执行约束，目标是让补译过程更稳定、更可复现。若需要机器可消费的字段约束，请同时参考 [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)。

## 1. 协议目标

- 固定 Agent 任务输入
- 固定 Agent 输出结构
- 降低自由发挥导致的漂移
- 让人工 review、MCP、CLI 和后续自动化更容易集成

## 2. 协议版本

- protocol: `i18n-agent/v1`
- source locale: `zh-CN`
- report source: `internal/pkg/i18n/.meta/report.json`

## 3. 输入契约

执行补译前，调用方应至少提供：

1. `internal/pkg/i18n/.meta/report.json`
2. `internal/pkg/i18n/locales/zh-CN.json`
3. 目标 locale 文件，例如 `internal/pkg/i18n/locales/ja-JP.json`
4. `internal/pkg/i18n/glossary.json`
5. 相关代码上下文（如果 key 来自 `Msg` 或 `Public(...)`）

Agent 应优先读取 `report.json` 中这些字段：

- `summary`
- `missing_translations`
- `stale_translations`
- `draft_translations`
- `glossary_hints`

## 4. report.json 最小依赖字段

协议假定 `report.json` 至少提供：

- `source_locale`
- `summary`
- `missing_translations`
- `stale_translations`
- `draft_translations`
- `glossary_hints`

其中每个待处理 item 至少应含：

- `language`
- `key`
- `source_text`
- `current_text`
- `status`

## 5. Agent 任务选择规则

推荐顺序：

1. 先看 `summary`
2. 优先处理目标 locale 的 `missing_translations`
3. 再处理 `stale_translations`
4. 再处理 `draft_translations`
5. 最后 review `glossary_hints`

默认不处理：

- `source_locale` 缺原文的条目
- 运行时动态 key
- 未在报告中出现、且任务未明确要求的 locale

## 6. 固定 Prompt 模板

调用方建议给 Agent 传递固定任务描述：

- protocol version：`i18n-agent/v1`
- target locale：例如 `ja-JP`
- task scope：`missing|stale|draft|glossary-review`
- writable files：locale JSON + `status.json`
- forbidden files：`catalog_gen.go`、业务代码、Makefile、CI
- validation steps：`make i18n-check`、`make i18n`

固定任务指令建议包含这些要求：

- 仅处理指定 locale
- 仅处理报告中列出的 key
- 以 `zh-CN` 为 source truth
- 保持占位符完全一致
- 优先遵循 glossary
- 不修改生成文件
- 如 source 文案不足以判断语义，先标记为需要人工 review

## 7. 固定输出契约

Agent 输出建议统一为结构化结果，可映射为 JSON：

- `protocol`
- `target_locale`
- `actions`
- `warnings`
- `needs_human_review`
- `validation`

其中 `actions` 中每项建议包含：

- `key`
- `action`
- `translation`
- `status`
- `reason`

推荐 `action` 枚举：

- `fill_missing`
- `refresh_stale`
- `refine_draft`
- `keep_existing`
- `skip`

推荐 `status` 枚举：

- `reviewed`
- `draft`
- `needs_human_review`

## 8. Agent 执行约束

Agent 默认只应修改：

- `internal/pkg/i18n/locales/<target-locale>.json`
- `internal/pkg/i18n/.meta/status.json`

Agent 不应默认修改：

- `internal/pkg/i18n/catalog_gen.go`
- `internal/pkg/i18n/glossary.json`
- 业务逻辑代码
- Makefile / CI

## 9. 验收规则

完成补译后，调用方或 Agent 应推动执行：

1. `make i18n-check`
2. `make i18n`

如果这次任务还改到了 i18n 工具实现本身，再执行：

3. `go test ./tools/...`

如果 `i18n-check` 只输出 glossary warning，则视为：

- 可继续进入生成功能链路
- 但应进入人工 review

## 10. 推荐集成方式

建议未来统一采用下面这条链路：

1. `make i18n-sync`
2. `make i18n-report`
3. 选择 target locale
4. 以 `i18n-agent/v1` 协议调用 Agent
5. 写回 locale / status
6. `make i18n-check`
7. `make i18n`

## 11. 相关文档

- [i18n Hybrid 方案](i18n-hybrid-plan.zh-CN.md)
- [i18n 工作流](i18n-workflow.zh-CN.md)
- [i18n Agent 协作](i18n-agent-workflow.zh-CN.md)
- [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)
- [i18n CLI / MCP Payload](i18n-payload.zh-CN.md)
