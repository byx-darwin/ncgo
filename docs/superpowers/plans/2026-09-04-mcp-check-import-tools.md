# ncgo_check / ncgo_import MCP Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ncgo_check` and `ncgo_import` MCP tools so the MCP surface matches the CLI's `ncgo check` and `ncgo import` commands, closing the CLI/MCP capability gap described in Issue #96.

**Architecture:** Extract the check-report logic (currently inline in `internal/cli/check.go`) into an exported `doctor.RunCheck` in `internal/doctor`, and the import-detection logic (currently inline in `internal/cli/import.go`) into a new `internal/importer` package, mirroring how `internal/upgrade` already separates domain logic from the `internal/cli` wrapper. Both `internal/cli` and the new `internal/mcp` tool files call these shared functions — this is required because `internal/mcp` never imports `internal/cli` (every existing MCP tool, e.g. `tool_doctor.go`, calls the underlying domain package directly). `ncgo_check` follows `tool_doctor.go`'s `structuredMCPTool[*doctor.Report]` pattern exactly (same `Report` type, same `WriteText`/`WriteJSON` renderers). `ncgo_import` is **always preview-only** through MCP — it never calls `manifest.Save` — matching the existing `ncgo_upgrade` MCP tool's "always plan mode" precedent (an explicit decision made with the user during Phase 1 brainstorming, to avoid an agent accidentally writing project files).

**Tech Stack:** Go 1.25, Cobra (CLI), stdio JSON-RPC 2.0 (MCP), `gopkg.in/yaml.v3` for manifest preview rendering.

**Spec:** No separate spec file — this is a "bounded" change per `superpowers:brainstorming` (both CLI flows already exist, both target packages already have counterparts to mirror: `internal/upgrade`, `tool_doctor.go`). The approved design is captured in this plan's Architecture section and the task list below.

## Global Constraints

- CLI flags, JSON output, MCP schemas (`content[0].text`, top-level structured fields), and generated file layouts are contract-sensitive (per `CLAUDE.md` → Key Contracts). `internal/cli/check.go` and `internal/cli/import.go`'s **externally observable behavior must not change** — this is a pure extract-and-reuse refactor plus new additive MCP tools.
- `internal/mcp` never imports `internal/cli` — new MCP tools call domain packages (`internal/doctor`, `internal/importer`) directly, same as every existing tool in `internal/mcp`.
- `ncgo_import` via MCP **never writes `.ncgo/manifest.yaml`** — it is unconditionally preview-only, regardless of any future arguments. This mirrors `ncgo_upgrade`'s existing "Plan: true always from MCP" behavior (`internal/mcp/tool_upgrade.go:30`).
- Keep English and Chinese docs aligned (`README.md`/`README.zh-CN.md`, `docs/examples.md`/`docs/examples.zh-CN.md`) per `CLAUDE.md` → Documentation.
- Follow existing code style exactly: `gofmt`-clean, small focused files, package-level `var run... = domain.Func` indirection in `internal/mcp` tool files for testability (see `internal/mcp/tool_doctor.go:11`, `runDoctorReport`).

---

### Task 1: Extract check-report logic into `internal/doctor`

**Files:**
- Create: `internal/doctor/check.go`
- Create: `internal/doctor/check_test.go`
- Modify: `internal/cli/check.go`
- Modify: `internal/cli/check_test.go`

**Interfaces:**
- Produces: `doctor.RunCheck(root string) (*doctor.Report, error)` — the same `*Report`/`Check` types `doctor.Run` already returns (`internal/doctor/doctor.go:69`, `:48`). Later tasks (Task 3) call this directly.
- Consumes: `manifest.Load(root string) (*manifest.Manifest, error)`, `scan.Scan(root string) (*scan.ScanResult, error)` — both already used by the current `internal/cli/check.go`.

- [ ] **Step 1: Write `internal/doctor/check_test.go`**

```go
package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedCheckProject builds a healthy mono service: manifest + one domain with
// a usecase file carrying anchors.
func seedCheckProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-test", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/x/demo",
		Service: manifest.Service{Name: "demo", Kind: manifest.KindHertz},
		Domains: []string{"device"},
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	usecase := filepath.Join(root, "internal", "usecase", "device", "device.go")
	if err := os.MkdirAll(filepath.Dir(usecase), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `package device

type UseCase struct{}

// ncgo:methods:start
// ncgo:methods:end
`
	if err := os.WriteFile(usecase, []byte(body), 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}
	return root
}

func TestRunCheckHealthyProject(t *testing.T) {
	root := seedCheckProject(t)
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("rep.OK() = false, want true; checks=%+v", rep.Checks)
	}
	if rep.Root != root || rep.Scope != ScopeService {
		t.Fatalf("rep.Root/Scope = %q/%q, want %q/%q", rep.Root, rep.Scope, root, ScopeService)
	}
	found := map[string]bool{}
	for _, c := range rep.Checks {
		found[c.ID] = true
	}
	for _, id := range []string{"check.anchor", "check.manifest.consistency", "check.context.stale"} {
		if !found[id] {
			t.Errorf("checks missing %s: %+v", id, rep.Checks)
		}
	}
}

func TestRunCheckBrokenAnchors(t *testing.T) {
	root := seedCheckProject(t)
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "// ncgo:methods:start\n", "")), 0o644); err != nil {
		t.Fatalf("rewrite usecase: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if rep.OK() {
		t.Fatal("rep.OK() = true, want false (broken anchors)")
	}
}

func TestRunCheckStaleContext(t *testing.T) {
	root := seedCheckProject(t)
	claude := filepath.Join(root, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n# Project Context for Claude Code\n\n## Project Facts\n\n- domains: `[device, ghost]`\n"
	if err := os.WriteFile(claude, []byte(content), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	rep, err := RunCheck(root)
	if err != nil {
		t.Fatalf("RunCheck: %v", err)
	}
	if rep.OK() {
		t.Fatal("rep.OK() = true, want false (stale context)")
	}
}

func TestRunCheckMissingManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := RunCheck(root); err == nil {
		t.Fatal("RunCheck should error when manifest is missing")
	}
}

func TestParseContextDomains(t *testing.T) {
	tests := []struct{ in, want string }{
		{"- domains: `[device, order]`", "device,order"},
		{"- domains: `[device]`", "device"},
		{"- domains: `[]`", ""},
		{"no domains line here", ""},
	}
	for _, tt := range tests {
		got := strings.Join(parseContextDomains(tt.in), ",")
		if got != tt.want {
			t.Errorf("parseContextDomains(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run the new test file to confirm it fails to compile (RunCheck doesn't exist yet)**

Run: `go test ./internal/doctor/... -run TestRunCheck -v`
Expected: FAIL — `undefined: RunCheck` (and `undefined: parseContextDomains`)

- [ ] **Step 3: Create `internal/doctor/check.go`**

```go
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scan"
)

// RunCheck validates AI context integrity and manifest consistency for the
// ncgo service rooted at root: it verifies that every usecase has paired
// // ncgo:methods anchors, that manifest domains match internal/usecase/*/
// directories, and that rendered AI context files' declared domains match
// the manifest. It returns an error only when root is not an ncgo service
// (e.g. missing or invalid manifest).
func RunCheck(root string) (*Report, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	s, err := scan.Scan(root)
	if err != nil {
		return nil, err
	}
	rep := &Report{Root: root, Scope: ScopeService}
	rep.Checks = append(rep.Checks, checkAnchors(s)...)
	rep.Checks = append(rep.Checks, checkConsistency(s)...)
	rep.Checks = append(rep.Checks, checkContextStale(root, m)...)
	rep.Summary = Summarize(rep.Checks)
	return rep, nil
}

func checkAnchors(s *scan.ScanResult) []Check {
	var out []Check
	bad := 0
	for _, d := range s.Domains {
		if d.UsecaseExists && !d.AnchorsOK {
			bad++
			out = append(out, Check{
				ID: "check.anchor", OK: false, Severity: SeverityError,
				Message: fmt.Sprintf("domain %s has unpaired method anchors", d.Name),
				Hint:    "run `ncgo add method <domain>.X` or fix the // ncgo:methods:start|end markers",
			})
		}
	}
	if bad == 0 {
		out = append(out, Check{
			ID: "check.anchor", OK: true, Severity: SeverityError,
			Message: "all usecase files have paired method anchors",
		})
	}
	return out
}

func checkConsistency(s *scan.ScanResult) []Check {
	var out []Check
	bad := 0
	for _, i := range s.Issues {
		if i.Kind != scan.IssueMissingUsecase && i.Kind != scan.IssueUndeclaredDomain {
			continue
		}
		bad++
		out = append(out, Check{
			ID: "check.manifest.consistency", OK: false, Severity: SeverityError,
			Message: i.Message, File: i.File,
		})
	}
	if bad == 0 {
		out = append(out, Check{
			ID: "check.manifest.consistency", OK: true, Severity: SeverityError,
			Message: "manifest domains match internal/usecase/*/ directories",
		})
	}
	return out
}

// checkContextStale compares the domains declared in a rendered context file
// (CLAUDE.md or AGENTS.md, whichever exists) against the current manifest. A
// mismatch means the AI context is stale (a domain was added/removed without
// re-running `ai sync`). Missing context files are skipped (not a failure).
func checkContextStale(root string, m *manifest.Manifest) []Check {
	path := ""
	for _, rel := range contextFileTargets() {
		if rel == ".claude/skills/ncgo-dev/SKILL.md" || rel == ".cursor/rules/ncgo.mdc" {
			continue // SKILL.md / .mdc do not carry the domains fact line
		}
		candidate := filepath.Join(root, rel)
		if pathExists(candidate) {
			path = candidate
			break
		}
	}
	if path == "" {
		return []Check{okContextCheck("no rendered context file present; nothing to compare")}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return []Check{{
			ID: "check.context.stale", OK: false, Severity: SeverityError,
			Message: fmt.Sprintf("read %s: %v", path, err), File: path,
		}}
	}
	rendered := parseContextDomains(string(body))
	if rendered == nil {
		return []Check{okContextCheck("context file has no domains fact line")}
	}
	if !sameStringSet(rendered, m.Domains) {
		return []Check{{
			ID: "check.context.stale", OK: false, Severity: SeverityError,
			Message: fmt.Sprintf("%s is stale: context declares domains %v, manifest has %v", filepath.Base(path), rendered, m.Domains),
			File:    path,
			Hint:    "run `ncgo ai sync --root .`",
		}}
	}
	return []Check{okContextCheck("AI context domains match manifest")}
}

func okContextCheck(msg string) Check {
	return Check{ID: "check.context.stale", OK: true, Severity: SeverityError, Message: msg}
}

// parseContextDomains extracts the domain list from a rendered context file's
// "- domains: [a, b]" fact line. Returns nil when the line is absent.
func parseContextDomains(content string) []string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- domains: `[") {
			continue
		}
		rest := strings.TrimPrefix(line, "- domains: `[")
		rest = strings.TrimSuffix(rest, "]`")
		if rest == "" {
			return []string{}
		}
		parts := strings.Split(rest, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// sameStringSet reports whether two slices contain the same strings
// (order-insensitive).
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

// contextFileTargets lists the context files ai sync renders and check audits.
func contextFileTargets() []string {
	return []string{
		"AGENTS.md",
		"CLAUDE.md",
		".claude/skills/ncgo-dev/SKILL.md",
		".claude/generated/project-context.md",
		".cursor/rules/ncgo.mdc",
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

- [ ] **Step 4: Run the new doctor tests to confirm they pass**

Run: `go test ./internal/doctor/... -run 'TestRunCheck|TestParseContextDomains' -v`
Expected: PASS

- [ ] **Step 5: Slim `internal/cli/check.go` down to a thin wrapper around `doctor.RunCheck`**

Replace the full file content with:

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

type checkOptions struct {
	root   string
	output string
}

// exitCodeError lets a command choose its process exit code explicitly.
type exitCodeError struct {
	code int
	msg  string
}

func (e *exitCodeError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("ncgo check: exited with code %d", e.code)
}

func newCheckCmd() *cobra.Command {
	opts := &checkOptions{}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate AI context integrity and manifest consistency",
		Long: "Verify that every usecase has paired // ncgo:methods anchors, that " +
			"manifest domains match internal/usecase/*/ directories, and that rendered " +
			"AI context files' declared domains match the manifest. Exits 0 on pass, 1 on " +
			"check failure, 2 on command error (e.g. root is not an ncgo service).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Service root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.output, "output", "text", "Output format: text or json")
	return cmd
}

func runCheck(cmd *cobra.Command, opts *checkOptions) error {
	if opts.output == "" {
		opts.output = "text"
	}
	if opts.output != "text" && opts.output != "json" {
		return &exitCodeError{code: 2, msg: fmt.Sprintf("check: unsupported --output %q; want text or json", opts.output)}
	}
	rep, err := doctor.RunCheck(opts.root)
	if err != nil {
		return &exitCodeError{code: 2, msg: err.Error()}
	}
	switch opts.output {
	case "json":
		err = doctor.WriteJSON(cmd.OutOrStdout(), rep)
	default:
		err = doctor.WriteText(cmd.OutOrStdout(), rep)
	}
	if err != nil {
		return &exitCodeError{code: 2, msg: err.Error()}
	}
	if !rep.OK() {
		return &exitCodeError{code: 1}
	}
	return nil
}
```

- [ ] **Step 6: Remove the now-duplicated unit test from `internal/cli/check_test.go`**

Delete `TestParseContextDomains` from `internal/cli/check_test.go` (it moved to `internal/doctor/check_test.go` in Step 1). Keep every other test in that file unchanged — they exercise `runCheck`/`checkOptions`, which still exist in `internal/cli` and behave identically.

- [ ] **Step 7: Run all check-related tests to confirm behavior is preserved**

Run: `go test ./internal/cli/... ./internal/doctor/... -run 'Check' -v`
Expected: PASS — all `internal/cli` check tests (`TestRunCheckExitZeroOnHealthyProject`, `TestRunCheckExitOneOnBrokenAnchors`, `TestRunCheckExitOneOnStaleDomains`, `TestRunCheckExitOneOnStaleContext`, `TestRunCheckExitTwoOnMissingManifest`, `TestRunCheckJSONOutput`) and all new `internal/doctor` check tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/doctor/check.go internal/doctor/check_test.go internal/cli/check.go internal/cli/check_test.go
git commit -m "refactor(doctor): extract ncgo check report logic into doctor.RunCheck"
```

---

### Task 2: Extract import-detection logic into `internal/importer`

**Files:**
- Create: `internal/importer/importer.go`
- Create: `internal/importer/importer_test.go`
- Modify: `internal/cli/import.go`
- Modify: `internal/cli/import_test.go`

**Interfaces:**
- Produces: `importer.Options{Root, Kind, NCGOVersion, AssetsVersion string}` and `importer.Detect(opts importer.Options) (*manifest.Manifest, error)` — returns the manifest that would be written, without saving it. Later tasks (Task 4) call this directly.
- Consumes: `manifest.Path(root string) string`, `manifest.KindHertz`, `manifest.KindKitex`, `manifest.ModeMono` (all in `internal/manifest`, already imported by the current `internal/cli/import.go`).

- [ ] **Step 1: Write `internal/importer/importer_test.go`**

```go
package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestDetectModule(t *testing.T) {
	t.Run("extracts module path from go.mod", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "go.mod"), []byte("module github.com/test/myapp\n\ngo 1.25\n"))
		got, err := detectModule(dir)
		if err != nil {
			t.Fatalf("detectModule() error = %v", err)
		}
		if got != "github.com/test/myapp" {
			t.Fatalf("detectModule() = %q, want %q", got, "github.com/test/myapp")
		}
	})

	t.Run("errors when go.mod is missing", func(t *testing.T) {
		empty := t.TempDir()
		_, err := detectModule(empty)
		if err == nil {
			t.Fatal("detectModule() should error when go.mod is missing")
		}
	})

	t.Run("errors when go.mod has no module directive", func(t *testing.T) {
		d := t.TempDir()
		mustWriteFile(t, filepath.Join(d, "go.mod"), []byte("go 1.25\n"))
		_, err := detectModule(d)
		if err == nil {
			t.Fatal("detectModule() should error when no module directive")
		}
		if !strings.Contains(err.Error(), "module directive not found") {
			t.Fatalf("error = %q, want 'module directive not found'", err.Error())
		}
	})
}

func TestDetectKind(t *testing.T) {
	t.Run("detects hertz from router.go marker", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "router.go"), []byte("// Code generated by hz. DO NOT EDIT.\npackage main\n"))
		got, err := detectKind(dir)
		if err != nil {
			t.Fatalf("detectKind() error = %v", err)
		}
		if got != manifest.KindHertz {
			t.Fatalf("detectKind() = %q, want %q", got, manifest.KindHertz)
		}
	})

	t.Run("detects kitex from handler.go marker", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "handler.go"), []byte("// Code generated by kitex. DO NOT EDIT.\npackage main\n"))
		got, err := detectKind(dir)
		if err != nil {
			t.Fatalf("detectKind() error = %v", err)
		}
		if got != manifest.KindKitex {
			t.Fatalf("detectKind() = %q, want %q", got, manifest.KindKitex)
		}
	})

	t.Run("errors when neither marker is found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := detectKind(dir)
		if err == nil {
			t.Fatal("detectKind() should error when no markers found")
		}
		if !strings.Contains(err.Error(), "cannot detect service kind") {
			t.Fatalf("error = %q, want 'cannot detect service kind'", err.Error())
		}
	})
}

func TestDetectServiceName(t *testing.T) {
	t.Run("from IDL file", func(t *testing.T) {
		dir := t.TempDir()
		mustMkdirAll(t, filepath.Join(dir, "idl", "app"))
		mustWriteFile(t, filepath.Join(dir, "idl", "app", "user-api.proto"), []byte("syntax = \"proto3\";\n"))
		got, err := detectServiceName(dir, manifest.KindHertz)
		if err != nil {
			t.Fatalf("detectServiceName() error = %v", err)
		}
		if got != "user-api" {
			t.Fatalf("detectServiceName() = %q, want %q", got, "user-api")
		}
	})

	t.Run("fallback to directory name", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "myfallback")
		mustMkdirAll(t, sub)
		got, err := detectServiceName(sub, manifest.KindHertz)
		if err != nil {
			t.Fatalf("detectServiceName() error = %v", err)
		}
		if got != "myfallback" {
			t.Fatalf("detectServiceName() = %q, want %q", got, "myfallback")
		}
	})
}

func TestDetect(t *testing.T) {
	t.Run("dry preview shape for hertz project", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "go.mod"), []byte("module github.com/test/myapp\n\ngo 1.25\n"))
		mustWriteFile(t, filepath.Join(dir, "router.go"), []byte("// Code generated by hz. DO NOT EDIT.\npackage main\n"))

		m, err := Detect(Options{Root: dir, NCGOVersion: "0.1.0-test", AssetsVersion: "assets-test"})
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if m.Module != "github.com/test/myapp" {
			t.Fatalf("Module = %q, want %q", m.Module, "github.com/test/myapp")
		}
		if m.Service.Kind != manifest.KindHertz {
			t.Fatalf("Service.Kind = %q, want %q", m.Service.Kind, manifest.KindHertz)
		}
		if m.Mode != manifest.ModeMono {
			t.Fatalf("Mode = %q, want %q", m.Mode, manifest.ModeMono)
		}
		if m.Ncgo.Version != "0.1.0-test" || m.Ncgo.AssetsVersion != "assets-test" {
			t.Fatalf("Ncgo = %+v, want version/assets test values", m.Ncgo)
		}
	})

	t.Run("errors when manifest already exists", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "go.mod"), []byte("module github.com/test/myapp\n\ngo 1.25\n"))
		mustMkdirAll(t, filepath.Join(dir, ".ncgo"))
		mustWriteFile(t, filepath.Join(dir, ".ncgo", "manifest.yaml"), []byte("ncgo:\n  version: 0.1.0-dev\n"))

		_, err := Detect(Options{Root: dir})
		if err == nil {
			t.Fatal("Detect should reject when manifest already exists")
		}
		if !strings.Contains(err.Error(), "manifest already exists") {
			t.Fatalf("error = %q, want 'manifest already exists'", err.Error())
		}
	})

	t.Run("errors when go.mod is missing", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Detect(Options{Root: dir})
		if err == nil {
			t.Fatal("Detect should reject when go.mod is missing")
		}
		if !strings.Contains(err.Error(), "go.mod not found") {
			t.Fatalf("error = %q, want 'go.mod not found'", err.Error())
		}
	})

	t.Run("errors on invalid explicit kind", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "go.mod"), []byte("module github.com/test/myapp\n\ngo 1.25\n"))
		_, err := Detect(Options{Root: dir, Kind: "grpc"})
		if err == nil {
			t.Fatal("Detect should reject invalid Kind")
		}
		if !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("error = %q, want 'invalid'", err.Error())
		}
	})
}
```

- [ ] **Step 2: Run the new test file to confirm it fails to compile**

Run: `go test ./internal/importer/... -v`
Expected: FAIL — package `internal/importer` does not exist yet

- [ ] **Step 3: Create `internal/importer/importer.go`**

```go
// Package importer detects an existing Hertz/Kitex project's manifest fields
// without writing anything to disk. internal/cli's `ncgo import` command and
// internal/mcp's `ncgo_import` tool both build on Detect.
package importer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// Options configures Detect.
type Options struct {
	Root          string
	Kind          string // auto-detected if empty
	NCGOVersion   string
	AssetsVersion string
}

// Detect inspects an existing Go project at opts.Root and builds the
// manifest.Manifest that `ncgo import` would write, without saving it. It
// rejects roots that already have a manifest or lack a go.mod. Kind is
// auto-detected from generator marker files (router.go for Hertz,
// handler.go for Kitex) when opts.Kind is empty.
func Detect(opts Options) (*manifest.Manifest, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	manifestPath := manifest.Path(root)
	if _, err := os.Stat(manifestPath); err == nil {
		return nil, fmt.Errorf("manifest already exists at %s; remove it first or edit it manually", manifestPath)
	}

	goModPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("go.mod not found at %s; this command requires an existing Go module", goModPath)
	}

	module, err := detectModule(root)
	if err != nil {
		return nil, err
	}

	kind := opts.Kind
	if kind == "" {
		kind, err = detectKind(root)
		if err != nil {
			return nil, err
		}
	} else if kind != manifest.KindHertz && kind != manifest.KindKitex {
		return nil, fmt.Errorf("--kind %q is invalid (hertz|kitex)", kind)
	}

	serviceName, err := detectServiceName(root, kind)
	if err != nil {
		return nil, err
	}

	idlPath := detectIDLPath(root, serviceName)

	return &manifest.Manifest{
		Ncgo: manifest.Meta{
			Version:       opts.NCGOVersion,
			AssetsVersion: opts.AssetsVersion,
		},
		Mode:   manifest.ModeMono,
		Module: module,
		Service: manifest.Service{
			Name:         serviceName,
			Kind:         kind,
			WithDatabase: false,
			IDL:          idlPath,
		},
	}, nil
}

// detectModule reads the first "module" directive from go.mod.
func detectModule(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

// detectKind inspects source markers to determine whether the project is
// Hertz or Kitex.
func detectKind(root string) (string, error) {
	if isHertzProject(filepath.Join(root, "router.go")) {
		return manifest.KindHertz, nil
	}
	if isKitexProject(filepath.Join(root, "handler.go")) {
		return manifest.KindKitex, nil
	}
	return "", fmt.Errorf("cannot detect service kind: no router.go (hz marker) or handler.go (kitex marker) found; use --kind to specify")
}

func isHertzProject(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), "// Code generated by hz.")
}

func isKitexProject(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(b), "// Code generated by kitex.")
}

// detectServiceName tries to extract the service name from IDL files first,
// then from source code patterns, then falls back to the directory name.
func detectServiceName(root string, kind string) (string, error) {
	idlPath := detectIDLPath(root, "")
	if idlPath != "" {
		if name := serviceNameFromIDL(idlPath); name != "" {
			return name, nil
		}
	}
	switch kind {
	case manifest.KindHertz:
		if name := serviceNameFromHertz(root); name != "" {
			return name, nil
		}
	case manifest.KindKitex:
		if name := serviceNameFromKitex(root); name != "" {
			return name, nil
		}
	}
	return filepath.Base(root), nil
}

func serviceNameFromIDL(idlPath string) string {
	base := filepath.Base(idlPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.TrimPrefix(name, "service_")
	return name
}

// serviceNameFromHertz tries to extract the service name from router.go's
// import paths or registration patterns.
func serviceNameFromHertz(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "router.go"))
	if err != nil {
		return ""
	}
	content := string(b)
	re := regexp.MustCompile(`"\S+/biz/router"`)
	match := re.FindString(content)
	if match != "" {
		match = strings.Trim(match, `"`)
		parts := strings.Split(match, "/")
		for i, p := range parts {
			if p == "biz" && i > 0 {
				return parts[i-1]
			}
		}
	}
	return ""
}

// serviceNameFromKitex tries to extract the service name from handler.go's
// NewServer call: <ServiceName>.NewServer(.
func serviceNameFromKitex(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "handler.go"))
	if err != nil {
		return ""
	}
	content := string(b)
	re := regexp.MustCompile(`(\w+)\.NewServer\(`)
	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// detectIDLPath searches the idl/ directory for .proto or .thrift files.
// If serviceName is non-empty, prioritizes idl/app/<name>.proto.
func detectIDLPath(root string, serviceName string) string {
	if serviceName != "" {
		candidates := []string{
			filepath.Join("idl", "app", serviceName+".proto"),
			filepath.Join("idl", "app", serviceName+".thrift"),
			filepath.Join("idl", serviceName+".proto"),
			filepath.Join("idl", serviceName+".thrift"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(root, c)); err == nil {
				return c
			}
		}
	}
	var found string
	idlDir := filepath.Join(root, "idl")
	err := filepath.WalkDir(idlDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || found != "" {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext == ".proto" || ext == ".thrift" {
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil {
				found = rel
			}
		}
		return nil
	})
	if err != nil {
		return ""
	}
	return found
}
```

- [ ] **Step 4: Run the new importer tests to confirm they pass**

Run: `go test ./internal/importer/... -v`
Expected: PASS

- [ ] **Step 5: Slim `internal/cli/import.go` down to a thin wrapper around `importer.Detect`**

Replace the full file content with:

```go
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/importer"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

type importOptions struct {
	Root   string
	Kind   string // auto-detected if empty
	DryRun bool
}

func newImportCmd() *cobra.Command {
	opts := &importOptions{}
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an existing hz/kitex project into ncgo",
		Long: `Detect an existing Hertz or Kitex project and generate .ncgo/manifest.yaml.

ncgo import detects project type from generated markers in the source code,
extracts the Go module path from go.mod, and writes a minimal manifest.
This allows existing projects to use ncgo doctor, ncgo ai sync, and MCP tools.

Kind auto-detection looks for generator marker files: router.go containing
"// Code generated by hz." (Hertz) or handler.go containing
"// Code generated by kitex." (Kitex). Projects scaffolded with
` + "`ncgo new --no-generate`" + ` have not run the generators yet, so they have
no marker files; import such projects with an explicit --kind flag
(for example: ncgo import --root . --kind kitex).`,
		Example: `  ncgo import
  ncgo import --root ./my-service
  ncgo import --root . --dry-run`,
		RunE: runImport(opts),
	}
	cmd.Flags().StringVar(&opts.Root, "root", ".", "Project root")
	cmd.Flags().StringVar(&opts.Kind, "kind", "", "Service kind (hertz or kitex); auto-detected if empty")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview without writing files")
	return cmd
}

func runImport(opts *importOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		m, err := importer.Detect(importer.Options{
			Root:          opts.Root,
			Kind:          opts.Kind,
			NCGOVersion:   Version,
			AssetsVersion: assets.Version(),
		})
		if err != nil {
			return fmt.Errorf("import: %w", err)
		}

		if opts.DryRun {
			out := cmd.OutOrStdout()
			fmt.Fprint(out, "Preview of generated manifest:\n\n")
			b, err := yaml.Marshal(m)
			if err != nil {
				return fmt.Errorf("import: marshal preview: %w", err)
			}
			fmt.Fprint(out, string(b))
			return nil
		}

		root, err := filepath.Abs(opts.Root)
		if err != nil {
			return fmt.Errorf("import: resolve root: %w", err)
		}
		if err := manifest.Save(root, m); err != nil {
			return fmt.Errorf("import: %w", err)
		}

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "✓ Created .ncgo/manifest.yaml\n\n")
		fmt.Fprintln(out, "Next steps:")
		fmt.Fprintln(out, "  ncgo doctor                    # Check project health")
		fmt.Fprintln(out, "  ncgo ai sync                   # Generate AI context files")
		fmt.Fprintln(out, "  ncgo add infra redis --dry-run # Preview optional infrastructure")
		return nil
	}
}
```

- [ ] **Step 6: Remove the now-duplicated unit tests from `internal/cli/import_test.go`**

Delete these tests from `internal/cli/import_test.go` (they moved to `internal/importer/importer_test.go` in Step 1, whitebox-testing the unexported `detect*`/`serviceNameFrom*` helpers that no longer live in package `cli`):
`TestDetectModule`, `TestDetectKind`, `TestDetectServiceName`, `TestDetectIDLPath`, `TestServiceNameFromHertz`, `TestServiceNameFromIDLStripsServicePrefix`, `TestServiceNameFromKitex`.

Also delete the now-unused `mustWriteFile`/`mustMkdirAll` helpers **only if** no remaining test in the file uses them — check first with `grep -n "mustWriteFile\|mustMkdirAll" internal/cli/import_test.go`; the remaining CLI-level tests (`TestImportDryRun`, `TestImportRejectsExistingManifest`, `TestImportRejectsMissingGoMod`, `TestImportFullWrite`, `TestImportExplicitKind`, `TestImportInvalidKind`, `TestImportNoDetectableKind`, `TestRootCmdIncludesImportCommand`, `TestImportHelpDocumentsKindDetection`) do use them, so keep both helpers.

Keep all other tests in `internal/cli/import_test.go` unchanged — they exercise `runImport`/`importOptions`/`newImportCmd`, which still exist in `internal/cli` and behave identically.

- [ ] **Step 7: Run all import-related tests to confirm behavior is preserved**

Run: `go test ./internal/cli/... ./internal/importer/... -run 'Import|Detect|ServiceName' -v`
Expected: PASS — all remaining `internal/cli` import tests and all `internal/importer` tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/importer/importer.go internal/importer/importer_test.go internal/cli/import.go internal/cli/import_test.go
git commit -m "refactor(importer): extract ncgo import detection logic into internal/importer"
```

---

### Task 3: Add the `ncgo_check` MCP tool

**Files:**
- Create: `internal/mcp/tool_check.go`
- Create: `internal/mcp/check_test.go`
- Modify: `internal/mcp/tools.go`

**Interfaces:**
- Consumes: `doctor.RunCheck(root string) (*doctor.Report, error)` (Task 1), `structuredMCPTool[T]`/`resolveMCPOutput`/`formatMCPOutput`/`buildMCPResult`/`textResult` (`internal/mcp/output.go`), `schemaObject`/`rootField`/`outputTextJSONField` (`internal/mcp/schema.go`).
- Produces: `callCheck(raw json.RawMessage) (map[string]any, error)` — registered as `"ncgo_check"` in `tools.go`'s `callTool` switch and tool list.

- [ ] **Step 1: Write `internal/mcp/check_test.go`**

```go
package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

func TestServeToolCallCheck(t *testing.T) {
	old := runCheckReport
	runCheckReport = func(string) (*doctor.Report, error) {
		return &doctor.Report{
			Root:  "/repo/demo",
			Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{
				CheckCount: 2, PassedCount: 1, FailedCount: 1, ErrorCount: 1,
			},
			Checks: []doctor.Check{
				{ID: "check.anchor", OK: true, Severity: doctor.SeverityError, Message: "all usecase files have paired method anchors"},
				{ID: "check.manifest.consistency", OK: false, Severity: doctor.SeverityError, Message: "manifest domains mismatch"},
			},
		}, nil
	}
	defer func() { runCheckReport = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_check", "arguments": map[string]any{"root": "/repo/demo"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("check failure unexpectedly succeeded: %+v", result)
	}
	if result["root"].(string) != "/repo/demo" || result["scope"].(string) != string(doctor.ScopeService) {
		t.Fatalf("header = %+v", result)
	}
	if result["ok"].(bool) {
		t.Fatalf("ok = true, want false")
	}
	if !strings.Contains(resultText(result), "manifest domains mismatch") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallCheckJSON(t *testing.T) {
	old := runCheckReport
	runCheckReport = func(string) (*doctor.Report, error) {
		return &doctor.Report{
			Root: "/repo/demo", Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{CheckCount: 1, PassedCount: 1},
			Checks:  []doctor.Check{{ID: "check.anchor", OK: true, Severity: doctor.SeverityError, Message: "ok"}},
		}, nil
	}
	defer func() { runCheckReport = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_check", "arguments": map[string]any{"root": "/repo/demo", "output": "json"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if result["isError"].(bool) {
		t.Fatalf("check unexpectedly failed: %+v", result)
	}
	if !strings.Contains(resultText(result), `"id": "check.anchor"`) {
		t.Fatalf("json content missing check id: %s", resultText(result))
	}
}

func TestServeToolCallCheckInvalidOutput(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_check", "arguments": map[string]any{"root": "/repo/demo", "output": "sarif"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("expected error for unsupported output=sarif, got: %+v", result)
	}
}
```

- [ ] **Step 2: Run the new test file to confirm it fails to compile**

Run: `go test ./internal/mcp/... -run TestServeToolCallCheck -v`
Expected: FAIL — `undefined: runCheckReport` (tool not registered yet, `ncgo_check` unknown to `callTool`)

- [ ] **Step 3: Create `internal/mcp/tool_check.go`**

```go
package mcp

import (
	"encoding/json"
	"io"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

var runCheckReport = doctor.RunCheck

var checkMCPTool = structuredMCPTool[*doctor.Report]{
	name:      "check",
	supported: []string{mcpOutputText, mcpOutputJSON},
	format:    formatMCPCheckOutput,
	fields:    mcpCheckFields,
	isError: func(rep *doctor.Report) bool {
		return !rep.OK()
	},
}

func callCheck(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Root == "" {
		args.Root = "."
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

func formatMCPCheckOutput(rep *doctor.Report, output string) (string, error) {
	return formatMCPOutput(output, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error { return doctor.WriteText(w, rep) },
		mcpOutputJSON: func(w io.Writer) error { return doctor.WriteJSON(w, rep) },
	})
}

func mcpCheckFields(rep *doctor.Report) map[string]any {
	return map[string]any{
		"root":    rep.Root,
		"scope":   rep.Scope,
		"summary": rep.Summary,
		"checks":  rep.Checks,
		"ok":      rep.OK(),
	}
}
```

- [ ] **Step 4: Register `ncgo_check` in `internal/mcp/tools.go`**

Add to the slice returned by `(s *Server) tools()`, immediately after the `ncgo_doctor` entry (`internal/mcp/tools.go:22`):

```go
		{Name: "ncgo_check", Description: "Validate AI context integrity and manifest consistency for an ncgo service (read-only).", InputSchema: schemaObject([]string{"root"}, rootField("Service root containing .ncgo/manifest.yaml"), outputTextJSONField())},
```

Add to the `switch p.Name` in `callTool` (`internal/mcp/tools.go`), immediately after the `case "ncgo_doctor":` case:

```go
	case "ncgo_check":
		return callCheck(p.Arguments)
```

- [ ] **Step 5: Run the new MCP tests to confirm they pass**

Run: `go test ./internal/mcp/... -run TestServeToolCallCheck -v`
Expected: PASS

- [ ] **Step 6: Run the full MCP package test suite to confirm no regression**

Run: `go test ./internal/mcp/... -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/mcp/tool_check.go internal/mcp/check_test.go internal/mcp/tools.go
git commit -m "feat(mcp): add ncgo_check MCP tool"
```

---

### Task 4: Add the `ncgo_import` MCP tool (preview-only)

**Files:**
- Create: `internal/mcp/tool_import.go`
- Create: `internal/mcp/import_test.go`
- Modify: `internal/mcp/tools.go`

**Interfaces:**
- Consumes: `importer.Detect(opts importer.Options) (*manifest.Manifest, error)` (Task 2), `textResult`/`buildMCPResult` (`internal/mcp/output.go`), `schemaObject`/`rootField`/`enumField` (`internal/mcp/schema.go`).
- Produces: `callImport(raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error)` — registered as `"ncgo_import"` in `tools.go`'s `callTool` switch and tool list. **Never calls `manifest.Save`.**

- [ ] **Step 1: Write `internal/mcp/import_test.go`**

```go
package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestServeToolCallImport(t *testing.T) {
	old := runImportDetect
	runImportDetect = func(opts importDetectOptions) (*manifest.Manifest, error) {
		return &manifest.Manifest{
			Ncgo:    manifest.Meta{Version: "test-version", AssetsVersion: "test-assets"},
			Mode:    manifest.ModeMono,
			Module:  "github.com/acme/user-api",
			Service: manifest.Service{Name: "user-api", Kind: manifest.KindHertz},
		}, nil
	}
	defer func() { runImportDetect = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_import", "arguments": map[string]any{"root": "/repo/user-api"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if result["isError"].(bool) {
		t.Fatalf("import preview unexpectedly failed: %+v", result)
	}
	if result["preview"].(bool) != true {
		t.Fatalf("preview = %v, want true", result["preview"])
	}
	if result["module"].(string) != "github.com/acme/user-api" {
		t.Fatalf("module = %v, want github.com/acme/user-api", result["module"])
	}
	svc, ok := result["service"].(map[string]any)
	if !ok {
		t.Fatalf("result missing service field or wrong type: %+v", result)
	}
	if svc["kind"].(string) != manifest.KindHertz {
		t.Fatalf("service.kind = %v, want %v", svc["kind"], manifest.KindHertz)
	}
	if !strings.Contains(resultText(result), "Preview of generated manifest") {
		t.Fatalf("content missing preview header: %s", resultText(result))
	}
	if !strings.Contains(resultText(result), "github.com/acme/user-api") {
		t.Fatalf("content missing module path: %s", resultText(result))
	}
}

func TestServeToolCallImportError(t *testing.T) {
	old := runImportDetect
	runImportDetect = func(opts importDetectOptions) (*manifest.Manifest, error) {
		return nil, errImportFixture
	}
	defer func() { runImportDetect = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_import", "arguments": map[string]any{"root": "/repo/no-go-mod"}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("expected error result, got: %+v", result)
	}
	if !strings.Contains(resultText(result), "ncgo_import:") {
		t.Fatalf("content missing tool prefix: %s", resultText(result))
	}
}
```

- [ ] **Step 2: Add a fixture error variable used only by the test file**

Append to `internal/mcp/import_test.go` (same file, after the imports, before the first test function):

```go
var errImportFixture = errors.New("go.mod not found at /repo/no-go-mod/go.mod; this command requires an existing Go module")
```

Add `"errors"` to that file's import block.

- [ ] **Step 3: Run the new test file to confirm it fails to compile**

Run: `go test ./internal/mcp/... -run TestServeToolCallImport -v`
Expected: FAIL — `undefined: runImportDetect`, `undefined: importDetectOptions`

- [ ] **Step 4: Create `internal/mcp/tool_import.go`**

```go
package mcp

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/byx-darwin/ncgo/internal/importer"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// importDetectOptions mirrors importer.Options so tests can stub the
// detection call without depending on internal/importer's exported type
// name directly (keeps the indirection var's signature stable if
// importer.Options ever gains fields tool_import.go doesn't use).
type importDetectOptions = importer.Options

var runImportDetect = importer.Detect

// callImport previews the manifest `ncgo import` would generate for an
// existing hz/kitex project. It is always preview-only through MCP: it
// never calls manifest.Save, matching ncgo_upgrade's "always plan mode"
// behavior so an agent cannot accidentally write project files.
func callImport(raw json.RawMessage, ncgoVersion, assetsVersion string) (map[string]any, error) {
	var args struct {
		Root string `json:"root"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(raw, &args)
	if args.Root == "" {
		args.Root = "."
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

func buildImportPreviewFields(m *manifest.Manifest) map[string]any {
	return map[string]any{
		"preview": true,
		"module":  m.Module,
		"mode":    m.Mode,
		"service": map[string]any{
			"name":         m.Service.Name,
			"kind":         m.Service.Kind,
			"withDatabase": m.Service.WithDatabase,
			"idl":          m.Service.IDL,
		},
	}
}
```

- [ ] **Step 5: Register `ncgo_import` in `internal/mcp/tools.go`**

Add to the slice returned by `(s *Server) tools()`, immediately after the `ncgo_upgrade` entry (`internal/mcp/tools.go:34`):

```go
		{Name: "ncgo_import", Description: "Preview the .ncgo/manifest.yaml an existing hz/kitex project would import. Always preview-only via MCP; never writes files (run `ncgo import` locally to write).", InputSchema: schemaObject([]string{"root"}, rootField("Existing Go project root containing go.mod"), enumField("kind", []string{manifest.KindHertz, manifest.KindKitex}))},
```

Add to the `switch p.Name` in `callTool`, immediately after the `case "ncgo_upgrade":` case:

```go
	case "ncgo_import":
		return callImport(p.Arguments, s.NCGOVersion, s.AssetsVersion)
```

- [ ] **Step 6: Run the new MCP tests to confirm they pass**

Run: `go test ./internal/mcp/... -run TestServeToolCallImport -v`
Expected: PASS

- [ ] **Step 7: Run the full MCP package test suite to confirm no regression**

Run: `go test ./internal/mcp/... -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/mcp/tool_import.go internal/mcp/import_test.go internal/mcp/tools.go
git commit -m "feat(mcp): add ncgo_import MCP tool (preview-only)"
```

---

### Task 5: Update documentation (English + Chinese)

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/examples.md`
- Modify: `docs/examples.zh-CN.md`

**Interfaces:** None (docs only).

- [ ] **Step 1: Update `README.md`'s MCP tool list sentence**

Find this paragraph (around line 715-716):

```
`ncgo mcp serve` starts a stdio MCP server. It currently exposes
`ncgo_version`, `ncgo_doctor`, `ncgo_ai_init_claude`, `ncgo_ai_sync`,
`ncgo_i18n_report`, `ncgo_i18n_check`, `ncgo_protolint`, `ncgo_add_infra`,
`ncgo_add_method`, and `ncgo_ai_context` tools. `ncgo_ai_context` scans real
code and returns structured domains/methods/anchors/consistency for agents.
```

Replace with:

```
`ncgo mcp serve` starts a stdio MCP server. It currently exposes
`ncgo_version`, `ncgo_doctor`, `ncgo_check`, `ncgo_ai_init_claude`, `ncgo_ai_sync`,
`ncgo_i18n_report`, `ncgo_i18n_check`, `ncgo_protolint`, `ncgo_add_infra`,
`ncgo_add_method`, `ncgo_import`, and `ncgo_ai_context` tools. `ncgo_ai_context` scans real
code and returns structured domains/methods/anchors/consistency for agents.
`ncgo_check` mirrors `ncgo check` (read-only AI context/manifest validation).
`ncgo_import` is always preview-only through MCP — it never writes
`.ncgo/manifest.yaml`, unlike the CLI's `ncgo import`; run `ncgo import`
locally to actually write the file.
```

- [ ] **Step 2: Update `README.zh-CN.md`'s equivalent MCP tool list sentence**

Find the equivalent paragraph in `README.zh-CN.md` (search for `ncgo mcp serve` and the same tool-name list) and apply the same additions: add `ncgo_check` after `ncgo_doctor`, add `ncgo_import` after `ncgo_add_method`, and add two sentences translated as:

```
`ncgo_check` 对应 `ncgo check`（只读的 AI 上下文/manifest 校验）。
`ncgo_import` 通过 MCP 调用时始终为预览模式——不会写入
`.ncgo/manifest.yaml`，这与 CLI 的 `ncgo import` 不同；如需真正写入文件，
请在本地运行 `ncgo import`。
```

- [ ] **Step 3: Add tool contract entries to `docs/examples.md`'s "0. MCP contract-first reference"**

In the `### Tool contracts` list (`docs/examples.md:25`), add two entries. Insert directly after the `ncgo_doctor` entry (`docs/examples.md:31-33`):

```
- `ncgo_check`
  - inputs: `root`, `output=text|json`
  - stable top-level fields: `root`, `scope`, `summary`, `checks`, `ok`
  - read-only; mirrors `ncgo check`'s exit-code semantics via `ok`/`isError`
```

Insert directly after the `ncgo_add_rule_center` entry (`docs/examples.md:74-76`):

```
- `ncgo_import`
  - inputs: `root`, optional `kind=hertz|kitex` (auto-detected if omitted)
  - stable top-level fields: `preview`, `module`, `mode`, `service`
    (`service.name`, `service.kind`, `service.withDatabase`, `service.idl`)
  - always preview-only: never writes `.ncgo/manifest.yaml`, even though the
    CLI's `ncgo import` does; `content[0].text` is a YAML preview of the
    manifest that would be written
```

- [ ] **Step 4: Add matching entries to `docs/examples.zh-CN.md`**

Locate the equivalent `### 工具契约` (or matching Chinese heading) section in `docs/examples.zh-CN.md` and insert translated entries in the same two positions:

```
- `ncgo_check`
  - 输入：`root`，`output=text|json`
  - 稳定顶层字段：`root`、`scope`、`summary`、`checks`、`ok`
  - 只读；通过 `ok`/`isError` 对应 `ncgo check` 的退出码语义
```

```
- `ncgo_import`
  - 输入：`root`，可选 `kind=hertz|kitex`（留空则自动检测）
  - 稳定顶层字段：`preview`、`module`、`mode`、`service`
    （`service.name`、`service.kind`、`service.withDatabase`、`service.idl`）
  - 始终只预览：不会写入 `.ncgo/manifest.yaml`（CLI 的 `ncgo import` 会写入）；
    `content[0].text` 是待写入 manifest 的 YAML 预览
```

- [ ] **Step 5: Run markdown diagnostics on the changed docs**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')` (confirms no stray `.go` changes are unformatted; docs have no lint step in this repo beyond manual review — re-read each edited section for broken markdown tables/lists).

- [ ] **Step 6: Commit**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: document ncgo_check and ncgo_import MCP tools"
```

---

### Task 6: Full repository validation

**Files:** None (validation only).

**Interfaces:** None.

- [ ] **Step 1: Build everything**

Run: `go build ./... && go build .`
Expected: no errors

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no errors

- [ ] **Step 3: Full test suite**

Run: `go test ./... -count=1`
Expected: PASS, including `internal/cli`, `internal/doctor`, `internal/importer`, `internal/mcp`

- [ ] **Step 4: Smoke test**

Run: `./scripts/smoke.sh`
Expected: PASS

- [ ] **Step 5: gofmt check**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: empty output (no unformatted files)

- [ ] **Step 6: Manual MCP sanity check (tools/list shape)**

Run:

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' | go run . mcp serve | tail -1 | grep -o '"name":"ncgo_check"\|"name":"ncgo_import"'
```

Expected: both `"name":"ncgo_check"` and `"name":"ncgo_import"` printed.

- [ ] **Step 7: No commit for this task** — validation only; if any step fails, return to the relevant task above, fix, and re-run the narrowest failing check before re-running this task's steps.
