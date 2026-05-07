# Go Coding Rules

This file defines Go-specific coding rules for the `ncgo` repository.
Execution flow, testing order, and risk control live in `.claude/rules/agent-engineering.md`.

## 1. Goals

- Keep Go changes small, readable, and consistent with the current codebase.
- Preserve stable CLI, MCP, template, and generated-output contracts unless the task explicitly changes them.
- Prefer existing repository patterns over introducing new abstractions.

## 2. General Style

- Follow standard Go style and keep files `gofmt`-clean.
- Prefer small, focused functions over large rewrites.
- Prefer explicit, readable code over clever abstractions.
- Reuse existing helpers before adding new utility layers.
- Keep naming aligned with nearby packages and existing exported APIs.

## 3. Repository Structure

- `internal/cli`: CLI entrypoints and command wiring.
- `internal/mcp`: MCP tool schemas, tool handlers, structured outputs, and server behavior. Key file: `tools.go`.
- `internal/scaffold`: scaffold/template logic and generated tree behavior. Contains golden fixtures.
- `internal/doctor`, `internal/protolint`, `internal/ai`, `internal/upgrade`, `internal/extract`: domain-specific logic.
- `internal/assets/_data/{hertz,kitex}/*`: embedded scaffold templates. Treat as contract-sensitive; changes affect generated projects.
- `internal/assets/_data/docs/*`: embedded design docs used by generated projects and `ai sync`.
- `schemas/`: JSON schemas for i18n payloads.
- `scripts/smoke.sh`: final smoke validation entrypoint.

Keep changes inside the smallest relevant package. Do not move code across packages unless the task truly requires it.

## 4. Public and Contract-Sensitive Surfaces

Be especially conservative when editing:

- CLI flags, command text, and JSON output
- MCP schemas, `content[0].text`, top-level structured fields, and error behavior
- scaffold templates and generated file layouts
- doctor/protolint/i18n/add-infra output formats
- embedded design docs consumed by generated projects or `ai sync`

When these surfaces change, update tests and docs together.

## 5. Errors and Control Flow

- Return clear errors; do not swallow errors silently.
- Wrap errors with useful context when crossing package or filesystem boundaries.
- Preserve existing error wording when tests, CLI output, or MCP contracts rely on it.
- Prefer early returns to deeply nested control flow.

For generated project conventions described by the embedded Hertz/Kitex design docs, keep the documented boundary rules stable unless the task explicitly changes them.

## 6. Tests and Test Placement

- Put tests close to the code they validate.
- Add helper-level tests for pure logic, output formatting, schema helpers, and builders.
- Use integration tests for CLI, MCP, and multi-package wiring behavior.
- When changing templates or generated trees, preserve or update golden tests deliberately.

Repository-wide final validation still follows `CONTRIBUTING.md` and `.claude/rules/agent-engineering.md`.

## 7. Generated Files and Templates

- Do not hand-edit downstream generated project files as a substitute for fixing templates or generators.
- Treat embedded templates, design docs, and scaffold outputs as contract-sensitive.
- Keep generated markers, file ownership rules, and overwrite behavior stable unless explicitly requested.
- If a change affects scaffold output, update the corresponding golden tests or output checks.

## 8. Documentation and Code Coupling

- When Go changes affect user-facing behavior, update the relevant docs.
- Keep README/examples and English/Chinese variants aligned for the same behavior.
- If code introduces a new stable output field, flag, schema element, or workflow step, document it near the existing contract docs.

## 9. Repository-Specific Preferences

- Prefer package-level helpers for shared MCP output/schema logic instead of duplicating per-tool code.
- Prefer deterministic test fixtures and pinned metadata in golden tests. Golden fixtures live alongside the package under `testdata/` or `*_test.go` files in `internal/scaffold/`.
- Keep AI context generation (`ai sync`) idempotent and conservative about overwriting user-owned files.
- Keep MCP behavior dual-use: readable text in `content[0].text` for display, stable top-level structured fields for agents.
- MCP tool changes: verify `InputSchema`, `content[0].text` shape, top-level JSON fields, and error response behavior together.
- When a CLI command or flag changes: update the nearest doc (`README.md` or `docs/examples.md`), keep English and Chinese variants aligned, and run `./scripts/smoke.sh`.
- When scaffold templates under `internal/assets/_data/` change: update or regenerate corresponding golden tests before merging.

## 10. Avoid

- Large opportunistic refactors during behavior work
- Mixing docs-only, refactor-only, and behavior changes without need
- Renaming or reshaping stable fields without updating dependent tests and docs
- Editing unrelated files just to make style more uniform