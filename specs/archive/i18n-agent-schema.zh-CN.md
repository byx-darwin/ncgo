# ncgo Hertz i18n Agent JSON Schema

本文把 `i18n-agent/v1` 协议进一步细化为可机器消费的 JSON Schema 文档，覆盖两部分：

- `report.json` 输入 schema
- Agent 结构化输出 schema

## 1. 目标

- 固定 MCP / CLI / Agent 之间的数据边界
- 让补译任务可验证、可回放、可 review
- 为实际 `.schema.json` 文件提供稳定约束

## 2. Schema 标识

- report input schema id：`ncgo://schemas/i18n/report-input-v1`
- agent output schema id：`ncgo://schemas/i18n/agent-output-v1`
- protocol version：`i18n-agent/v1`

当前仓库已提供：

- `schemas/i18n/report-input-v1.schema.json`
- `schemas/i18n/agent-output-v1.schema.json`

## 3. report 输入 schema

最小 required 字段：

- `source_locale`
- `summary`
- `missing_translations`
- `stale_translations`
- `draft_translations`
- `glossary_hints`

说明：

- 当前 schema 与现有 `report.json` 输出兼容
- 对于空列表，允许出现 `null`
- 对于 `source_text` / `current_text` / `status` 这类 `omitempty` 字段，schema 允许缺省

推荐 JSON Schema 轮廓：

```
{
  "$id": "ncgo://schemas/i18n/report-input-v1",
  "type": "object",
  "required": [
    "source_locale", "summary", "missing_translations",
    "stale_translations", "draft_translations", "glossary_hints"
  ],
  "properties": {
    "source_locale": {"type": "string", "const": "zh-CN"},
    "summary": {
      "type": "object",
      "required": ["locale_count", "message_key_count"],
      "properties": {
        "locale_count": {"type": "integer", "minimum": 1},
        "message_key_count": {"type": "integer", "minimum": 0},
        "missing_translations_count": {"type": "integer", "minimum": 0},
        "stale_translations_count": {"type": "integer", "minimum": 0},
        "draft_translations_count": {"type": "integer", "minimum": 0},
        "glossary_hints_count": {"type": "integer", "minimum": 0}
      }
    },
    "missing_translations": {"type": "array", "items": {"$ref": "#/definitions/reportItem"}},
    "stale_translations": {"type": "array", "items": {"$ref": "#/definitions/reportItem"}},
    "draft_translations": {"type": "array", "items": {"$ref": "#/definitions/reportItem"}},
    "glossary_hints": {"type": "array", "items": {"$ref": "#/definitions/glossaryHint"}}
  }
}
```

`reportItem` 最小 required 字段：

- `language: string`
- `key: string`

可选字段：

- `source_text: string`
- `current_text: string`
- `status: string`

`glossaryHint` 最小字段：

- `language: string`
- `key: string`
- `term: string`
- `recommended: string`
- `current_text: string`

## 4. Agent 输出 schema

最小 required 字段：

- `protocol`
- `target_locale`
- `actions`
- `warnings`
- `needs_human_review`
- `validation`

推荐 JSON Schema 轮廓：

```
{
  "$id": "ncgo://schemas/i18n/agent-output-v1",
  "type": "object",
  "required": [
    "protocol", "target_locale", "actions",
    "warnings", "needs_human_review", "validation"
  ],
  "properties": {
    "protocol": {"type": "string", "const": "i18n-agent/v1"},
    "target_locale": {"type": "string", "minLength": 2},
    "warnings": {"type": "array", "items": {"type": "string"}},
    "needs_human_review": {"type": "boolean"},
    "actions": {
      "type": "array",
      "items": {"$ref": "#/definitions/action"}
    },
    "validation": {
      "type": "object",
      "required": ["required_steps"],
      "properties": {
        "required_steps": {"type": "array", "items": {"type": "string"}},
        "executed_steps": {"type": "array", "items": {"type": "string"}}
      }
    }
  }
}
```

`action` 最小 required 字段：

- `key: string`
- `action: enum`
- `status: enum`
- `reason: string`

可选字段：

- `translation: string`

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

## 5. 约束说明

- `target_locale` 必须与本次任务指定语言一致
- `actions[].key` 应来自 `report.json` 的待处理条目
- `translation` 不得为空；若无法判断语义，应使用 `skip` + `needs_human_review`
- 若输出包含 glossary 相关分歧，建议写入 `warnings`
- `validation.required_steps` 默认至少包含 `make i18n-check` 与 `make i18n`

## 6. 推荐落地方式

建议按两层维护：

1. 文档层说明 schema 语义和字段约束
2. 文件层提供真实 `.schema.json` 实现

## 7. 相关文档

- [i18n Agent 协议](i18n-agent-protocol.zh-CN.md)
- [i18n Agent 协作](i18n-agent-workflow.zh-CN.md)
- [i18n 工作流](i18n-workflow.zh-CN.md)
- [i18n Hybrid 方案](i18n-hybrid-plan.zh-CN.md)
- [i18n CLI / MCP Payload](i18n-payload.zh-CN.md)