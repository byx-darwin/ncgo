# ncgo Hertz i18n Internationalization System Design

- Status: v1
- Scope: Projects generated from `ncgo` Hertz templates
- Source: Merged from `docs/i18n-*.zh-CN.md` (6 docs)

## 1. Overview

The ncgo Hertz template includes a built-in Agent + toolchain i18n solution:

- **Source language**: `zh-CN`
- **Translation files**: `internal/pkg/i18n/locales/*.json`
- **Generate command**: `make i18n` (generates `catalog_gen.go`)
- **Sync command**: `make i18n-sync` (scans `Msg` / `Public(...)` calls)
- **Report command**: `make i18n-report` (produces `summary` + `glossary_hints`)
- **Check command**: `make i18n-check` (glossary warning, non-blocking by default)

## 2. Design Goals

- Auto-discover new message keys
- Auto-sync key sets across all locales
- AI Agent performs intelligent translation gap filling
- Toolchain handles structural, state, and generation governance

## 3. Workflow

### 3.1 New Message Key Flow

1. Register new key in business code (`Msg: "..."` or `Public("...")`)
2. Run `make i18n-sync` to sync key sets
3. Run `make i18n-report` to generate gap report
4. Agent reads report and fills missing translations
5. Run `make i18n` to regenerate catalog

### 3.2 Agent Translation Flow

Fixed inputs for Agent translation:

1. `internal/pkg/i18n/.meta/report.json` (or `report.md`)
2. `internal/pkg/i18n/locales/zh-CN.json` (source strings)
3. Target locale file (e.g., `ja-JP.json`)
4. `internal/pkg/i18n/glossary.json` (terminology)
5. Relevant business code context

Agent output must follow `i18n-agent/v1` protocol and JSON Schema.

## 4. Protocol & Schema

### 4.1 Protocol Version

- **Protocol**: `i18n-agent/v1`
- **Source Locale**: `zh-CN`
- **Report Source**: `internal/pkg/i18n/.meta/report.json`

### 4.2 Schema Identifiers

- Report Input Schema: `ncgo://schemas/i18n/report-input-v1`
- Agent Output Schema: `ncgo://schemas/i18n/agent-output-v1`

Actual schema files: `schemas/i18n/report-input-v1.schema.json` and `schemas/i18n/agent-output-v1.schema.json`

### 4.3 Report Structure

Minimum required fields: `source_locale`, `summary`

Agent should prioritize in `report.json`:
- `summary`
- `missing_translations`
- `stale_translations`
- `draft_translations`
- `glossary_hints` (review first)

## 5. CLI / MCP Interface

### CLI Commands

```bash
ncgo i18n report --root . --output json   # Read i18n report
ncgo i18n check --root . --mode dev       # Dev mode check
ncgo i18n check --root . --mode release   # Release mode check (strict)
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `ncgo_i18n_report` | Read structured i18n report |
| `ncgo_i18n_check` | Evaluate report health (dev/release mode) |

Both tools support `text` and `json` output formats.

## 6. Agent Constraints

- Agent role: read gap report + fill multi-language translations
- Agent does NOT: define engineering rules, skip validation, directly modify generated artifacts
- Prioritize glossary (`glossary.json`) for translations
- Uncertain translations should be marked `draft` for human review
- Output must include confidence information

## 7. Related Files

- `schemas/i18n/report-input-v1.schema.json`
- `schemas/i18n/agent-output-v1.schema.json`
- `internal/pkg/i18n/i18n.go` (language registration and negotiation in generated projects)

## 8. Detailed Reference

Original design docs archived in `specs/archive/`:
- `i18n-hybrid-plan.zh-CN.md` — Hybrid solution design
- `i18n-agent-protocol.zh-CN.md` — Agent protocol details
- `i18n-agent-schema.zh-CN.md` — JSON Schema constraints
- `i18n-agent-workflow.zh-CN.md` — Agent collaboration spec
- `i18n-workflow.zh-CN.md` — Development workflow
- `i18n-payload.zh-CN.md` — CLI/MCP structured payload
