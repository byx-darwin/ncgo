# MCP Sandbox Boundary — Follow-up for tool_check / tool_domain / tool_import

- Issue: #107 (follow-up scope from #97 / PR #106)
- Classification: bounded
- Status: approved

## Problem

PR #106 added `sandboxRoot()` validation to 8 MCP tool entrypoints that
accept `root`/`dir` but missed 3 more: `ncgo_check`, `ncgo_add_domain`,
`ncgo_import`. `ncgo_add_domain` writes files, so it's the highest-risk
gap — an unvalidated `root` (absolute path or `../` escape) could write
outside the workspace.

## Design

Reuse the existing `sandboxRoot()` helper in `internal/mcp/sandbox.go`.
No new validation logic.

- `internal/mcp/tool_check.go` (`callCheck`): call `sandboxRoot(args.Root)`
  after the `args.Root` default, before `runCheckReport(args.Root)`.
- `internal/mcp/tool_domain.go` (`callAddDomain`): call `sandboxRoot(args.Root)`
  after the `args.Root` default, before `domain.Add(...)`.
- `internal/mcp/tool_import.go` (`callImport`): call `sandboxRoot(args.Root)`
  after the `args.Root` default, before `runImportDetect(...)`.

Each call follows the PR #106 pattern:

```go
if _, err := sandboxRoot(args.Root); err != nil {
    return textResult(err.Error(), true), nil
}
```

## Testing

Extend the existing table-driven `TestSandboxRootRejectsEscapePaths` in
`internal/mcp/tool_sandbox_test.go` with 3 more cases (`ncgo_check`,
`ncgo_add_domain` with required `name`, `ncgo_import`), each exercised
against both an absolute-path escape and a `../` relative escape. No new
test file.

## Docs / comments

Update the comment on `sandboxRoot` in `internal/mcp/sandbox.go:43`,
which currently claims "every tool that accepts root/dir calls it" —
untrue both before and after #97. Replace with an accurate description
of the function's responsibility; do not add new lint/CI enforcement
(out of scope for this bounded fix — flagged as a possible follow-up in
the Issue, not a requirement).

## Out of scope

- New static-analysis/lint rule to catch future unvalidated tools.
- Any change to `sandboxRoot`'s validation semantics.
