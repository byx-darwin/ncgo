# Issue #48 Phase 2 — 动态层 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the dynamic layer for AI agent development with ncgo: a shared `internal/scan` package (go/parser code scanning), an MCP `ncgo_ai_context` tool, a `ncgo check` CLI command with 0/1/2 exit codes, and an `ai sync` generated-at timestamp marker.

**Architecture:** A new pure `internal/scan` package parses real code (domains/methods/anchors/consistency) and is consumed by both the MCP tool (`ncgo_ai_context`) and the CLI command (`ncgo check`). `ncgo check` reuses `doctor.Report`/`WriteJSON`/`WriteText` for output. `ai sync` gains a `<!-- ncgo:generated-at: <RFC3339> -->` marker that `ncgo check` compares against `manifest.GeneratedAt` for staleness detection.

**Tech Stack:** Go 1.25+, `go/parser`/`go/ast`/`go/token`, cobra (CLI), existing `internal/doctor`, `internal/ai`, `internal/manifest`, `internal/mcp` packages.

## Global Constraints

- Go files must be `gofmt`-clean; run `gofmt -l $(find . -name '*.go' -not -path './.git/*')`.
- Do not hand-edit downstream generated project files — fix templates/generators instead.
- CLI/MCP/template/doctor output surfaces are **contract-sensitive**; update tests + docs together.
- `ncgo check` exit codes: `0` pass / `1` check failed / `2` command error.
- `ncgo check` and `ncgo_ai_context` are **service-level** (mono). A workspace root (no `.ncgo/manifest.yaml`) is a command error.
- `ncgo_ai_context` and `ncgo check` do **not** cache; `ncgo check` has **no** `--target`.
- Keep EN and ZH docs aligned (README.md/README.zh-CN.md, docs/examples.md/docs/examples.zh-CN.md).
- Final validation: `go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh`.

---

### Task 1: `internal/scan` — shared code scanner

**Files:**
- Create: `internal/scan/scan.go`
- Create: `internal/scan/walk.go`
- Create: `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `manifest.Load(root)` (from `internal/manifest`).
- Produces:
  ```go
  package scan

  type Domain struct {
      Name           string   `json:"name"`
      ManifestListed bool     `json:"manifestListed"`
      UsecaseExists  bool     `json:"usecaseExists"`
      RepoExists     bool     `json:"repoExists"`
      Methods        []Method `json:"methods"`
      AnchorsOK      bool     `json:"anchorsOk"`
  }
  type Method struct {
      Name string `json:"name"`
      File string `json:"file,omitempty"`
      Line int    `json:"line,omitempty"`
  }
  type Issue struct {
      Kind    string `json:"kind"`
      Message string `json:"message"`
      File    string `json:"file,omitempty"`
  }
  type Scan struct {
      Root    string   `json:"root"`
      Domains []Domain `json:"domains"`
      Issues  []Issue  `json:"issues"`
  }
  func Scan(root string) (*Scan, error)
  ```
- Issue kind constants:
  ```go
  const (
      IssueMissingUsecase   = "missing_usecase"
      IssueUndeclaredDomain = "undeclared_domain"
      IssueAnchorMissing    = "anchor_missing"
      IssueAnchorUnpaired   = "anchor_unpaired"
  )
  ```
- Marker format constant (used by Task 2 to write, Task 4 to read):
  ```go
  const GeneratedAtMarker = "<!-- ncgo:generated-at: " // ends with " -->"
  ```

- [ ] **Step 1: Write the failing tests** — create `internal/scan/scan_test.go`

```go
package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// seedScanProject writes a manifest plus one domain's usecase/repository dirs.
func seedScanProject(t *testing.T, domains []string) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.Save(root, &manifest.Manifest{
		Ncgo:    manifest.Meta{Version: "0.1.0-test", AssetsVersion: "test"},
		Mode:    manifest.ModeMono,
		Module:  "github.com/x/demo",
		Service: manifest.Service{Name: "demo", Kind: manifest.KindHertz},
		Domains: domains,
	}); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	for _, d := range domains {
		usecase := filepath.Join(root, "internal", "usecase", d, d+".go")
		if err := os.MkdirAll(filepath.Dir(usecase), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		body := `package ` + d + `

type UseCase struct{}

func (u *UseCase) List() error { return nil }
func (u *UseCase) Repo() {}
// ncgo:methods:start
// ncgo:methods:end
`
		if err := os.WriteFile(usecase, []byte(body), 0o644); err != nil {
			t.Fatalf("write usecase: %v", err)
		}
	}
	return root
}

func TestScanReportsDomainsMethodsAnchors(t *testing.T) {
	root := seedScanProject(t, []string{"device", "order"})
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(s.Domains) != 2 {
		t.Fatalf("domains = %d, want 2", len(s.Domains))
	}
	byName := map[string]Domain{}
	for _, d := range s.Domains {
		byName[d.Name] = d
	}
	dev := byName["device"]
	if !dev.ManifestListed || !dev.UsecaseExists || !dev.AnchorsOK {
		t.Fatalf("device domain = %+v, want all true", dev)
	}
	if len(dev.Methods) != 1 || dev.Methods[0].Name != "List" {
		t.Fatalf("device methods = %+v, want [List] only (Repo excluded)", dev.Methods)
	}
}

func TestScanFlagsMissingUsecase(t *testing.T) {
	root := seedScanProject(t, []string{"device", "ghost"})
	os.Remove(filepath.Join(root, "internal", "usecase", "ghost"))
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssue(s.Issues, IssueMissingUsecase) {
		t.Fatalf("issues = %+v, want missing_usecase", s.Issues)
	}
}

func TestScanFlagsUndeclaredDomain(t *testing.T) {
	root := seedScanProject(t, []string{"device"})
	if err := os.MkdirAll(filepath.Join(root, "internal", "usecase", "rogue"), 0o755); err != nil {
		t.Fatalf("mkdir rogue: %v", err)
	}
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssue(s.Issues, IssueUndeclaredDomain) {
		t.Fatalf("issues = %+v, want undeclared_domain", s.Issues)
	}
}

func TestScanFlagsBrokenAnchors(t *testing.T) {
	root := seedScanProject(t, []string{"device"})
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "// ncgo:methods:start\n", "")), 0o644)
	s, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasIssue(s.Issues, IssueAnchorMissing) {
		t.Fatalf("issues = %+v, want anchor_missing", s.Issues)
	}
}

func TestScanErrorsOnMissingManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := Scan(root); err == nil {
		t.Fatal("Scan on non-project root should error")
	}
}

func hasIssue(issues []Issue, kind string) bool {
	for _, i := range issues {
		if i.Kind == kind {
			return true
		}
	}
	return false
}

// used by Task 4 via buildCheckReport; kept here to lock the format.
var _ = time.RFC3339
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/scan/ -count=1`
Expected: FAIL (package `scan` not yet defined).

- [ ] **Step 3: Create `internal/scan/walk.go`** — self-contained go/parser traversal

```go
package scan

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// walkGoFiles parses every non-test .go file under dir (recursively,
// skipping testdata/ and dot/underscore dirs) and invokes visit. Parse
// errors are skipped so an in-progress file does not break a scan.
func walkGoFiles(dir string, visit func(fset *token.FileSet, f *ast.File, path string)) error {
	fset := token.NewFileSet()
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if name == "testdata" || name == "vendor" || (len(name) > 1 && (name[0] == '.' || name[0] == '_')) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		visit(fset, f, path)
		return nil
	})
}
```

- [ ] **Step 4: Create `internal/scan/scan.go`**

```go
// Package scan inspects the real code of an ncgo service and reports what
// actually exists: domains, usecase methods, method-insertion anchors, and
// consistency between the manifest and the filesystem. It is consumed by the
// `ncgo_ai_context` MCP tool and the `ncgo check` CLI command.
package scan

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

const (
	// GeneratedAtMarker prefixes the timestamp line `ai sync` writes into
	// rendered context files; it ends with " -->". `ncgo check` reads it to
	// detect stale context.
	GeneratedAtMarker = "<!-- ncgo:generated-at: "

	startMarker = "// ncgo:methods:start"
	endMarker   = "// ncgo:methods:end"
)

const (
	IssueMissingUsecase   = "missing_usecase"
	IssueUndeclaredDomain = "undeclared_domain"
	IssueAnchorMissing    = "anchor_missing"
	IssueAnchorUnpaired   = "anchor_unpaired"
)

// Domain captures one domain's manifest vs on-disk state.
type Domain struct {
	Name           string   `json:"name"`
	ManifestListed bool     `json:"manifestListed"`
	UsecaseExists  bool     `json:"usecaseExists"`
	RepoExists     bool     `json:"repoExists"`
	Methods        []Method `json:"methods"`
	AnchorsOK      bool     `json:"anchorsOk"`
}

// Method is one usecase method found in real code.
type Method struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

// Issue is one manifest-vs-code inconsistency or anchor problem.
type Issue struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
}

// Scan is the structured result of inspecting one service root.
type Scan struct {
	Root    string   `json:"root"`
	Domains []Domain `json:"domains"`
	Issues  []Issue  `json:"issues"`
}

// Scan inspects the service at root. It returns an error only when the root
// is not an ncgo service (no .ncgo/manifest.yaml). Code-level inconsistencies
// are reported as Issues, not errors.
func Scan(root string) (*Scan, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	s := &Scan{Root: root}
	seen := map[string]bool{}
	usecaseDir := filepath.Join(root, "internal", "usecase")
	if entries, err := os.ReadDir(usecaseDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			seen[name] = true
			listed := domainListed(m, name)
			if !listed {
				s.Issues = append(s.Issues, Issue{
					Kind:    IssueUndeclaredDomain,
					Message: fmt.Sprintf("domain %q exists under internal/usecase but is not listed in manifest", name),
					File:    filepath.Join(usecaseDir, name),
				})
			}
			usecasePath := filepath.Join(usecaseDir, name, name+".go")
			usecaseExists := fileExists(usecasePath)
			methods, anchorsOK := scanUsecase(usecasePath)
			if usecaseExists && !anchorsOK {
				s.Issues = append(s.Issues, Issue{
					Kind:    anchorIssueKind(usecasePath),
					Message: "usecase file is missing or has unpaired // ncgo:methods:start|end anchors",
					File:    usecasePath,
				})
			}
			s.Domains = append(s.Domains, Domain{
				Name:           name,
				ManifestListed: listed,
				UsecaseExists:  usecaseExists,
				RepoExists:     fileExists(filepath.Join(root, "internal", "repository", name, name+".go")),
				Methods:        methods,
				AnchorsOK:      anchorsOK,
			})
		}
	}
	for _, name := range m.Domains {
		if seen[name] {
			continue
		}
		s.Issues = append(s.Issues, Issue{
			Kind:    IssueMissingUsecase,
			Message: fmt.Sprintf("domain %q is listed in manifest but has no internal/usecase/%s/%s.go", name, name, name),
			File:    filepath.Join(usecaseDir, name),
		})
		s.Domains = append(s.Domains, Domain{Name: name, ManifestListed: true, UsecaseExists: false})
	}
	sort.SliceStable(s.Domains, func(i, j int) bool { return s.Domains[i].Name < s.Domains[j].Name })
	return s, nil
}

// scanUsecase parses one usecase file for exported UseCase methods and
// validates the method-insertion anchors. Methods `New`, `Repo`, and other
// known accessors are excluded.
func scanUsecase(path string) ([]Method, bool) {
	var methods []Method
	anchorsOK := false
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	start := strings.Index(string(body), startMarker)
	end := strings.Index(string(body), endMarker)
	anchorsOK = start >= 0 && end >= start
	_ = walkGoFiles(filepath.Dir(path), func(fset *token.FileSet, f *ast.File, file string) {
		if filepath.Base(file) != filepath.Base(path) {
			return
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if receiverTypeName(fn.Recv.List[0].Type) != "UseCase" {
				continue
			}
			if isAccessor(fn.Name.Name) {
				continue
			}
			methods = append(methods, Method{
				Name: fn.Name.Name,
				File: file,
				Line: fset.Position(fn.Pos()).Line,
			})
		}
	})
	sort.SliceStable(methods, func(i, j int) bool { return methods[i].Name < methods[j].Name })
	return methods, anchorsOK
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

// isAccessor reports whether the method is a constructor or accessor that
// should not count as a domain method.
func isAccessor(name string) bool {
	switch name {
	case "New", "Repo":
		return true
	}
	return false
}

func anchorIssueKind(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return IssueAnchorMissing
	}
	if strings.Contains(string(body), startMarker) && strings.Contains(string(body), endMarker) {
		return IssueAnchorUnpaired
	}
	return IssueAnchorMissing
}

func domainListed(m *manifest.Manifest, name string) bool {
	for _, d := range m.Domains {
		if d == name {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/scan/ -count=1`
Expected: PASS (all 6 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/scan/
git commit -m "feat(scan): add shared code scanner (domains/methods/anchors/consistency)"
```

---

### Task 2: `ai sync` generated-at marker (contract change)

**Files:**
- Modify: `internal/ai/sync.go`
- Create: `internal/ai/generated_at.go`
- Modify: `internal/ai/sync_test.go`

**Interfaces:**
- Consumes: `scan.GeneratedAtMarker` (from Task 1).
- Produces:
  ```go
  package ai
  // ReadGeneratedAt parses the generated-at marker from a rendered context file.
  func ReadGeneratedAt(path string) (time.Time, bool)
  ```
- Behavior change: `writeTarget` now injects `<!-- ncgo:generated-at: <RFC3339> -->` immediately after the managed-marker line of every rendered target file.

- [ ] **Step 1: Write the failing test** — append to `internal/ai/sync_test.go`

```go
func TestSyncStampsGeneratedAtMarker(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, manifest.KindHertz)
	res, err := Sync(Options{Root: root, Target: TargetAll})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Written) == 0 {
		t.Fatal("expected files written")
	}
	for _, p := range []string{"AGENTS.md", "CLAUDE.md"} {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !strings.Contains(string(b), "<!-- ncgo:generated-at: ") {
			t.Errorf("%s missing generated-at marker:\n%s", p, b)
		}
	}
}

func TestReadGeneratedAt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n<!-- ncgo:generated-at: 2026-04-29T00:00:00Z -->\n# body\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ts, ok := ReadGeneratedAt(p)
	if !ok {
		t.Fatal("ReadGeneratedAt should find the marker")
	}
	if !ts.Equal(time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("ReadGeneratedAt = %v, want 2026-04-29T00:00:00Z", ts)
	}
}

func TestReadGeneratedAtMissingMarker(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(p, []byte("<!-- ncgo:managed -->\n# body\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := ReadGeneratedAt(p); ok {
		t.Fatal("ReadGeneratedAt should report no marker when absent")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ai/ -run 'TestSyncStampsGeneratedAtMarker|TestReadGeneratedAt' -count=1`
Expected: FAIL (no marker written, `ReadGeneratedAt` undefined).

- [ ] **Step 3: Create `internal/ai/generated_at.go`**

```go
package ai

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/scan"
)

// stampGeneratedAt injects a generated-at marker line immediately after the
// managed-marker line in rendered output. Returns rendered unchanged when the
// managed marker is absent (should not happen for ai sync targets).
func stampGeneratedAt(rendered string, ts time.Time) string {
	marker := scan.GeneratedAtMarker + ts.UTC().Format(time.RFC3339) + " -->"
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if strings.Contains(line, ManagedMarker) {
			lines = append(lines[:i+1], append([]string{marker}, lines[i+1:]...)...)
			break
		}
	}
	return strings.Join(lines, "\n")
}

// ReadGeneratedAt parses the `<!-- ncgo:generated-at: <RFC3339> -->` marker
// from a rendered context file. It reports (zero time, false) when the marker
// is absent or malformed.
func ReadGeneratedAt(path string) (time.Time, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, scan.GeneratedAtMarker) {
			continue
		}
		rest := strings.TrimPrefix(line, scan.GeneratedAtMarker)
		rest = strings.TrimSuffix(rest, " -->")
		rest = strings.TrimSuffix(rest, "-->")
		if n, err := strconv.ParseInt(rest, 10, 64); err == nil {
			// not RFC3339; treat as malformed
			_ = n
			return time.Time{}, false
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(rest))
		if err != nil {
			return time.Time{}, false
		}
		return ts, true
	}
	return time.Time{}, false
}
```

> **Note on `ReadGeneratedAt`:** the odd `strconv.ParseInt` branch is defensive — a bare timestamp without a colon is never valid RFC3339 and should not parse as a date. Keep it; it guards against a marker format drift. If `go vet`/linters flag `_ = n`, replace with `return time.Time{}, false`.

- [ ] **Step 4: Modify `internal/ai/sync.go` — `writeTarget`** to stamp the marker on write

In `writeTarget`, change the write block from:

```go
	if err := os.WriteFile(full, []byte(rendered), 0o644); err != nil {
```

to:

```go
	stamped := stampGeneratedAt(rendered, time.Now())
	if err := os.WriteFile(full, []byte(stamped), 0o644); err != nil {
```

Add the `"time"` import to `internal/ai/sync.go`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/ai/ -count=1`
Expected: PASS (all existing + 3 new tests).

- [ ] **Step 6: Commit**

```bash
git add internal/ai/sync.go internal/ai/generated_at.go internal/ai/sync_test.go
git commit -m "feat(ai): stamp ncgo:generated-at marker into ai sync output"
```

---

### Task 3: MCP `ncgo_ai_context` tool

**Files:**
- Create: `internal/mcp/tool_ai_context.go`
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/server_test.go`

**Interfaces:**
- Consumes: `scan.Scan`, `scan.Scan`/`Domain`/`Method`/`Issue` types (Task 1).
- Produces: `ncgo_ai_context` tool registered in `tools()`, dispatched in `callTool`.
- Schema: `{root: string (required), output: text|json}`.

- [ ] **Step 1: Write the failing tests** — append to `internal/mcp/server_test.go`

```go
func TestServeToolListHasAIContext(t *testing.T) {
	input := append(EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}),
		EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})...)
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	listed := responses[1].Result.(map[string]any)["tools"].([]any)
	var aiCtx map[string]any
	for _, item := range listed {
		if item.(map[string]any)["name"] == "ncgo_ai_context" {
			aiCtx = item.(map[string]any)
		}
	}
	if aiCtx == nil {
		t.Fatal("tools/list missing ncgo_ai_context")
	}
	props := aiCtx["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := props["root"]; !ok {
		t.Fatalf("ncgo_ai_context schema missing root: %+v", props)
	}
	if _, ok := props["output"]; !ok {
		t.Fatalf("ncgo_ai_context schema missing output: %+v", props)
	}
}

func TestServeToolCallAIContext(t *testing.T) {
	root := seedAIContextProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_context", "arguments": map[string]any{"root": root}},
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
		t.Fatalf("ncgo_ai_context returned error: %s", resultText(result))
	}
	domains, ok := result["domains"].([]any)
	if !ok || len(domains) == 0 {
		t.Fatalf("result missing domains field: %+v", result)
	}
	issues, ok := result["issues"].([]any)
	if !ok {
		t.Fatalf("result missing issues field: %+v", result)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %+v, want empty for healthy project", issues)
	}
	content := resultText(result)
	if !strings.Contains(content, "device") {
		t.Fatalf("content missing device domain: %s", content)
	}
}

func TestServeToolCallAIContextNoManifest(t *testing.T) {
	root := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_context", "arguments": map[string]any{"root": root}},
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
		t.Fatal("ncgo_ai_context on non-project root should be an error")
	}
}
```

Add the seeding helper near `seedMCPProject`:

```go
func seedAIContextProject(t *testing.T) string {
	t.Helper()
	root := seedMCPProject(t, manifest.KindHertz)
	usecase := filepath.Join(root, "internal", "usecase", "device", "device.go")
	if err := os.MkdirAll(filepath.Dir(usecase), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `package device

type UseCase struct{}

func (u *UseCase) List() error { return nil }
// ncgo:methods:start
// ncgo:methods:end
`
	if err := os.WriteFile(usecase, []byte(body), 0o644); err != nil {
		t.Fatalf("write usecase: %v", err)
	}
	return root
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/mcp/ -run 'TestServeTool.*AIContext' -count=1`
Expected: FAIL (`ncgo_ai_context` not registered, `seedAIContextProject` unused is fine at this stage but tests reference missing tool).

- [ ] **Step 3: Create `internal/mcp/tool_ai_context.go`**

```go
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/byx-darwin/ncgo/internal/scan"
)

// callAIContext scans real code and returns structured context for an ncgo
// service: domains (with file existence), methods, anchors, and issues.
func callAIContext(raw json.RawMessage) (map[string]any, error) {
	var args struct {
		Root   string `json:"root"`
		Output string `json:"output"`
	}
	_ = json.Unmarshal(raw, &args)
	s, err := scan.Scan(args.Root)
	if err != nil {
		return textResult("ncgo_ai_context: "+err.Error(), true), nil
	}
	output, err := resolveMCPOutput("ai_context", args.Output, mcpOutputText, mcpOutputJSON)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	text, err := formatAIContext(s, output)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	return buildMCPResult(text, false, mcpAIContextFields(s)), nil
}

func formatAIContext(s *scan.Scan, output string) (string, error) {
	switch output {
	case mcpOutputJSON:
		var buf strings.Builder
		if err := writeJSONOutput(&buf, s); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return formatAIContextText(s), nil
	}
}

// mcpAIContextFields exposes stable top-level fields so agents can read the
// scan without parsing content[0].text.
func mcpAIContextFields(s *scan.Scan) map[string]any {
	return map[string]any{
		"root":    s.Root,
		"domains": s.Domains,
		"methods": flattenMethods(s),
		"anchors": anchorSummaries(s),
		"issues":  s.Issues,
	}
}

func formatAIContextText(s *scan.Scan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ncgo_ai_context for %s\n\n", s.Root)
	for _, d := range s.Domains {
		state := "missing"
		if d.UsecaseExists {
			state = "ok"
		}
		fmt.Fprintf(&b, "- %s: usecase %s, %d methods, anchors %v\n",
			d.Name, state, len(d.Methods), d.AnchorsOK)
	}
	if len(s.Issues) > 0 {
		fmt.Fprintf(&b, "\nissues (%d):\n", len(s.Issues))
		for _, i := range s.Issues {
			fmt.Fprintf(&b, "  - [%s] %s\n", i.Kind, i.Message)
		}
	}
	return b.String()
}

func flattenMethods(s *scan.Scan) []map[string]any {
	var out []map[string]any
	for _, d := range s.Domains {
		for _, m := range d.Methods {
			out = append(out, map[string]any{
				"domain": d.Name,
				"name":   m.Name,
				"file":   m.File,
				"line":   m.Line,
			})
		}
	}
	return out
}

func anchorSummaries(s *scan.Scan) []map[string]any {
	var out []map[string]any
	for _, d := range s.Domains {
		out = append(out, map[string]any{"domain": d.Name, "ok": d.AnchorsOK})
	}
	return out
}

var _ io.Writer // keep io import if unused after edits
```

> **Note:** if the `io` import is unused after writing, remove it. The `var _ io.Writer` line is only a placeholder guard; delete it when the file compiles without it.

- [ ] **Step 4: Register in `internal/mcp/tools.go`**

In `tools()`, add after the `ncgo_ai_sync` entry:

```go
{Name: "ncgo_ai_context", Description: "Scan real code and return structured context (domains/methods/anchors/consistency) for an ncgo service.", InputSchema: schemaObject([]string{"root"}, rootField("Service root containing .ncgo/manifest.yaml"), outputTextJSONField())},
```

In `callTool`, add after `case "ncgo_ai_sync":`:

```go
case "ncgo_ai_context":
	return callAIContext(p.Arguments)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/mcp/ -run 'TestServeTool.*AIContext' -count=1` then `go test ./internal/mcp/ -count=1`
Expected: PASS (new + existing).

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tool_ai_context.go internal/mcp/tools.go internal/mcp/server_test.go
git commit -m "feat(mcp): add ncgo_ai_context tool with structured scan output"
```

---

### Task 4: CLI `ncgo check` command (exit codes 0/1/2)

**Files:**
- Create: `internal/cli/check.go`
- Create: `internal/cli/check_test.go`
- Modify: `internal/cli/root.go` (register command + `exitCodeError` handling in `Main`)
- Modify: `internal/doctor/doctor.go` (export `Summarize`)

**Interfaces:**
- Consumes: `scan.Scan`, `scan.GeneratedAtMarker` (Task 1), `ai.ReadGeneratedAt` (Task 2), `doctor.Report`/`Check`/`WriteJSON`/`WriteText`, `manifest.Load`.
- Produces: `ncgo check` cobra command; `exitCodeError` type honored by `Main()`.
- doctor package gains:
  ```go
  // Summarize aggregates a slice of Checks into a ReportSummary.
  func Summarize(checks []Check) ReportSummary
  ```

- [ ] **Step 1: Modify `internal/doctor/doctor.go` — export Summarize**

Rename the private `summarizeChecks` (doctor.go:132) to exported `Summarize` and update the call site at doctor.go:108:

```go
	r.Summary = Summarize(r.Checks)
```

```go
// Summarize aggregates a slice of Checks into a ReportSummary.
func Summarize(checks []Check) ReportSummary {
	var s ReportSummary
	s.CheckCount = len(checks)
	for _, c := range checks {
		if c.OK {
			s.PassedCount++
			continue
		}
		s.FailedCount++
		if c.Severity == SeverityWarn {
			s.WarningCount++
		} else {
			s.ErrorCount++
		}
	}
	return s
}
```

- [ ] **Step 2: Write the failing tests** — create `internal/cli/check_test.go`

```go
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

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

func TestRunCheckExitZeroOnHealthyProject(t *testing.T) {
	root := seedCheckProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runCheck(cmd, &checkOptions{root: root}); err != nil {
		t.Fatalf("runCheck: %v", err)
	}
	if !strings.Contains(out.String(), "all checks passed") {
		t.Fatalf("output missing success line:\n%s", out.String())
	}
}

func TestRunCheckExitOneOnBrokenAnchors(t *testing.T) {
	root := seedCheckProject(t)
	p := filepath.Join(root, "internal", "usecase", "device", "device.go")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "// ncgo:methods:start\n", "")), 0o644)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 1 {
		t.Fatalf("err = %v, want exitCodeError code 1", err)
	}
}

func TestRunCheckExitOneOnStaleContext(t *testing.T) {
	root := seedCheckProject(t)
	// Write a CLAUDE.md with a marker older than manifest.GeneratedAt.
	claude := filepath.Join(root, "CLAUDE.md")
	content := "<!-- ncgo:managed -->\n<!-- ncgo:generated-at: 2020-01-01T00:00:00Z -->\n# body\n"
	if err := os.WriteFile(claude, []byte(content), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 1 {
		t.Fatalf("err = %v, want exitCodeError code 1", err)
	}
}

func TestRunCheckExitTwoOnMissingManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runCheck(cmd, &checkOptions{root: root})
	var ec *exitCodeError
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("err = %v, want exitCodeError code 2", err)
	}
}

func TestRunCheckJSONOutput(t *testing.T) {
	root := seedCheckProject(t)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runCheck(cmd, &checkOptions{root: root, output: "json"}); err != nil {
		t.Fatalf("runCheck: %v", err)
	}
	var got struct {
		Summary struct {
			CheckCount int `json:"checkCount"`
		} `json:"summary"`
		Checks []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.Summary.CheckCount == 0 {
		t.Fatalf("summary empty: %+v", got)
	}
	found := map[string]bool{}
	for _, c := range got.Checks {
		found[c.ID] = true
	}
	for _, id := range []string{"check.anchor", "check.manifest.consistency", "check.context.stale"} {
		if !found[id] {
			t.Errorf("json missing check %s: %+v", id, got.Checks)
		}
	}
}

// time import kept for ReadGeneratedAt-style comparisons if needed.
var _ = time.RFC3339
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestRunCheck' -count=1`
Expected: FAIL (`runCheck`, `checkOptions`, `exitCodeError` undefined).

- [ ] **Step 4: Create `internal/cli/check.go`**

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/doctor"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scan"
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
			"AI context files are not older than the manifest. Exits 0 on pass, 1 on " +
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
	rep, err := buildCheckReport(opts.root)
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

// buildCheckReport assembles a doctor.Report from a scan of the service root.
// It returns an error only when the root is not an ncgo service.
func buildCheckReport(root string) (*doctor.Report, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	s, err := scan.Scan(root)
	if err != nil {
		return nil, err
	}
	rep := &doctor.Report{Root: root, Scope: doctor.ScopeService}
	rep.Checks = append(rep.Checks, anchorChecks(s)...)
	rep.Checks = append(rep.Checks, consistencyChecks(s)...)
	rep.Checks = append(rep.Checks, contextStaleChecks(root, m)...)
	rep.Summary = doctor.Summarize(rep.Checks)
	return rep, nil
}

func anchorChecks(s *scan.Scan) []doctor.Check {
	var out []doctor.Check
	bad := 0
	for _, d := range s.Domains {
		if d.UsecaseExists && !d.AnchorsOK {
			bad++
			out = append(out, doctor.Check{
				ID: "check.anchor", OK: false, Severity: doctor.SeverityError,
				Message: fmt.Sprintf("domain %s has unpaired method anchors", d.Name),
				Hint:    "run `ncgo add method <domain>.X` or fix the // ncgo:methods:start|end markers",
			})
		}
	}
	if bad == 0 {
		out = append(out, doctor.Check{
			ID: "check.anchor", OK: true, Severity: doctor.SeverityError,
			Message: "all usecase files have paired method anchors",
		})
	}
	return out
}

func consistencyChecks(s *scan.Scan) []doctor.Check {
	var out []doctor.Check
	bad := 0
	for _, i := range s.Issues {
		if i.Kind != scan.IssueMissingUsecase && i.Kind != scan.IssueUndeclaredDomain {
			continue
		}
		bad++
		out = append(out, doctor.Check{
			ID: "check.manifest.consistency", OK: false, Severity: doctor.SeverityError,
			Message: i.Message, File: i.File,
		})
	}
	if bad == 0 {
		out = append(out, doctor.Check{
			ID: "check.manifest.consistency", OK: true, Severity: doctor.SeverityError,
			Message: "manifest domains match internal/usecase/*/ directories",
		})
	}
	return out
}

// contextStaleChecks compares each rendered context file's generated-at marker
// against the manifest timestamp. Missing files are skipped (not a failure).
func contextStaleChecks(root string, m *manifest.Manifest) []doctor.Check {
	var out []doctor.Check
	stale := 0
	for _, rel := range contextFileTargets() {
		path := filepath.Join(root, rel)
		if !pathExists(path) {
			continue
		}
		ts, ok := ai.ReadGeneratedAt(path)
		if ok && !ts.Before(m.GeneratedAt) {
			continue
		}
		stale++
		out = append(out, doctor.Check{
			ID: "check.context.stale", OK: false, Severity: doctor.SeverityError,
			Message: fmt.Sprintf("%s is stale (context older than manifest)", rel),
			File:    path,
			Hint:    "run `ncgo ai sync --root .`",
		})
	}
	if stale == 0 {
		out = append(out, doctor.Check{
			ID: "check.context.stale", OK: true, Severity: doctor.SeverityError,
			Message: "AI context files are up to date",
		})
	}
	return out
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

var _ = errors.Is // kept for exitCodeError tests; remove if unused
var _ = strings.Contains
```

> **Note:** remove the trailing `var _ =` placeholder lines if the imports they guard (`errors`, `strings`) become unused after writing. `errors` and `strings` may not be needed in check.go itself — only in check_test.go.

- [ ] **Step 5: Register in `internal/cli/root.go`**

Add `newCheckCmd()` to the command list (after `newDoctorCmd()`):

```go
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newCheckCmd())
```

Update `Main()` to honor `exitCodeError`:

```go
func Main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		var ec *exitCodeError
		if errors.As(err, &ec) {
			if ec.msg != "" {
				fmt.Fprintln(os.Stderr, ec.msg)
			}
			os.Exit(ec.code)
		}
		if _, silent := err.(silentErr); !silent {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
```

`errors` is already imported in `root.go`.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestRunCheck' -count=1` then `go test ./internal/cli/ ./internal/doctor/ -count=1`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/check.go internal/cli/check_test.go internal/cli/root.go internal/doctor/doctor.go
git commit -m "feat(cli): add ncgo check with 0/1/2 exit codes"
```

---

### Task 5: Smoke test + documentation (EN/ZH)

**Files:**
- Modify: `scripts/smoke.sh`
- Modify: `README.md`, `README.zh-CN.md`
- Modify: `docs/examples.md`, `docs/examples.zh-CN.md`

**Interfaces:**
- Consumes: `ncgo check` CLI (Task 4), `ncgo_ai_context` MCP tool (Task 3).

- [ ] **Step 1: Extend `scripts/smoke.sh`** — add after the `upgrade --plan is read-only` block:

```bash
log "ncgo check --help exposes --output"
"$BIN" check --help >"$TMP_DIR/check-help.out"
grep -q -- '--output' "$TMP_DIR/check-help.out"

log "ncgo check passes on a healthy service"
CHECK_ROOT="$TMP_DIR/check-ok"
write_manifest "$CHECK_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
mkdir -p "$CHECK_ROOT/internal/usecase/demo"
cat >"$CHECK_ROOT/internal/usecase/demo/demo.go" <<'GO'
package demo

type UseCase struct{}

// ncgo:methods:start
// ncgo:methods:end
GO
"$BIN" check --root "$CHECK_ROOT" >"$TMP_DIR/check-ok.out"
grep -q 'all checks passed' "$TMP_DIR/check-ok.out"

log "ncgo check exits 2 on a non-project root"
"$BIN" check --root "$TMP_DIR" >"$TMP_DIR/check-err.out" 2>&1 && { echo "check should have failed"; exit 1; } || true
```

Add `ncgo_ai_context` to the MCP `required` set in the embedded python:

```python
required = {"ncgo_version", "ncgo_doctor", "ncgo_ai_sync", "ncgo_add_infra", "ncgo_add_method", "ncgo_ai_context"}
```

- [ ] **Step 2: Run smoke**

Run: `./scripts/smoke.sh`
Expected: PASS (all existing + new `check` steps).

- [ ] **Step 3: Update `README.md` + `README.zh-CN.md`**

Add a `ncgo check` line to the command list (near `ncgo doctor`):

```markdown
- `ncgo check` — validate AI context integrity: method anchors, manifest↔usecase consistency, and stale context files. Exits 0 pass / 1 check failed / 2 command error. (`--output text|json`)
- `ncgo_ai_context` — MCP tool that scans real code and returns structured domains/methods/anchors/consistency for agents.
```

Add a note under the `ai sync` docs that rendered files carry a `<!-- ncgo:generated-at: ... -->` marker used by `ncgo check`.

- [ ] **Step 4: Update `docs/examples.md` + `docs/examples.zh-CN.md`**

Extend the agent workflow example with the dynamic-layer loop:

```markdown
## Validating an agent's changes

After implementing a feature with `ncgo add domain` / `ncgo add method`, run:

    ncgo check --root .

If any usecase lost its `// ncgo:methods` anchors, the manifest drifted from
`internal/usecase/*/`, or AI context files are stale, the command exits 1 with
a structured report (`--output json`). Refresh context with:

    ncgo ai sync --root .
```

- [ ] **Step 5: Run markdown diagnostics**

Run: `npx markdownlint-cli2 "README.md" "README.zh-CN.md" "docs/examples.md" "docs/examples.zh-CN.md" 2>/dev/null || true` (report whatever it flags; fix trivially).
Alternatively, if no markdownlint is configured, run `git diff --check` on the edited docs.

- [ ] **Step 6: Commit**

```bash
git add scripts/smoke.sh README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs(ai): document ncgo check and ncgo_ai_context (en/zh); extend smoke"
```

---

### Task 6: Final validation

- [ ] **Step 1: Run the full CI-equivalent gate**

```bash
go build ./...
go build .
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```

- [ ] **Step 2: Update golden tests if any scaffold output changed**

The `ai sync` marker lives in `internal/ai`; `mono.Generate` does not invoke `ai sync`, so mono golden snapshots should be unaffected. Verify with:

```bash
go test ./internal/scaffold/mono/... -count=1
```

If anything drifted, run `go test ./internal/scaffold/mono/... -update-golden -count=1` and inspect the diff.

- [ ] **Step 3: Fix any findings and rerun the gate until green.**

---

## Self-Review

**Spec coverage:**
- §3.1 `internal/scan` (domains/methods/anchors/consistency) → Task 1 ✓
- §3.2 MCP `ncgo_ai_context` (root + output schema, double output) → Task 3 ✓
- §3.3 `ncgo check` (3 checks, 0/1/2 exit, doctor.Report reuse) → Task 4 ✓
- §3.4 `ai sync` generated-at marker (contract change) → Task 2 ✓
- §5 test matrix (scan unit, check integration, MCP integration, ai marker) → Tasks 1-4 ✓
- §6 docs (README + examples EN/ZH) → Task 5 ✓
- D9 service-level scope (workspace root → command error) → Task 1 `Scan` returns error on missing manifest; Task 4 exit 2 ✓
- D4 reuse doctor.Report → Task 4 `buildCheckReport` + `doctor.Summarize` ✓

**Placeholder scan:** All steps carry concrete code or commands; no TBD/TODO.

**Type consistency:** `scan.Scan`, `scan.Domain`, `scan.Method`, `scan.Issue`, `scan.GeneratedAtMarker`, `ai.ReadGeneratedAt`, `doctor.Summarize`, `exitCodeError`, `checkOptions`, `runCheck`, `buildCheckReport` are defined once and reused consistently. `stampGeneratedAt` (Task 2) and `ReadGeneratedAt` (Task 2) share the `scan.GeneratedAtMarker` constant.

**Open verification point:** `check.context.stale` skips context files that do not exist (treats absence as "not stale"). This matches the design intent that `ncgo check` validates an agent's changes rather than demanding `ai sync` has run. Flag for reviewer confirmation.
