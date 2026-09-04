# MCP Sandbox Boundary Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the sandbox-boundary validation gap on the 3 MCP tool entrypoints (`ncgo_check`, `ncgo_add_domain`, `ncgo_import`) left out of PR #106's fix for Issue #97.

**Architecture:** Reuse the existing `sandboxRoot()` helper (`internal/mcp/sandbox.go`) — call it in each of the 3 handlers right after `args.Root` gets its default, before the handler does any work with it. This mirrors the exact pattern PR #106 already applied to 8 other tools. No new validation logic.

**Tech Stack:** Go, `internal/mcp` package, existing `EncodeMessage`/`DecodeResponses`/`New(...).Serve(...)` test harness.

**Spec:** `docs/superpowers/specs/2026-09-04-mcp-sandbox-boundary-followup-design.md`

## Global Constraints

- Reuse `sandboxRoot()` exactly as-is — no new validation logic (spec: Design section).
- Follow the PR #106 call pattern: `if _, err := sandboxRoot(args.Root); err != nil { return textResult(err.Error(), true), nil }`.
- Extend the existing `TestSandboxRootRejectsEscapePaths` table in `internal/mcp/tool_sandbox_test.go` — no new test file.
- Do not add new lint/CI enforcement (out of scope per spec).
- Existing MCP integration tests must keep passing.

---

### Task 1: Add sandboxRoot() to ncgo_check, ncgo_add_domain, ncgo_import + extend rejection tests

**Files:**
- Modify: `internal/mcp/tool_check.go` (`callCheck`, around line 24-35)
- Modify: `internal/mcp/tool_domain.go` (`callAddDomain`, around line 27-53)
- Modify: `internal/mcp/tool_import.go` (`callImport`, around line 20-38)
- Test: `internal/mcp/tool_sandbox_test.go` (extend `TestSandboxRootRejectsEscapePaths`)

**Interfaces:**
- Consumes: `sandboxRoot(target string) (string, error)` from `internal/mcp/sandbox.go` (already exported at package scope, unchanged signature).
- Produces: nothing new consumed by later tasks — Task 2 only touches a comment in `sandbox.go`.

- [ ] **Step 1: Write the failing test — extend the rejection-path table**

Edit `internal/mcp/tool_sandbox_test.go`, add 3 entries to the `cases` slice (after the existing `ncgo_add_method` entry, before the closing `}`):

```go
		{name: "ncgo_check", args: map[string]any{}},
		{name: "ncgo_add_domain", args: map[string]any{"name": "device"}},
		{name: "ncgo_import", args: map[string]any{}},
```

Full updated slice for reference (only the new lines are additions — everything else in the file is unchanged):

```go
	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "ncgo_new", args: map[string]any{"name": "demo", "module": "github.com/x/demo", "noGenerate": true}},
		{name: "ncgo_i18n_report", args: map[string]any{}},
		{name: "ncgo_i18n_check", args: map[string]any{}},
		{name: "ncgo_protolint", args: map[string]any{"files": []string{"app/demo.proto"}}},
		{name: "ncgo_doctor", args: map[string]any{}},
		{name: "ncgo_add_rule_center", args: map[string]any{"addr": "127.0.0.1:8888"}},
		{name: "ncgo_ai_sync", args: map[string]any{}},
		{name: "ncgo_ai_init_claude", args: map[string]any{}},
		{name: "ncgo_ai_context", args: map[string]any{}},
		{name: "ncgo_add_infra", args: map[string]any{"kind": "redis"}},
		{name: "ncgo_add_method", args: map[string]any{"spec": "device.Get"}},
		{name: "ncgo_check", args: map[string]any{}},
		{name: "ncgo_add_domain", args: map[string]any{"name": "device"}},
		{name: "ncgo_import", args: map[string]any{}},
	}
```

No other changes needed in the test file — the existing loop already builds `args["root"] = escapePath` for every case (the `rootKey` special-case only applies to `ncgo_new`, which uses `dir`), calls the tool through the JSON-RPC harness, and asserts `isError` is true with `"outside the workspace"` in the text. All 3 new tools accept `root` and go through `callCheck`/`callAddDomain`/`callImport`, matching the harness's existing assumptions.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/mcp/... -run TestSandboxRootRejectsEscapePaths -v`

Expected: FAIL on the 6 new subtests (`ncgo_check/absolute`, `ncgo_check/relative`, `ncgo_add_domain/absolute`, `ncgo_add_domain/relative`, `ncgo_import/absolute`, `ncgo_import/relative`) — each fails with "unexpectedly succeeded" because the handlers don't yet call `sandboxRoot`.

- [ ] **Step 3: Implement — add sandboxRoot() to tool_check.go**

In `internal/mcp/tool_check.go`, inside `callCheck`, insert the check right after `args.Root` is defaulted and before `runCheckReport` is called:

```go
func callCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}
	output, err := checkMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	rep, err := runCheckReport(args.Root)
	if err != nil {
		return textResult("ncgo_check: "+err.Error(), true), nil
	}
	out, err := checkMCPTool.buildResult(rep, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}
```

- [ ] **Step 4: Implement — add sandboxRoot() to tool_domain.go**

In `internal/mcp/tool_domain.go`, inside `callAddDomain`, insert the check right after `args.Root` is defaulted, before `addDomainMCPTool.resolveOutput` and `domain.Add(...)`:

```go
func callAddDomain(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Name   string `json:"name"`
		Root   string `json:"root"`
		Force  bool   `json:"force"`
		DryRun bool   `json:"dryRun"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.Name == "" {
		return textResult("name is required", true), nil
	}
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	output, err := addDomainMCPTool.resolveOutput(args.Output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err := domain.Add(domain.Options{
		Root:   args.Root,
		Name:   args.Name,
		Force:  args.Force,
		DryRun: args.DryRun,
	})
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	out, err := addDomainMCPTool.buildResult(res, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return out, nil
}
```

- [ ] **Step 5: Implement — add sandboxRoot() to tool_import.go**

In `internal/mcp/tool_import.go`, inside `callImport`, insert the check right after `args.Root` is defaulted, before `runImportDetect(...)`:

```go
func callImport(raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Root == "" {
		args.Root = "."
	}
	if _, err := sandboxRoot(args.Root); err != nil {
		return textResult(err.Error(), true), nil
	}

	m, err := runImportDetect(importDetectOptions{
		Root:          args.Root,
		Kind:          args.Kind,
		NCGOVersion:   ncgoVersion,
		AssetsVersion: assetsVersion,
	})
	if err != nil {
		return textResult("ncgo_import: "+err.Error(), true), nil
	}

	b, err := yaml.Marshal(m)
	if err != nil {
		return textResult("ncgo_import: marshal preview: "+err.Error(), true), nil
	}
	text := fmt.Sprintf("Preview of generated manifest (MCP is always preview-only; run `ncgo import` locally to write it):\n\n%s", string(b))

	return buildMCPResult(text, false, buildImportPreviewFields(m)), nil
}
```

Note: `callImport` returns `(map[string]any, error)` on the `textResult(...), nil` path exactly like the other two — matches the existing error-return convention in this file (see the other `textResult(..., true), nil` returns already in this function).

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/mcp/... -run TestSandboxRootRejectsEscapePaths -v`

Expected: PASS, all subtests including the 6 new ones.

- [ ] **Step 7: Run the full MCP package test suite**

Run: `go test ./internal/mcp/... -count=1`

Expected: PASS — confirms `callCheck`/`callAddDomain`/`callImport`'s existing (non-escape) test cases still succeed with the new sandboxRoot check in place (their `root` values are already within the workspace, e.g. `.` or unset).

- [ ] **Step 8: Commit**

```bash
git add internal/mcp/tool_check.go internal/mcp/tool_domain.go internal/mcp/tool_import.go internal/mcp/tool_sandbox_test.go
git commit -m "fix(mcp): enforce sandboxRoot on ncgo_check/ncgo_add_domain/ncgo_import"
```

---

### Task 2: Fix the stale sandboxRoot comment in sandbox.go

**Files:**
- Modify: `internal/mcp/sandbox.go:42-43`

**Interfaces:**
- Consumes: nothing (comment-only change).
- Produces: nothing consumed by other tasks.

- [ ] **Step 1: Update the comment**

In `internal/mcp/sandbox.go`, replace the comment above `sandboxRoot` (currently claims "It is called from every MCP tool handler that accepts a root or dir parameter" — untrue both before and after Issue #97/#107, since new tools can still forget to call it):

Before:
```go
// sandboxRoot validates the target path stays within the workspace.
// It is called from every MCP tool handler that accepts a root or dir parameter.
func sandboxRoot(target string) (string, error) {
```

After:
```go
// sandboxRoot validates that target resolves within the current workspace
// (cwd), returning an error if it is an absolute path or a relative path
// that escapes it (e.g. via "../"). Every MCP tool handler that accepts a
// root/dir parameter MUST call this before using that value — see
// TestSandboxRootRejectsEscapePaths in tool_sandbox_test.go for the list of
// tools currently covered; a new tool handler that accepts root/dir needs
// a call here and a new case in that table.
func sandboxRoot(target string) (string, error) {
```

- [ ] **Step 2: Verify the package still builds and tests pass**

Run: `go build ./internal/mcp/... && go test ./internal/mcp/... -count=1`

Expected: PASS (comment-only change, no behavior impact).

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/sandbox.go
git commit -m "docs(mcp): fix stale sandboxRoot comment to name the actual enforcement point"
```

---

## Final Validation

After both tasks:

```bash
go build ./... && go build . && go vet ./... && go test ./... -count=1
```

Expected: PASS. This is the CI-equivalent check per `CLAUDE.md`.
