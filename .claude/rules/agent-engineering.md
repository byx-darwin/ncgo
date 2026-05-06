# Agent Engineering Rules

This file defines how an AI coding agent should work in this repository.
It focuses on execution quality, validation discipline, and risk control.
It complements `CONTRIBUTING.md` rather than replacing it.

## 1. Goals

- Make small, correct, explainable changes.
- Prefer fast, reliable validation over guesswork.
- Keep repository behavior, contracts, and generated outputs stable unless the task explicitly changes them.
- Avoid risky actions unless the user explicitly asked for them.

## 2. Core Rules

- MUST read relevant implementation and nearby tests before editing.
- MUST keep diffs minimal and avoid unrelated cleanup.
- MUST validate changes with the smallest useful checks first.
- MUST update tests when behavior changes.
- MUST explain what changed, what was run, and any remaining risk.
- MUST NOT install dependencies, deploy, change external state, or run destructive operations without explicit permission.

## 3. Information Gathering

- Confirm the target symbol, file, or contract exists before editing.
- Read the narrowest set of files that can justify the change safely.
- Prefer existing helpers, patterns, and tests over inventing new structures.
- Stop exploring when additional reading no longer changes the implementation plan.
- If behavior is ambiguous after reasonable inspection, ask instead of guessing.

## 4. Change Strategy

- Prefer the smallest possible edit that fully solves the requested task.
- Preserve existing public behavior unless the task explicitly changes it.
- Do not mix refactors with behavior changes unless necessary.
- Keep generated files, templates, schemas, CLI output, and MCP contracts especially stable.
- When editing shared helpers or builders, assume broader regression risk and validate accordingly.

## 5. Testing Strategy

### 5.1 Unit Tests

Use unit tests for helpers, pure logic, formatting, schema parsing, output building, and other isolated behavior.

- SHOULD add or update unit tests whenever a logic branch changes.
- SHOULD prefer helper-level coverage instead of relying only on higher-level integration tests.

### 5.2 Integration Tests

Use integration tests for CLI commands, MCP tools, server behavior, or flows that wire multiple components together.

- MUST run relevant integration tests when public contracts, tool schemas, structured outputs, or command behavior may be affected.

### 5.3 Smoke Tests

Use smoke checks for non-destructive entrypoint verification, such as:

- `--help`
- minimal command execution
- targeted end-to-end command paths

Smoke tests should be fast and safe.

## 6. Validation Order

Run validation from smallest and fastest to broadest and slowest:

1. single test function or focused unit tests
2. relevant test file
3. relevant package or component tests
4. related integration tests
5. small smoke checks
6. broader repository checks only when needed

For final PR-quality validation, use the repository checks from `CONTRIBUTING.md` when the task warrants full confidence:

- `go build ./...`
- `go build .`
- `go vet ./...`
- `go test ./... -count=1`
- `./scripts/smoke.sh`

## 7. Failure Handling

- If validation fails, make the smallest plausible fix and rerun the most relevant check.
- Do not stack multiple speculative fixes before rerunning.
- If repeated attempts do not improve the situation, stop and ask for clarification.
- MUST NOT bypass failing validation by silently skipping tests or reducing coverage without explanation.

## 8. Documentation Rules

- When commands, install flow, release flow, generated outputs, or MCP/CLI contracts change, update docs and examples.
- Keep English and Chinese docs aligned when they describe the same user-facing behavior.
- After documentation edits, run markdown diagnostics.
- Prefer one clear source of truth over repeating the same contract in multiple places.

## 9. Repository-Specific Guidance

- CLI changes SHOULD be validated with targeted command tests and a minimal smoke path when practical.
- MCP changes MUST verify schema, output shape, `content[0].text`, top-level fields, and error behavior.
- Shared helper refactors SHOULD add package-level unit tests.
- Template, scaffold, and generated-output changes SHOULD be validated conservatively because they affect downstream users.
- For docs-only changes, explicitly say that the task is docs-only and report the diagnostics result.

## 10. Communication

- State the plan before major edits or validation runs.
- Summarize progress as the task advances.
- On completion, report:
  - what changed
  - what tests or checks were run
  - whether diagnostics were checked
  - any follow-up risk or optional next step

## 11. Stop Conditions

- Stop when the requested task is complete and relevant validation has passed.
- Do not continue into adjacent improvements without asking.
- If a next step is helpful but outside scope, suggest it rather than doing it automatically.