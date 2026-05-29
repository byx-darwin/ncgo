# ncgo Hertz i18n 国际化系统设计

- 状态：v1
- 适用范围：`ncgo` Hertz 模板生成的项目
- 源文档：合并自 `docs/i18n-*.zh-CN.md`（6 篇）

## 1. 概述

ncgo Hertz 模板内置了一套 Agent+工具链协作的国际化（i18n）方案：

- **源语言**：`zh-CN`
- **翻译文件**：`internal/pkg/i18n/locales/*.json`
- **生成命令**：`make i18n`（生成 `catalog_gen.go`）
- **同步命令**：`make i18n-sync`（扫描 `Msg` / `Public(...)` 调用）
- **报告命令**：`make i18n-report`（输出 `summary` + `glossary_hints`）
- **校验命令**：`make i18n-check`（glossary warning，默认不阻断）

## 2. 设计目标

- 自动发现新增消息 key
- 自动同步所有 locale 的 key 集合
- 通过 AI Agent 对缺失项进行智能补译
- 工具链完成结构治理、状态治理与生成治理

## 3. 工作流

### 3.1 新增消息 key

1. 在业务代码中注册新 key（`Msg: "..."` 或 `Public("...")`）
2. 运行 `make i18n-sync` 同步 key 集合
3. 运行 `make i18n-report` 生成缺口报告
4. Agent 读取报告，补译缺失翻译
5. 运行 `make i18n` 重新生成 catalog

### 3.2 Agent 补译流程

Agent 参与补译时的固定输入：

1. `internal/pkg/i18n/.meta/report.json`（或 `report.md`）
2. `internal/pkg/i18n/locales/zh-CN.json`（源文案）
3. 目标 locale 文件（如 `ja-JP.json`）
4. `internal/pkg/i18n/glossary.json`（术语表）
5. 相关业务代码上下文

Agent 输出需遵循 `i18n-agent/v1` 协议和 JSON Schema。

## 4. 协议与 Schema

### 4.1 协议版本

- **Protocol**: `i18n-agent/v1`
- **Source Locale**: `zh-CN`
- **Report Source**: `internal/pkg/i18n/.meta/report.json`

### 4.2 Schema 标识

- Report Input Schema: `ncgo://schemas/i18n/report-input-v1`
- Agent Output Schema: `ncgo://schemas/i18n/agent-output-v1`

实际 schema 文件位于：`schemas/i18n/report-input-v1.schema.json` 和 `schemas/i18n/agent-output-v1.schema.json`

### 4.3 Report 结构

最小 required 字段：`source_locale`、`summary`

Agent 应优先读取 `report.json` 中：
- `summary`
- `missing_translations`
- `stale_translations`
- `draft_translations`
- `glossary_hints`（优先 review）

## 5. CLI / MCP 接口

### CLI 命令

```bash
ncgo i18n report --root . --output json   # 读取国际化报告
ncgo i18n check --root . --mode dev       # dev 模式校验
ncgo i18n check --root . --mode release   # release 模式校验（严格）
```

### MCP 工具

| 工具 | 说明 |
|------|------|
| `ncgo_i18n_report` | 读取结构化国际化报告 |
| `ncgo_i18n_check` | 评估报告健康度（dev/release 模式） |

两种工具均支持 `text` 和 `json` 输出格式。

## 6. Agent 约束

- Agent 定位：读取缺口报告 + 补译多语言文案
- Agent 不负责：定义工程规则、跳过校验、直接修改生成物
- 翻译优先参考术语表（`glossary.json`）
- 不确定的翻译应标记为 `draft`，待人工 review
- 输出需包含置信度信息

## 7. 相关文件

- `schemas/i18n/report-input-v1.schema.json`
- `schemas/i18n/agent-output-v1.schema.json`
- `internal/pkg/i18n/i18n.go`（生成项目中的语言注册与协商）

## 8. 详细参考

原始设计文档已归档至 `specs/archive/`：
- `i18n-hybrid-plan.zh-CN.md` — 混合方案总体设计
- `i18n-agent-protocol.zh-CN.md` — Agent 协议详细定义
- `i18n-agent-schema.zh-CN.md` — JSON Schema 详细约束
- `i18n-agent-workflow.zh-CN.md` — Agent 协作规范
- `i18n-workflow.zh-CN.md` — 开发工作流
- `i18n-payload.zh-CN.md` — CLI/MCP 结构化 Payload
