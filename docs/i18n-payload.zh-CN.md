# ncgo Hertz i18n CLI / MCP Structured Payload

本文定义 i18n 工具链在 CLI `--output json` 与 MCP tool 返回中的结构化 payload，目标是让它们和 `i18n-agent/v1` 协议、JSON Schema 保持一致。

## 1. 设计目标

- 对齐现有 `add infra --output json` 风格
- 对齐现有 MCP `textResult(...) + structured fields` 风格
- 避免 CLI / MCP 各自发明不同字段
- 让 Agent 能直接消费 `report` / `check` 结果

## 2. 当前代码风格约束

当前仓库已有两种成熟模式：

1. CLI：`--output json` 直接输出 JSON object
2. MCP：返回 `content` + `isError`，并在顶层附带结构化字段

因此 i18n payload 建议遵循：CLI JSON 直接输出业务对象；MCP 保留文本摘要，同时暴露同名结构化字段。

## 3. 建议命令 / tool 形态

### CLI

- `ncgo i18n report --root . --output json`
- `ncgo i18n check --root . --mode dev --output json`
- `ncgo i18n check --root . --mode release --output json`

### MCP

- `ncgo_i18n_report`
- `ncgo_i18n_check`

## 4. CLI report payload

CLI `i18n report --output json` 建议直接输出：

- `root`
- `sourceLocale`
- `localesDir`
- `statusPath`
- `glossaryPath`
- `reportPathJSON`
- `reportPathMarkdown`
- `schema`
- `report`
- `nextSteps`

其中 `report` 应直接符合 `schemas/i18n/report-input-v1.schema.json`，`schema` 建议包含 `id`、`path`。

## 5. CLI check payload

CLI `i18n check --output json` 建议输出：

- `root`
- `mode`
- `ok`
- `sourceLocale`
- `schema`
- `summary`
- `failures`
- `warnings`
- `nextSteps`

其中 `summary` 建议复用 `report.summary`；`failures` 用于阻断项；`warnings` 主要承载 glossary warning。

## 6. MCP report / check payload

MCP `ncgo_i18n_report` 建议返回：

- `content`
- `isError`
- `root`
- `sourceLocale`
- `schema`
- `report`
- `reportPathJSON`
- `reportPathMarkdown`
- `nextSteps`

MCP `ncgo_i18n_check` 建议返回：

- `content`
- `isError`
- `root`
- `mode`
- `ok`
- `summary`
- `failures`
- `warnings`
- `nextSteps`

约定：

- `content[0].text` 放简短人类可读摘要
- `report` 保持与 CLI JSON 完全同构
- 当 `failures` 非空时，`isError=true`
- 只有 glossary warning 时，`isError=false`

## 7. CLI / MCP / Agent 示例

### 7.1 CLI report

命令：

`ncgo i18n report --root . --output json`

返回示例：

```json
{
  "root": "/repo/demo",
  "sourceLocale": "zh-CN",
  "schema": {"id": "ncgo://schemas/i18n/report-input-v1", "path": "schemas/i18n/report-input-v1.schema.json"},
  "report": {
    "source_locale": "zh-CN",
    "summary": {"locale_count": 8, "message_key_count": 120, "missing_translations_count": 3}
  },
  "nextSteps": ["review internal/pkg/i18n/.meta/report.md", "make i18n-check", "make i18n"]
}
```

### 7.2 CLI check

命令：

- `ncgo i18n check --root . --mode dev --output json`
- `ncgo i18n check --root . --mode release --output json`

返回示例：

```json
{
  "root": "/repo/demo",
  "mode": "release",
  "ok": false,
  "sourceLocale": "zh-CN",
  "schema": {"report": {"id": "ncgo://schemas/i18n/report-input-v1", "path": "schemas/i18n/report-input-v1.schema.json"}},
  "summary": {"locale_count": 8, "message_key_count": 120, "draft_translations_count": 2},
  "failures": ["ja-JP/internal_error is draft", "it-IT/internal_error is stale"],
  "warnings": ["fr-FR/internal_error may not use recommended glossary term \"signature\" (want \"firma\")"]
}
```

### 7.3 MCP report

`tools/call` 请求示例：

```json
{
  "name": "ncgo_i18n_report",
  "arguments": {"root": "/repo/demo"}
}
```

返回示例：

```json
{
  "content": [{"type": "text", "text": "i18n report loaded for zh-CN: locales=8 keys=120 missing=3 stale=1 draft=2 warnings=1"}],
  "isError": false,
  "root": "/repo/demo",
  "sourceLocale": "zh-CN",
  "schema": {"id": "ncgo://schemas/i18n/report-input-v1", "path": "schemas/i18n/report-input-v1.schema.json"},
  "report": {"source_locale": "zh-CN"}
}
```

### 7.4 MCP check

`tools/call` 请求示例：

```json
{
  "name": "ncgo_i18n_check",
  "arguments": {"root": "/repo/demo", "mode": "dev"}
}
```

返回示例：

```json
{
  "content": [{"type": "text", "text": "i18n check (dev): ok (failures=0 warnings=1)"}],
  "isError": false,
  "root": "/repo/demo",
  "mode": "dev",
  "ok": true,
  "sourceLocale": "zh-CN",
  "schema": {"report": {"id": "ncgo://schemas/i18n/report-input-v1", "path": "schemas/i18n/report-input-v1.schema.json"}},
  "failures": [],
  "warnings": ["it-IT/internal_error may not use recommended glossary term \"signature\" (want \"firma\")"]
}
```

### 7.5 Agent 消费建议

建议 Agent 按下面顺序消费：

1. 先调用 `ncgo_i18n_report` 或 `ncgo i18n report --output json`
2. 从 `report.missing_translations` / `stale_translations` / `draft_translations` 中挑选当前任务范围
3. 写回 locale JSON 与 `status.json`
4. 调用 `ncgo_i18n_check` 或 `ncgo i18n check --mode release --output json`
5. 若 `failures` 为空，再执行 `make i18n`

如果 Agent 最终输出自己的补译结果，建议继续遵循 [i18n Agent 协议](i18n-agent-protocol.zh-CN.md) 与 [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)。

## 8. schema 映射规则

建议统一映射：

- `schema.id = ncgo://schemas/i18n/report-input-v1`
- `schema.path = schemas/i18n/report-input-v1.schema.json`

如果未来为 `check` 单独建 schema，可扩展 `ncgo://schemas/i18n/check-output-v1`。

## 9. 推荐字段命名

建议遵守以下规则：

- CLI JSON 顶层：camelCase
- MCP 顶层：沿用现有 camelCase
- 嵌套 `report`：保持 snake_case，与现有 `report.json` 一致
- wrapper 层用 `sourceLocale`，embedded report 层保留 `source_locale`

## 10. 落地顺序（当前已完成）

当前按以下顺序落地：

1. `ncgo i18n report --output json`
2. `ncgo_i18n_report`
3. `ncgo i18n check --output json`
4. `ncgo_i18n_check`

以上四步目前都已完成，因此 CLI / MCP 两侧都已经具备 `report/check` 的结构化消费能力。

## 11. 相关文档

- [i18n Agent 协议](i18n-agent-protocol.zh-CN.md)
- [i18n Agent JSON Schema](i18n-agent-schema.zh-CN.md)
- [i18n Agent 协作](i18n-agent-workflow.zh-CN.md)
- [i18n 工作流](i18n-workflow.zh-CN.md)
