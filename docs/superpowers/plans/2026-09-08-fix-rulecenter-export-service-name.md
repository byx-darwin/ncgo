# Fix rule-center.proto Service Name Export Collision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `exportIDLs()` from rewriting `rule-center.proto`'s fixed external `service RuleService` identifier when the exporting project's service name (e.g. `"Rule"`) happens to share a PascalCase prefix with it.

**Architecture:** Add a package-level list of "fixed external contract" IDL filenames (parallel to the existing `ExcludedPaths` var) and skip the `replaceServiceName` call — but not the file export or the `{{.Module}}` substitution — for any `idl/` file whose relative path is in that list.

**Tech Stack:** Go 1.25, standard library (`regexp`, `path/filepath`, `os`), Go `testing` package.

**Spec:** `.cache/workflows/designs/wf-2026-09-08-004-design.md`

## Global Constraints

- Do not modify `replaceServiceName` itself (the `typeRE`/`segRE` regex logic) — Issue #119's `\b` boundary fix in the `segRE` branch must stay untouched.
- Do not modify `idlTemplatePath` — this issue is about file *content* (`service RuleService` string), not file *path* parameterization.
- `rule-center.proto` must still be exported (unlike Hertz's `api.proto`, which is skipped entirely) and must still get `{{.Module}}` substitution — only the `replaceServiceName` call is skipped for it.
- Keep the diff minimal: one new package-level var, one new conditional in `exportIDLs`, one doc-comment update, one new test function. No unrelated refactors.

---

### Task 1: Add fixed-contract exclusion to exportIDLs

**Files:**
- Modify: `internal/scaffold/template/export.go:62-65` (add new var after `ExcludedPaths`)
- Modify: `internal/scaffold/template/export.go:149-180` (`exportIDLs` doc comment + body)
- Test: `internal/scaffold/template/export_test.go` (new test function, appended after `TestExport_IDL`, which ends at line 371)

**Interfaces:**
- Consumes: existing `exportIDLs(root string, opts ExportOptions) ([]string, error)`, existing `replaceServiceName(body, serviceName string) string`, existing test helper `writeFileExport(t *testing.T, root, rel, content string)` (defined at `export_test.go:219`).
- Produces: new package-level var `fixedContractIDLs []string` in `internal/scaffold/template/export.go`, consumed only within `exportIDLs`. No other task depends on this.

- [ ] **Step 1: Write the failing test**

Add this test function to `internal/scaffold/template/export_test.go`, right after the existing `TestExport_IDL` function (which currently ends at line 371, right before `func TestExport_MinimalHertz`):

```go
func TestExport_IDL_FixedContractNotRenamed(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n")
	writeFileExport(t, dir, "idl/rule-center.proto",
		"syntax = \"proto3\";\npackage ratelimit;\n"+
			"option go_package = \"github.com/acme/test/kitex_gen/api/ratelimit/v1;ratelimit\";\n"+
			"service RuleService {\n  rpc GetRule(GetRuleReq) returns (GetRuleResp);\n}\n")

	result, err := Export(ExportOptions{Root: dir, Kind: "kitex",
		Module: "github.com/acme/test", ServiceName: "Rule"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(result.IDLs) != 1 || result.IDLs[0] != "idl/rule-center.proto" {
		t.Fatalf("IDLs = %v", result.IDLs)
	}
	body, err := os.ReadFile(filepath.Join(dir, "template", "idl", "rule-center.proto"))
	if err != nil {
		t.Fatalf("exported idl missing: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "service RuleService {") {
		t.Errorf("fixed contract service name must survive export unchanged:\n%s", s)
	}
	if strings.Contains(s, "{{.ServiceName}}") {
		t.Errorf("fixed contract file must not be parameterized by project service name:\n%s", s)
	}
	if !strings.Contains(s, "{{.Module}}/kitex_gen/api/ratelimit/v1") {
		t.Errorf("module path must still be variabilized:\n%s", s)
	}
}
```

This test exercises the exact collision from Issue #120: `ServiceName: "Rule"` against a body containing `RuleService`, which is the PascalCase prefix collision that the `typeRE` branch of `replaceServiceName` currently mishandles.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestExport_IDL_FixedContractNotRenamed -v`

Expected: FAIL — the assertion `!strings.Contains(s, "{{.ServiceName}}")` fails because `replaceServiceName("...service RuleService {...", "Rule")` currently rewrites `RuleService` to `{{.ServiceName}}Service`.

- [ ] **Step 3: Write minimal implementation**

In `internal/scaffold/template/export.go`, add a new var immediately after `ExcludedPaths` (currently lines 62-65):

```go
// ExcludedPaths returns paths that should never be exported as templates.
var ExcludedPaths = []string{
	"internal/pb/", // hz-generated protobuf code
	"kitex_gen/",   // kitex-generated RPC stubs
}

// fixedContractIDLs lists idl/ files (relative to the idl/ root) whose
// service names are fixed external contracts and must not be rewritten by
// replaceServiceName, regardless of the exporting project's service name.
// Unlike ExcludedPaths/Hertz's api.proto skip, these files ARE exported —
// only the service-name substitution is skipped for them.
var fixedContractIDLs = []string{
	"rule-center.proto", // rule-center preset's fixed `service RuleService`
}
```

Then update the doc comment and body of `exportIDLs` (currently lines 149-180). The doc comment currently reads:

```go
// exportIDLs variabilizes the project's service IDL into template/idl/.
// hz standard support files (openapi/, validate/) stay embedded and are
// excluded. A missing idl/ dir is not an error.
func exportIDLs(root string, opts ExportOptions) ([]string, error) {
```

Replace it with:

```go
// exportIDLs variabilizes the project's service IDL into template/idl/.
// hz standard support files (openapi/, validate/) stay embedded and are
// excluded. Files listed in fixedContractIDLs are exported but keep their
// service name identifiers literal (see fixedContractIDLs doc). A missing
// idl/ dir is not an error.
func exportIDLs(root string, opts ExportOptions) ([]string, error) {
```

Then, inside the `filepath.Walk` callback, change this block (currently lines 175-180):

```go
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		body := regexp.MustCompile(regexp.QuoteMeta(opts.Module)).ReplaceAllString(string(content), "{{.Module}}")
		body = replaceServiceName(body, opts.ServiceName)
```

to:

```go
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		body := regexp.MustCompile(regexp.QuoteMeta(opts.Module)).ReplaceAllString(string(content), "{{.Module}}")
		if !isFixedContractIDL(rel) {
			body = replaceServiceName(body, opts.ServiceName)
		}
```

Add a small helper next to `fixedContractIDLs` (or directly below `exportIDLs` — place it right after the `exportIDLs` function, before `idlTemplatePath`, currently starting at line 195):

```go
// isFixedContractIDL reports whether rel (idl/-relative path) is a fixed
// external contract file whose service names must not be parameterized.
func isFixedContractIDL(rel string) bool {
	for _, p := range fixedContractIDLs {
		if rel == p {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/template/... -run TestExport_IDL_FixedContractNotRenamed -v`

Expected: PASS

- [ ] **Step 5: Run the full export package test suite to check for regressions**

Run: `go test ./internal/scaffold/template/... -count=1 -v`

Expected: PASS — in particular `TestExport_IDL`, `TestReplaceServiceName`, `TestReplaceServiceName_LowercaseDelimiterAdjacent` (Issue #119's regression test) must still pass unchanged, confirming this change is additive and doesn't touch `replaceServiceName`'s internals.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/template/export.go internal/scaffold/template/export_test.go
git commit -m "fix(export): keep rule-center.proto's fixed service name literal on export"
```

---

## Post-Task Validation (run once, after Task 1)

- [ ] `go build ./...` — confirm no compile breakage repo-wide
- [ ] `go vet ./...` — confirm no vet issues introduced
- [ ] `go test ./internal/scaffold/... -count=1` — confirm no golden test in `internal/scaffold/mono` or `internal/scaffold/rpc` regresses (none of them call `template.Export`, but this catches any indirect coupling)
- [ ] `gofmt -l internal/scaffold/template/export.go internal/scaffold/template/export_test.go` — expect empty output (no formatting diffs)

## Notes for the Executor

- There is no existing golden test coverage for `template.Export()` + rule-center content (confirmed via `grep -rn "rule-center" internal/scaffold/template/golden_test.go internal/scaffold/template/integration_test.go` returning no matches), so no golden snapshot needs updating for this fix — Task 1's new unit test is the only coverage this issue requires.
- This is a single, self-contained task; there is no need to split further or dispatch multiple subagents.
