# Fixed-Contract Go File Protection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent `Export()`'s Go-file export path (`fileToTemplate` → `replaceServiceName`) from mangling `rule-center`'s fixed identifiers (`RuleCenterClient`, `NewRuleCenterClient`, etc.) and its comment literal (`"rule-center.internal:8888"`) when an exporting project's `ServiceName` collides with them (e.g. `ServiceName: "Rule"`).

**Architecture:** Add a `fixedContractGoFiles` list (relative project paths) and an `isFixedContractGoFile` helper to `internal/scaffold/template/export.go`, mirroring the existing `fixedContractIDLs`/`isFixedContractIDL` pattern from #120. In `fileToTemplate`, skip the `replaceServiceName` call entirely for matched files — Module-path substitution and brace-escaping still run. No path parameterization is affected because this file's file rule (`internal/pkg/**/*.go` in `KitexRules()`) doesn't set `LoopService`.

**Tech Stack:** Go, `testing` package (table-driven + golden-style export tests already in `export_test.go`).

**Spec:** `docs/superpowers/specs/2026-09-09-fixed-contract-go-files-design.md`

## Global Constraints

- Whole-file exclusion only — no identifier/substring-level protection mechanism (the design doc confirms no content in the fixed file legitimately needs `ServiceName` templating).
- Must not alter `fixedContractIDLs`, `isFixedContractIDL`, or `idlTemplatePath` (IDL export path, #120).
- Must not alter the `\b`-boundary behavior of `replaceServiceName`'s lowercase branch for non-fixed files (#119).
- `{{.Module}}` substitution must still apply to fixed Go files (only `replaceServiceName` is skipped, not the whole `fileToTemplate` pipeline).

---

### Task 1: Add `fixedContractGoFiles` list and `isFixedContractGoFile` helper, wire into `fileToTemplate`

**Files:**
- Modify: `internal/scaffold/template/export.go` (new var + func near `fixedContractIDLs`/`isFixedContractIDL` at lines 76-94 and 216-228; wire-in inside `fileToTemplate` at lines 289-317)
- Test: `internal/scaffold/template/export_test.go`

**Interfaces:**
- Consumes: existing `fileToTemplate(_, absPath, relPath string, opts ExportOptions, rule FileRule, svcNameLower string) (*TemplateFile, error)`, existing `replaceServiceName(body, serviceName string) string`, existing `writeFileExport(t, root, rel, content string)` and `loadExportedTemplateByPath(t, root, kind, want string) *TemplateFile` test helpers (already in `export_test.go`).
- Produces: `var fixedContractGoFiles []string` and `func isFixedContractGoFile(rel string) bool` — no other task depends on these (single-task plan).

- [ ] **Step 1: Write the failing regression test**

Add to `internal/scaffold/template/export_test.go` (after `TestExport_IDL_FixedContractNotRenamed`, i.e. after line ~400):

```go
func TestExport_GoFile_FixedContractNotRenamed(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n")
	writeFileExport(t, dir, "internal/pkg/middleware/rule_center_client.go", `// Optional rule-center client for Kitex services.
//
// NewRuleCenterClient creates a Kitex client connected to the rule-center
// service at the given address (e.g. "rule-center.internal:8888").
package middleware

import (
	"fmt"

	"github.com/acme/test/internal/base/conf"
)

// RuleCenterConfig mirrors the timeout pattern of Kitex's generated client.
type RuleCenterConfig struct {
	Address string
}

// RuleCenterClient implements the rate-limit resolver.
type RuleCenterClient struct {
	cfg RuleCenterConfig
}

// NewRuleCenterClient creates a client for the given address.
func NewRuleCenterClient(address string) (*RuleCenterClient, error) {
	if address == "" {
		return nil, fmt.Errorf("rule_center: address is required")
	}
	_ = conf.RateLimitRuleConfig{}
	return &RuleCenterClient{cfg: RuleCenterConfig{Address: address}}, nil
}
`)

	result, err := Export(ExportOptions{Root: dir, Kind: "kitex",
		Module: "github.com/acme/test", ServiceName: "Rule"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	found := false
	for _, tp := range result.Templates {
		if tp == "internal/pkg/middleware/rule_center_client.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rule_center_client.go in exported templates, got %v", result.Templates)
	}

	tpl := loadExportedTemplateByPath(t, dir, "kitex", "internal/pkg/middleware/rule_center_client.go")
	s := tpl.Body

	if !strings.Contains(s, "type RuleCenterClient struct") {
		t.Errorf("fixed identifier RuleCenterClient must survive export unchanged:\n%s", s)
	}
	if !strings.Contains(s, "type RuleCenterConfig struct") {
		t.Errorf("fixed identifier RuleCenterConfig must survive export unchanged:\n%s", s)
	}
	if !strings.Contains(s, "func NewRuleCenterClient(address string)") {
		t.Errorf("fixed identifier NewRuleCenterClient must survive export unchanged:\n%s", s)
	}
	if !strings.Contains(s, `"rule-center.internal:8888"`) {
		t.Errorf("fixed comment literal rule-center.internal:8888 must survive export unchanged:\n%s", s)
	}
	if strings.Contains(s, "{{.ServiceName}}") {
		t.Errorf("fixed contract Go file must not be parameterized by project service name:\n%s", s)
	}
	if !strings.Contains(s, "{{.Module}}/internal/base/conf") {
		t.Errorf("module path must still be variabilized:\n%s", s)
	}
}

func TestExport_GoFile_NonFixedStillRenamed(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n")
	writeFileExport(t, dir, "internal/pkg/other/rule_helper.go", `package other

// RuleHelper is a normal, non-fixed-contract type.
type RuleHelper struct{}
`)

	result, err := Export(ExportOptions{Root: dir, Kind: "kitex",
		Module: "github.com/acme/test", ServiceName: "Rule"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	found := false
	for _, tp := range result.Templates {
		if tp == "internal/pkg/other/rule_helper.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rule_helper.go in exported templates, got %v", result.Templates)
	}

	tpl := loadExportedTemplateByPath(t, dir, "kitex", "internal/pkg/other/rule_helper.go")
	s := tpl.Body
	if !strings.Contains(s, "type {{.ServiceName}}Helper struct") {
		t.Errorf("non-fixed Go file must still have ServiceName parameterized:\n%s", s)
	}
}

func TestIsFixedContractGoFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"internal/pkg/middleware/rule_center_client.go", true},
		{"internal/pkg/middleware/other_client.go", false},
		{"internal/pkg/other/rule_helper.go", false},
		{"rule_center_client.go", false},
	}
	for _, tt := range tests {
		if got := isFixedContractGoFile(tt.path); got != tt.want {
			t.Errorf("isFixedContractGoFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scaffold/template/... -run 'TestExport_GoFile_FixedContractNotRenamed|TestExport_GoFile_NonFixedStillRenamed|TestIsFixedContractGoFile' -v -count=1`

Expected: `TestExport_GoFile_FixedContractNotRenamed` and `TestIsFixedContractGoFile` FAIL to compile/run because `isFixedContractGoFile` doesn't exist yet (compile error `undefined: isFixedContractGoFile`). `TestExport_GoFile_NonFixedStillRenamed` should currently PASS on its own (existing behavior), confirming it's a valid baseline once the package compiles again after Step 3.

- [ ] **Step 3: Implement `fixedContractGoFiles` and `isFixedContractGoFile`, wire into `fileToTemplate`**

In `internal/scaffold/template/export.go`, add after the `fixedContractIDLs` var block (after line 82, i.e. right after the closing `}` of `fixedContractIDLs`):

```go
// fixedContractGoFiles lists Go source files (relative to the project
// root) whose identifiers/strings are fixed external contracts and must
// not be rewritten by replaceServiceName during export, regardless of
// the exporting project's service name. Unlike fixedContractIDLs, these
// files' exported paths are already stable (the file rules matching them
// don't set LoopService), so only their body content needs protecting.
var fixedContractGoFiles = []string{
	"internal/pkg/middleware/rule_center_client.go", // rule-center preset's fixed RuleCenterClient/NewRuleCenterClient identifiers and address example
}
```

Add after `isFixedContractIDL` (after its closing `}`, currently ending at line 219):

```go
// isFixedContractGoFile reports whether rel (project-root-relative path)
// is a fixed external contract Go file whose identifiers/strings must not
// be parameterized by replaceServiceName.
func isFixedContractGoFile(rel string) bool {
	for _, p := range fixedContractGoFiles {
		if rel == p {
			return true
		}
	}
	return false
}
```

Modify `fileToTemplate` (lines 301-302) from:

```go
	// Replace service name identifiers
	body = replaceServiceName(body, opts.ServiceName)
```

to:

```go
	// Replace service name identifiers, unless this file is a fixed
	// external contract whose identifiers/strings must survive export
	// unchanged (see fixedContractGoFiles).
	if !isFixedContractGoFile(relPath) {
		body = replaceServiceName(body, opts.ServiceName)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/template/... -run 'TestExport_GoFile_FixedContractNotRenamed|TestExport_GoFile_NonFixedStillRenamed|TestIsFixedContractGoFile' -v -count=1`

Expected: all three PASS.

- [ ] **Step 5: Run full package tests to confirm no regressions (#119/#120 interference check)**

Run: `go test ./internal/scaffold/template/... -count=1 -v`

Expected: all tests PASS, including `TestExport_IDL_FixedContractNotRenamed` and `TestExport_Makefile_FixedContractIDLPathNotRenamed` (#120) and any `\b`-boundary tests for `replaceServiceName` (#119).

- [ ] **Step 6: Run repository-wide build and vet**

Run: `go build ./... && go vet ./internal/scaffold/template/...`

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/template/export.go internal/scaffold/template/export_test.go
git commit -m "fix(export): protect rule-center's fixed Go identifiers from replaceServiceName on export"
```

---

## Self-Review Notes

- **Spec coverage:** Design doc's three sections (Design, Testing, Out of scope) are each covered: `fixedContractGoFiles`/`isFixedContractGoFile` (Design) in Step 3; regression test + non-fixed-file test + existing #119/#120 suite rerun (Testing) in Steps 1, 4, 5; no changes to `fixedContractIDLs`/`\b`-boundary code (Out of scope) — Step 3's diff is additive plus a 2-line conditional, nothing else touched.
- **Placeholder scan:** none — all steps show exact code and exact commands.
- **Type consistency:** `isFixedContractGoFile(rel string) bool` signature matches its only call site in `fileToTemplate` (`relPath` is already `string`, matching the existing `relPath string` parameter) and its test (`tt.path` is `string`).
