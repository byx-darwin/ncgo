# ncgo i18n Internationalization System Design

- Status: v1
- Scope: ncgo Hertz template-generated projects
- Full design (ZH): [004-i18n-system.zh-CN.md](./004-i18n-system.zh-CN.md)

## 1. Overview

ncgo Hertz templates include a built-in Agent + toolchain i18n system:

- **Source language**: zh-CN
- **Translation files**: `internal/pkg/i18n/locales/*.json`
- **Code generation**: `make i18n` (generates `catalog_gen.go`)
- **Key sync**: `make i18n-sync` (scans `Msg` / `Public(...)` calls)
- **Gap report**: `make i18n-report` (outputs `summary` + `glossary_hints`)
- **Validation**: `make i18n-check` (glossary warnings, non-blocking by default)

## 2. Design Goals

- Automatic discovery of new message keys
- Automatic cross-locale key set synchronization
- AI Agent-driven intelligent translation of missing entries
- Structural governance, state governance, and generation governance via toolchain

## 3. Workflow

### Adding a New Message Key

1. Register the new key in business code (`Msg: "..."` or `Public("...")`)
2. Run `make i18n-sync` to sync key sets
3. Run `make i18n-report` to generate a gap report
4. Agent reads the report and fills in missing translations
5. Run `make i18n` to regenerate the catalog

### Agent Translation Flow

Fixed inputs for the Agent:
1. `internal/pkg/i18n/.meta/report.json` (or `report.md`)
2. `internal/pkg/i18n/locales/zh-CN.json` (source strings)
3. Target locale file (e.g., `ja-JP.json`)
4. `internal/pkg/i18n/glossary.json` (terminology reference)
5. Relevant business code context

Agent output must follow the `i18n-agent/v1` protocol and JSON Schema.

## 4. Key Design Decisions

### Protocol and Schema

- **Protocol**: `i18n-agent/v1`
- **Source Locale**: `zh-CN`
- **Report Input Schema**: `ncgo://schemas/i18n/report-input-v1`
- **Agent Output Schema**: `ncgo://schemas/i18n/agent-output-v1`

Schema files: `schemas/i18n/report-input-v1.schema.json`, `schemas/i18n/agent-output-v1.schema.json`

### Report Structure

Minimum required fields: `source_locale`, `summary`. Agents should read: `summary`, `missing_translations`, `stale_translations`, `draft_translations`, and `glossary_hints`.

### Agent Constraints

- Role: read gap reports + translate multi-language strings
- NOT responsible for: defining engineering rules, skipping validation, directly modifying generated output
- Prioritize glossary (`glossary.json`) for translations
- Mark uncertain translations as `draft` for human review
- Include confidence information in output

## 5. CLI / MCP Interface

### CLI Commands

```bash
ncgo i18n report --root . --output json   # Read i18n report
ncgo i18n check --root . --mode dev       # Dev mode validation
ncgo i18n check --root . --mode release   # Release mode (strict)
```

### MCP Tools

| Tool               | Description                              |
|--------------------|------------------------------------------|
| `ncgo_i18n_report` | Read structured i18n report              |
| `ncgo_i18n_check`  | Evaluate report health (dev/release mode) |

Both tools support `text` and `json` output formats.

## 6. Related Files

- `schemas/i18n/report-input-v1.schema.json`
- `schemas/i18n/agent-output-v1.schema.json`
- `internal/pkg/i18n/i18n.go` (language registration and negotiation in generated projects)

## 7. Archived Detailed Docs

Original design documents archived in `specs/archive/`:
- `i18n-hybrid-plan.zh-CN.md`
- `i18n-agent-protocol.zh-CN.md`
- `i18n-agent-schema.zh-CN.md`
- `i18n-agent-workflow.zh-CN.md`
- `i18n-workflow.zh-CN.md`
- `i18n-payload.zh-CN.md`

## 8. Reference

Full protocol details, schema constraints, and Agent workflow are in the Chinese document:
[004-i18n-system.zh-CN.md](./004-i18n-system.zh-CN.md)
