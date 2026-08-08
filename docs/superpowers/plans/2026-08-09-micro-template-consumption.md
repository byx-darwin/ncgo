# Micro Template Consumption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable `ncgo add rpc/bff` and `ncgo new --mode micro` to consume template packages (mono kind or micro kind), extending `LoadPackage` with kind-aware subdirectory resolution.

**Architecture:** Extend `scaffoldtemplate.LoadPackage` to resolve subdirectories based on `(pkg.kind, expectedKind)`. Mono packages (`kitex`/`hertz`) are unchanged. Micro packages expose `workspace/` (for `ncgo new --mode micro`), `kitex-template/` + `idl/kitex/` (for `add rpc`), and `hertz-template/` + `idl/hertz/` (for `add bff`). The CLI and MCP layers add `--template`/`--template-dir`/`--preset` flags, resolve them via `registry.ResolveTemplateDir`, and pass `TemplateDir` through to the scaffold generators.

**Tech Stack:** Go 1.25+, cobra (CLI), gopkg.in/yaml.v3 (template.yaml parsing), text/template (workspace .tpl rendering), existing registry client.

## Global Constraints

- Preserve backward compatibility: mono packages (`kind: kitex/hertz`) must work identically without changes.
- Mutual exclusion: `--preset` ↔ `--template` ↔ `--template-dir` — at most one.
- `kind: micro` template packages are new; existing mono packages are unaffected.
- Micro workspace overlay: built-in skeleton first, then overlay from template package. `.tpl` suffix = variable substitution; no suffix = copy verbatim.
- Golden tests use `testdata/` fixtures; smoke tests use `--no-generate` to avoid requiring hz/kitex on PATH.
- Docs EN/ZH must stay aligned.

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `internal/scaffold/template/workspace.go` | `OverlayWorkspaceTemplates` — walk `<pkg.Dir>/workspace/`, render `.tpl` files, copy others |
| `internal/scaffold/template/workspace_test.go` | Tests for workspace overlay |
| `internal/registry/resolve.go` | `ResolveTemplateDir(templateName, templateDir)` — shared CLI helper |
| `internal/registry/resolve_test.go` | Tests for ResolveTemplateDir |

### Modified files

| File | Changes |
|---|---|
| `internal/scaffold/template/package.go` | Add kind-aware subdirectory resolution in `LoadPackage` |
| `internal/scaffold/template/package_test.go` | Tests for micro package loading |
| `internal/scaffold/rpc/rpc.go` | Add `TemplateDir` to `Options`; pass to `mono.Generate` |
| `internal/scaffold/rpc/rpc_test.go` | Tests for template pass-through |
| `internal/scaffold/bff/bff.go` | Add `TemplateDir` + `Preset` to `Options`; pass to `mono.Generate` |
| `internal/scaffold/bff/bff_test.go` | Tests for template + preset pass-through |
| `internal/scaffold/micro/micro.go` | Add `TemplateDir` to `Options`; call `OverlayWorkspaceTemplates` |
| `internal/scaffold/micro/micro_test.go` | Tests for workspace overlay |
| `internal/cli/add.go` | Add `--template`/`--template-dir` to add rpc; add `--preset`/`--template`/`--template-dir` to add bff |
| `internal/cli/root.go` | Wire `--template`/`--template-dir` through to `micro.Generate` in `runNewMicro` |
| `internal/cli/root_test.go` | Tests for micro template flags |
| `internal/mcp/tools.go` | Extend `ncgo_add_rpc`/`ncgo_add_bff` InputSchema with `template`/`templateDir`/`preset` |
| `internal/mcp/tool_rpc.go` | Add `template`/`templateDir` fields to `callAddRPC` args |
| `internal/mcp/tool_bff.go` | Add `template`/`templateDir`/`preset` fields to `callAddBFF` args |
| `internal/mcp/tool_new.go` | Pass `TemplateDir` to `micro.Generate` in `runNewMicro` |
| `README.md` / `README.zh-CN.md` | Command tables: add new flags |
| `docs/examples.md` / `docs/examples.zh-CN.md` | New section: micro workspace template consumption |

---

### Task 1: Extend LoadPackage with kind-aware subdirectory resolution

**Files:**
- Modify: `internal/scaffold/template/package.go:55-120`
- Test: `internal/scaffold/template/package_test.go`

**Interfaces:**
- Consumes: existing `Package`, `PackageMeta` types
- Produces: extended `LoadPackage(dir, kind)` that handles `(micro, kitex)`, `(micro, hertz)`, `(micro, micro)` combinations

- [ ] **Step 1: Write failing test for micro package loading as workspace**

Add to `internal/scaffold/template/package_test.go`:

```go
func TestLoadPackageMicroAsWorkspace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"),
		[]byte("name: my-micro\nkind: micro\ndescription: d\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "workspace"), 0o755)
	os.WriteFile(filepath.Join(dir, "workspace", "compose.yaml.tpl"), []byte("name: {{.Name}}\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "main.yaml"), []byte("path: main.go\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "hertz-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "hertz-template", "main.yaml"), []byte("path: main.go\n"), 0o644)

	pkg, err := LoadPackage(dir, "micro")
	if err != nil {
		t.Fatalf("LoadPackage micro: %v", err)
	}
	if pkg.Meta.Kind != "micro" {
		t.Errorf("kind = %q, want micro", pkg.Meta.Kind)
	}
	if pkg.TemplateDir != filepath.Join(dir, "workspace") {
		t.Errorf("TemplateDir = %q, want workspace/", pkg.TemplateDir)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestLoadPackageMicroAsWorkspace -count=1 -v`
Expected: FAIL — `LoadPackage` currently rejects `kind: micro` because it expects `<kind>-template/` to be `micro-template/`.

- [ ] **Step 3: Write failing test for micro package loading as kitex service source**

Add to `internal/scaffold/template/package_test.go`:

```go
func TestLoadPackageMicroAsKitexSource(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"),
		[]byte("name: my-micro\nkind: micro\ndescription: d\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "workspace"), 0o755)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "main.yaml"), []byte("path: main.go\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "idl", "kitex"), 0o755)
	os.WriteFile(filepath.Join(dir, "idl", "kitex", "svc.proto"), []byte("syntax = \"proto3\";\n"), 0o644)

	pkg, err := LoadPackage(dir, "kitex")
	if err != nil {
		t.Fatalf("LoadPackage micro-as-kitex: %v", err)
	}
	if pkg.TemplateDir != filepath.Join(dir, "kitex-template") {
		t.Errorf("TemplateDir = %q, want kitex-template/", pkg.TemplateDir)
	}
	if pkg.IDLDir != filepath.Join(dir, "idl", "kitex") {
		t.Errorf("IDLDir = %q, want idl/kitex/", pkg.IDLDir)
	}
	if len(pkg.Templates) != 1 || len(pkg.IDLs) != 1 {
		t.Errorf("templates=%d idls=%d", len(pkg.Templates), len(pkg.IDLs))
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestLoadPackageMicroAsKitexSource -count=1 -v`
Expected: FAIL — kind mismatch (`micro` vs expected `kitex`).

- [ ] **Step 5: Implement kind-aware resolution in LoadPackage**

Modify `internal/scaffold/template/package.go`. Replace the kind check and template directory resolution with a helper:

```go
func LoadPackage(dir, kind string) (*Package, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve template dir: %w", err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("template package %q does not exist", dir)
	}
	pkg := &Package{Dir: abs}
	meta, err := ReadPackageMeta(abs)
	switch {
	case err == nil:
		pkg.Meta, pkg.HasMeta = meta, true
	case errors.Is(err, fs.ErrNotExist):
		// optional metadata
	default:
		return nil, err
	}
	tplSubDir, idlSubDir, err := resolveTemplateSubDirs(pkg.Meta.Kind, kind)
	if err != nil {
		return nil, fmt.Errorf("template package %q: %w", dir, err)
	}
	pkg.TemplateDir = filepath.Join(abs, tplSubDir)
	entries, err := os.ReadDir(pkg.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("template package %q has no %s/ directory", dir, tplSubDir)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		pkg.Templates = append(pkg.Templates, filepath.Join(pkg.TemplateDir, e.Name()))
	}
	if len(pkg.Templates) == 0 {
		return nil, fmt.Errorf("template package %q has no .yaml templates in %s/", dir, tplSubDir)
	}
	idlAbs := filepath.Join(abs, idlSubDir)
	pkg.IDLDir = idlAbs
	if fi, err := os.Stat(idlAbs); err == nil && fi.IsDir() {
		_ = filepath.Walk(idlAbs, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".proto") {
				pkg.IDLs = append(pkg.IDLs, path)
			}
			return nil
		})
	}
	// IDL fallback: if expected kind-specific idl dir is empty, try flat idl/
	if idlSubDir != "idl" && len(pkg.IDLs) == 0 {
		flatIDL := filepath.Join(abs, "idl")
		if fi, err := os.Stat(flatIDL); err == nil && fi.IsDir() {
			_ = filepath.Walk(flatIDL, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(path, ".proto") {
					pkg.IDLs = append(pkg.IDLs, path)
				}
				return nil
			})
			if len(pkg.IDLs) > 0 {
				pkg.IDLDir = flatIDL
			}
		}
	}
	pkg.SchemaDir = filepath.Join(abs, "schema")
	if fi, err := os.Stat(pkg.SchemaDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(pkg.SchemaDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".sql") {
				pkg.Schemas = append(pkg.Schemas, path)
			}
			return nil
		})
	}
	layoutPath := filepath.Join(abs, "layout.yaml")
	if _, err := os.Stat(layoutPath); err == nil {
		pkg.LayoutFile = layoutPath
	}
	return pkg, nil
}

// resolveTemplateSubDirs returns the template and IDL subdirectory names
// based on the package's declared kind and the consumer's expected kind.
func resolveTemplateSubDirs(pkgKind, expectedKind string) (tplDir, idlDir string, err error) {
	switch {
	case pkgKind == "" || pkgKind == expectedKind:
		// No metadata (pkgKind="") or matching kind: standard layout.
		// For micro expected kind, use workspace/; otherwise <kind>-template/.
		if expectedKind == "micro" {
			return "workspace", "", nil
		}
		return expectedKind + "-template", "idl", nil
	case pkgKind == "micro" && expectedKind == "kitex":
		return "kitex-template", "idl/kitex", nil
	case pkgKind == "micro" && expectedKind == "hertz":
		return "hertz-template", "idl/hertz", nil
	default:
		return "", "", fmt.Errorf("template package kind %q does not match expected kind %q", pkgKind, expectedKind)
	}
}
```

Also add a test for backward compatibility (existing mono behavior):

```go
func TestLoadPackageMonoKitexUnchanged(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: d\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "main.yaml"), []byte("path: main.go\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "idl"), 0o755)
	os.WriteFile(filepath.Join(dir, "idl", "svc.proto"), []byte("syntax = \"proto3\";\n"), 0o644)

	pkg, err := LoadPackage(dir, "kitex")
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	if pkg.TemplateDir != filepath.Join(dir, "kitex-template") {
		t.Errorf("backward compat broken: TemplateDir = %q", pkg.TemplateDir)
	}
}
```

- [ ] **Step 6: Run all LoadPackage tests to verify they pass**

Run: `go test ./internal/scaffold/template/... -count=1 -v`
Expected: ALL PASS — including existing tests (backward compat) and new micro tests.

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/template/package.go internal/scaffold/template/package_test.go
git commit -m "feat(template): extend LoadPackage with kind-aware subdirectory resolution for micro packages"
```

---

### Task 2: Micro workspace template overlay

**Files:**
- Create: `internal/scaffold/template/workspace.go`
- Create: `internal/scaffold/template/workspace_test.go`
- Modify: `internal/scaffold/micro/micro.go`
- Modify: `internal/scaffold/micro/micro_test.go`

**Interfaces:**
- Consumes: `Package` from Task 1 (with `TemplateDir = workspace/`, `Templates` list)
- Produces: `OverlayWorkspaceTemplates(dir string, pkg *Package, data RenderData) error`
- Consumes: `micro.Options` gains `TemplateDir string`

- [ ] **Step 1: Write failing test for workspace overlay**

Create `internal/scaffold/template/workspace_test.go`:

```go
package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlayWorkspaceTemplates(t *testing.T) {
	// Set up a micro package with workspace/ templates
	pkgDir := t.TempDir()
	os.MkdirAll(filepath.Join(pkgDir, "workspace", "scripts"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "compose.yaml.tpl"),
		[]byte("name: {{.Name}}\nmodule: {{.Module}}\n"), 0o644)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "scripts", "build.sh.tpl"),
		[]byte("#!/bin/sh\n# {{.Name}} build\n"), 0o644)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "extra.txt"),
		[]byte("verbatim copy\n"), 0o644)

	pkg := &Package{
		Dir:         pkgDir,
		TemplateDir: filepath.Join(pkgDir, "workspace"),
		Templates:   []string{filepath.Join(pkgDir, "workspace", "compose.yaml.tpl"), filepath.Join(pkgDir, "workspace", "scripts", "build.sh.tpl")},
	}

	targetDir := t.TempDir()
	// Pre-write a built-in file that should be overwritten
	os.WriteFile(filepath.Join(targetDir, "compose.yaml"), []byte("original\n"), 0o644)

	data := RenderData{Name: "shop", Module: "github.com/acme/shop"}
	if err := OverlayWorkspaceTemplates(targetDir, pkg, data); err != nil {
		t.Fatalf("OverlayWorkspaceTemplates: %v", err)
	}

	// .tpl file rendered and suffix stripped
	compose, err := os.ReadFile(filepath.Join(targetDir, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	if !strings.Contains(string(compose), "name: shop") || !strings.Contains(string(compose), "module: github.com/acme/shop") {
		t.Errorf("compose not rendered: %s", compose)
	}

	// nested .tpl rendered
	build, err := os.ReadFile(filepath.Join(targetDir, "scripts", "build.sh"))
	if err != nil {
		t.Fatalf("read build.sh: %v", err)
	}
	if !strings.Contains(string(build), "shop build") {
		t.Errorf("build.sh not rendered: %s", build)
	}

	// non-.tpl file copied verbatim
	extra, err := os.ReadFile(filepath.Join(targetDir, "extra.txt"))
	if err != nil {
		t.Fatalf("read extra: %v", err)
	}
	if string(extra) != "verbatim copy\n" {
		t.Errorf("extra.txt = %q, want verbatim", extra)
	}
}

func TestOverlayWorkspaceTemplatesMissingDir(t *testing.T) {
	pkgDir := t.TempDir()
	pkg := &Package{Dir: pkgDir, TemplateDir: filepath.Join(pkgDir, "workspace")}
	err := OverlayWorkspaceTemplates(t.TempDir(), pkg, RenderData{})
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("want missing workspace error, got %v", err)
	}
}

func TestOverlayWorkspaceTemplatesEmpty(t *testing.T) {
	pkgDir := t.TempDir()
	os.MkdirAll(filepath.Join(pkgDir, "workspace"), 0o755)
	pkg := &Package{Dir: pkgDir, TemplateDir: filepath.Join(pkgDir, "workspace")}
	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "existing"), []byte("keep\n"), 0o644)
	if err := OverlayWorkspaceTemplates(targetDir, pkg, RenderData{}); err != nil {
		t.Fatalf("empty overlay should not error: %v", err)
	}
	// existing file preserved
	if _, err := os.Stat(filepath.Join(targetDir, "existing")); err != nil {
		t.Errorf("existing file lost: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestOverlay -count=1 -v`
Expected: FAIL — `OverlayWorkspaceTemplates` and `RenderData` not defined (or wrong signature).

Note: `RenderData` may already exist in `internal/scaffold/template/render.go`. Check and reuse the existing type if so. If the existing `RenderData` has different fields, extend it with `Name` (if not already present).

- [ ] **Step 3: Implement OverlayWorkspaceTemplates**

Create `internal/scaffold/template/workspace.go`:

```go
package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// OverlayWorkspaceTemplates walks <pkg.TemplateDir>/ recursively and overlays
// files onto <targetDir>. Files with a .tpl suffix are rendered with Go
// text/template using data; the .tpl suffix is stripped from the target path.
// Files without .tpl are copied verbatim. Either way, existing files at the
// target path are overwritten.
func OverlayWorkspaceTemplates(targetDir string, pkg *Package, data RenderData) error {
	root := pkg.TemplateDir
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return fmt.Errorf("workspace overlay: %s is not a directory", root)
	}
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(targetDir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(rel, ".tpl") {
			target = strings.TrimSuffix(target, ".tpl")
			body, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("workspace overlay: read %s: %w", path, err)
			}
			tmpl, err := template.New(rel).Parse(string(body))
			if err != nil {
				return fmt.Errorf("workspace overlay: parse %s: %w", path, err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("workspace overlay: render %s: %w", path, err)
			}
			return os.WriteFile(target, buf.Bytes(), 0o644)
		}
		// Non-.tpl: copy verbatim
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("workspace overlay: read %s: %w", path, err)
		}
		return os.WriteFile(target, body, 0o644)
	})
}
```

Note: `RenderData` is assumed to exist in `render.go`. If `Name` is not a field on the existing `RenderData`, add it. If the existing type uses different field names (e.g., `ServiceName`), use that and document the alias in the design.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/scaffold/template/... -run TestOverlay -count=1 -v`
Expected: ALL PASS.

- [ ] **Step 5: Add TemplateDir to micro.Options and call overlay in Generate**

Modify `internal/scaffold/micro/micro.go`:

Add `TemplateDir string` to `Options` struct:

```go
type Options struct {
	Name          string
	Module        string
	Dir           string
	AssetsVersion string
	NCGOVersion   string
	Now           time.Time
	TemplateDir   string // external template package dir; overlays workspace/ templates onto built-in skeleton
}
```

In `Generate`, after writing the built-in skeleton, add overlay:

```go
func Generate(opts Options) (*Result, error) {
	// ... existing validation, ensureEmptyDir ...
	// ... existing writeWorkspace, writeReadme, WriteWorkspaceCompose, WriteRepositoryHooks, services/.gitkeep ...

	// Overlay workspace templates if TemplateDir is set
	if opts.TemplateDir != "" {
		pkg, err := scaffoldtemplate.LoadPackage(opts.TemplateDir, "micro")
		if err != nil {
			return nil, err
		}
		data := scaffoldtemplate.RenderData{
			Module:      opts.Module,
			ServiceName: opts.Name,
		}
		if err := scaffoldtemplate.OverlayWorkspaceTemplates(dir, pkg, data); err != nil {
			return nil, fmt.Errorf("micro: workspace overlay: %w", err)
		}
	}

	return &Result{Dir: dir, NextSteps: nextSteps(opts)}, nil
}
```

- [ ] **Step 6: Add test for micro.Generate with TemplateDir**

Add to `internal/scaffold/micro/micro_test.go`:

```go
func TestGenerateWithTemplateDirOverlaysWorkspace(t *testing.T) {
	// Create a micro template package fixture
	pkgDir := t.TempDir()
	os.WriteFile(filepath.Join(pkgDir, "template.yaml"),
		[]byte("name: test-micro\nkind: micro\ndescription: test\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "workspace"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "custom.txt.tpl"),
		[]byte("module={{.Module}} name={{.Name}}\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "kitex-template", "a.yaml"), []byte("path: a.go\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "hertz-template"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "hertz-template", "a.yaml"), []byte("path: a.go\n"), 0o644)

	opts := baseOpts(t)
	opts.TemplateDir = pkgDir
	res, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate with template: %v", err)
	}
	// Built-in files still present
	for _, p := range []string{"ncgo.workspace", "README.md", "compose.yaml"} {
		if _, err := os.Stat(filepath.Join(res.Dir, p)); err != nil {
			t.Errorf("built-in %s missing: %v", p, err)
		}
	}
	// Overlay file rendered
	custom, err := os.ReadFile(filepath.Join(res.Dir, "custom.txt"))
	if err != nil {
		t.Fatalf("read custom.txt: %v", err)
	}
	if !strings.Contains(string(custom), "module=github.com/acme/shop") {
		t.Errorf("custom.txt not rendered: %s", custom)
	}
}
```

- [ ] **Step 7: Run all micro tests**

Run: `go test ./internal/scaffold/micro/... -count=1 -v`
Expected: ALL PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/scaffold/template/workspace.go internal/scaffold/template/workspace_test.go internal/scaffold/micro/micro.go internal/scaffold/micro/micro_test.go
git commit -m "feat(micro): add workspace template overlay for ncgo new --mode micro --template-dir"
```

---

### Task 3: Registry ResolveTemplateDir helper

**Files:**
- Create: `internal/registry/resolve.go`
- Create: `internal/registry/resolve_test.go`

**Interfaces:**
- Consumes: `Client.LocalPath`, `os.Stat`
- Produces: `ResolveTemplateDir(templateName, templateDir string) (string, error)`

- [ ] **Step 1: Write failing test**

Create `internal/registry/resolve_test.go`:

```go
package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTemplateDirBothEmpty(t *testing.T) {
	dir, err := ResolveTemplateDir("", "")
	if err != nil || dir != "" {
		t.Errorf("both empty: dir=%q err=%v, want empty/nil", dir, err)
	}
}

func TestResolveTemplateDirLocalPath(t *testing.T) {
	tmp := t.TempDir()
	dir, err := ResolveTemplateDir("", tmp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	abs, _ := filepath.Abs(tmp)
	if dir != abs {
		t.Errorf("dir=%q, want %q", dir, abs)
	}
}

func TestResolveTemplateDirFromCache(t *testing.T) {
	cacheRoot := t.TempDir()
	name := "base-kitex"
	pkgDir := filepath.Join(cacheRoot, name)
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, "template.yaml"), []byte("kind: kitex\n"), 0o644)

	client := NewClient("", nil)
	client.Root = cacheRoot

	dir, err := ResolveTemplateDirWith(client, name, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dir != pkgDir {
		t.Errorf("dir=%q, want %q", dir, pkgDir)
	}
}

func TestResolveTemplateDirNotInCache(t *testing.T) {
	cacheRoot := t.TempDir()
	client := NewClient("", nil)
	client.Root = cacheRoot

	_, err := ResolveTemplateDirWith(client, "nonexistent", "")
	if err == nil || !strings.Contains(err.Error(), "not in cache") {
		t.Errorf("want cache miss error, got %v", err)
	}
}

func TestResolveTemplateDirMutualExclusion(t *testing.T) {
	_, err := ResolveTemplateDir("base-kitex", "/some/path")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("want mutual exclusion error, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/registry/... -count=1 -v`
Expected: FAIL — `ResolveTemplateDir` and `ResolveTemplateDirWith` not defined.

- [ ] **Step 3: Implement ResolveTemplateDir**

Create `internal/registry/resolve.go`:

```go
package registry

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveTemplateDir converts --template/--template-dir to an absolute directory path.
// Returns ("", nil) when neither flag is set (default embedded templates).
// Uses the default registry client for --template resolution.
func ResolveTemplateDir(templateName, templateDir string) (string, error) {
	if templateName != "" && templateDir != "" {
		return "", fmt.Errorf("--template and --template-dir are mutually exclusive")
	}
	if templateName != "" {
		client := NewClient(ResolveURL(""), nil)
		return ResolveTemplateDirWith(client, templateName, templateDir)
	}
	if templateDir != "" {
		abs, err := filepath.Abs(templateDir)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return "", nil
}

// ResolveTemplateDirWith resolves --template using the provided client.
// Exported for testing with a fake client.
func ResolveTemplateDirWith(client *Client, templateName, templateDir string) (string, error) {
	if templateName != "" && templateDir != "" {
		return "", fmt.Errorf("--template and --template-dir are mutually exclusive")
	}
	if templateName != "" {
		dir := client.LocalPath(templateName)
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return "", fmt.Errorf("template %q not in cache (%s); run: ncgo template pull %s", templateName, dir, templateName)
		}
		return dir, nil
	}
	if templateDir != "" {
		abs, err := filepath.Abs(templateDir)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return "", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/registry/... -count=1 -v`
Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/registry/resolve.go internal/registry/resolve_test.go
git commit -m "feat(registry): add ResolveTemplateDir helper for --template/--template-dir resolution"
```

---

### Task 4: Add TemplateDir to rpc.Options and bff.Options, pass through to mono.Generate

**Files:**
- Modify: `internal/scaffold/rpc/rpc.go:25-37` (Options struct) and `:91-102` (mono.Generate call)
- Modify: `internal/scaffold/rpc/rpc_test.go`
- Modify: `internal/scaffold/bff/bff.go:25-36` (Options struct) and `:88-98` (mono.Generate call)
- Modify: `internal/scaffold/bff/bff_test.go`

**Interfaces:**
- Consumes: `mono.Options.TemplateDir` (existing)
- Produces: `rpc.Options.TemplateDir`, `bff.Options.TemplateDir`, `bff.Options.Preset`

- [ ] **Step 1: Add TemplateDir to rpc.Options and pass through**

Add to `internal/scaffold/rpc/rpc.go` Options struct:

```go
TemplateDir   string      // external template package dir; replaces embedded kitex-template and IDL placeholder
```

Update the `mono.Generate` call to pass `TemplateDir`:

```go
monoRes, err := mono.Generate(ctx, mono.Options{
	Name:          opts.Name,
	Module:        module,
	Kind:          manifest.KindKitex,
	Dir:           serviceDir,
	AssetsVersion: opts.AssetsVersion,
	NCGOVersion:   opts.NCGOVersion,
	NoGenerate:    opts.NoGenerate,
	Preset:        opts.Preset,
	TemplateDir:   opts.TemplateDir,
	Runner:        opts.Runner,
	Now:           opts.Now,
})
```

- [ ] **Step 2: Add test for rpc.Add with TemplateDir**

Add to `internal/scaffold/rpc/rpc_test.go`:

```go
func TestAddWithTemplateDirUsesKitexTemplates(t *testing.T) {
	// Create a kitex template package fixture
	pkgDir := t.TempDir()
	os.WriteFile(filepath.Join(pkgDir, "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: test\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "kitex-template", "custom.yaml"), []byte("path: custom.go\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "idl"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "idl", "custom.proto"), []byte("syntax = \"proto3\";\n"), 0o644)

	root := seedWorkspace(t, nil)
	opts := baseOpts(root)
	opts.TemplateDir = pkgDir
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add with template: %v", err)
	}
	// Verify template was applied (custom.yaml → template/kitex-template/custom.yaml)
	if _, err := os.Stat(filepath.Join(res.ServiceDir, "template", "kitex-template", "custom.yaml")); err != nil {
		t.Errorf("template package custom.yaml not applied: %v", err)
	}
}
```

- [ ] **Step 3: Add TemplateDir + Preset to bff.Options and pass through**

Add to `internal/scaffold/bff/bff.go` Options struct:

```go
Preset        string      // preset template name (e.g., "rule-center")
TemplateDir   string      // external template package dir; replaces embedded hertz-template and IDL placeholder
```

Update the `mono.Generate` call to pass both:

```go
monoRes, err := mono.Generate(ctx, mono.Options{
	Name:          opts.Name,
	Module:        module,
	Kind:          manifest.KindHertz,
	Dir:           serviceDir,
	AssetsVersion: opts.AssetsVersion,
	NCGOVersion:   opts.NCGOVersion,
	NoGenerate:    opts.NoGenerate,
	Preset:        opts.Preset,
	TemplateDir:   opts.TemplateDir,
	Runner:        opts.Runner,
	Now:           opts.Now,
})
```

- [ ] **Step 4: Add test for bff.Add with TemplateDir**

Add to `internal/scaffold/bff/bff_test.go`:

```go
func TestAddWithTemplateDirUsesHertzTemplates(t *testing.T) {
	pkgDir := t.TempDir()
	os.WriteFile(filepath.Join(pkgDir, "template.yaml"),
		[]byte("name: base-hertz\nkind: hertz\ndescription: test\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "hertz-template"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "hertz-template", "custom.yaml"), []byte("path: custom.go\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "idl"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "idl", "custom.proto"), []byte("syntax = \"proto3\";\n"), 0o644)

	root := seedWorkspace(t, nil)
	opts := baseOpts(root)
	opts.TemplateDir = pkgDir
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add with template: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.ServiceDir, "template", "hertz-template", "custom.yaml")); err != nil {
		t.Errorf("template package custom.yaml not applied: %v", err)
	}
}

func TestAddWithPresetRuleCenter(t *testing.T) {
	root := seedWorkspace(t, nil)
	opts := baseOpts(root)
	opts.Preset = "rule-center"
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add with preset: %v", err)
	}
	// rule-center preset writes idl/rule-center.proto
	if _, err := os.Stat(filepath.Join(res.ServiceDir, "idl", "rule-center.proto")); err != nil {
		t.Errorf("rule-center preset IDL not written: %v", err)
	}
}
```

- [ ] **Step 5: Run rpc and bff tests**

Run: `go test ./internal/scaffold/rpc/... ./internal/scaffold/bff/... -count=1 -v`
Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/rpc/rpc.go internal/scaffold/rpc/rpc_test.go internal/scaffold/bff/bff.go internal/scaffold/bff/bff_test.go
git commit -m "feat(scaffold): add TemplateDir to rpc/bff Options, pass through to mono.Generate"
```

---

### Task 5: CLI flags for add rpc/bff and micro mode

**Files:**
- Modify: `internal/cli/add.go` (addRPCOptions, addBFFOptions structs; newAddRPCCmd, newAddBFFCmd flag registration; runAddRPC, runAddBFF resolution)
- Modify: `internal/cli/root.go` (runNewMicro: pass TemplateDir)
- Create or modify: `internal/cli/add_test.go` or `internal/cli/root_test.go`

**Interfaces:**
- Consumes: `registry.ResolveTemplateDir` from Task 3
- Consumes: `rpc.Options.TemplateDir`, `bff.Options.TemplateDir`, `bff.Options.Preset` from Task 4
- Consumes: `micro.Options.TemplateDir` from Task 2

- [ ] **Step 1: Add --template/--template-dir to add rpc**

Modify `addRPCOptions` struct in `internal/cli/add.go`:

```go
type addRPCOptions struct {
	root        string
	module      string
	dir         string
	noGenerate  bool
	dryRun      bool
	plan        bool
	output      string
	preset      string
	template    string // template package name from registry
	templateDir string // template package local directory path
}
```

Register flags in `newAddRPCCmd`:

```go
f.StringVar(&opts.template, "template", "", "Template package name from registry (kitex or micro kind)")
f.StringVar(&opts.templateDir, "template-dir", "", "Template package local directory path (kitex or micro kind)")
```

Resolve in `runAddRPC` before calling `rpc.Add`:

```go
templateDir, err := registry.ResolveTemplateDir(opts.template, opts.templateDir)
if err != nil {
	return err
}
if opts.preset != "" && templateDir != "" {
	return errors.New("--preset and --template/--template-dir are mutually exclusive")
}
```

Pass `TemplateDir` to `rpc.Options`:

```go
res, err := rpc.Add(cmd.Context(), rpc.Options{
	Root:          opts.root,
	Name:          name,
	Module:        opts.module,
	Dir:           opts.dir,
	AssetsVersion: assets.Version(),
	NCGOVersion:   Version,
	NoGenerate:    opts.noGenerate,
	DryRun:        opts.dryRun,
	Preset:        opts.preset,
	TemplateDir:   templateDir,
})
```

- [ ] **Step 2: Add --preset/--template/--template-dir to add bff**

Modify `addBFFOptions` struct:

```go
type addBFFOptions struct {
	root        string
	module      string
	dir         string
	noGenerate  bool
	dryRun      bool
	plan        bool
	output      string
	preset      string
	template    string
	templateDir string
}
```

Register flags in `newAddBFFCmd`:

```go
f.StringVar(&opts.preset, "preset", "", "Preset template to use (e.g., rule-center)")
f.StringVar(&opts.template, "template", "", "Template package name from registry (hertz or micro kind)")
f.StringVar(&opts.templateDir, "template-dir", "", "Template package local directory path (hertz or micro kind)")
```

Resolve in `runAddBFF`:

```go
templateDir, err := registry.ResolveTemplateDir(opts.template, opts.templateDir)
if err != nil {
	return err
}
if opts.preset != "" && templateDir != "" {
	return errors.New("--preset and --template/--template-dir are mutually exclusive")
}
```

Pass to `bff.Options`:

```go
res, err := bff.Add(cmd.Context(), bff.Options{
	Root:          opts.root,
	Name:          name,
	Module:        opts.module,
	Dir:           opts.dir,
	AssetsVersion: assets.Version(),
	NCGOVersion:   Version,
	NoGenerate:    opts.noGenerate,
	DryRun:        opts.dryRun,
	Preset:        opts.preset,
	TemplateDir:   templateDir,
})
```

- [ ] **Step 3: Wire --template/--template-dir through to micro.Generate in runNewMicro**

Modify `runNewMicro` in `internal/cli/root.go`:

Add validation for micro-incompatible flags and template resolution:

```go
func runNewMicro(cmd *cobra.Command, name string, opts *newOptions) error {
	if opts.kind != manifest.KindHertz {
		return errors.New("--kind is only supported with --mode mono")
	}
	if opts.idl != "" {
		return errors.New("--idl is only supported with --mode mono")
	}
	if opts.db != "none" {
		return errors.New("--db is only supported with --mode mono")
	}
	if len(opts.infra) > 0 {
		return errors.New("--infra is only supported with --mode mono")
	}
	if opts.preset != "" {
		return errors.New("--preset is only supported with --mode mono")
	}
	templateDir, err := registry.ResolveTemplateDir(opts.templateName, opts.templateDir)
	if err != nil {
		return err
	}
	dir := opts.dir
	if dir == "" {
		dir = filepath.Join(".", name)
	}
	res, err := micro.Generate(micro.Options{
		Name:          name,
		Module:        opts.module,
		Dir:           dir,
		AssetsVersion: assets.Version(),
		NCGOVersion:   Version,
		TemplateDir:   templateDir,
	})
	if err != nil {
		return err
	}
	// ... rest unchanged ...
}
```

- [ ] **Step 4: Add CLI tests for new flags**

Add to `internal/cli/root_test.go`:

```go
func TestNewMicroCmdAcceptsTemplateDirFlag(t *testing.T) {
	cmd := newNewCmd()
	f := cmd.Flags().Lookup("template-dir")
	if f == nil {
		t.Fatal("--template-dir flag not registered on ncgo new")
	}
}

func TestRunNewMicroTemplateAndTemplateDirMutuallyExclusive(t *testing.T) {
	// Use a similar pattern to TestRunNewMonoTemplateAndTemplateDirMutuallyExclusive
	// Verify that --template + --template-dir together produces an error
}
```

Add to `internal/cli/add_test.go` (or `root_test.go` if `add_test.go` doesn't exist):

```go
func TestAddRPCCmdHasTemplateFlags(t *testing.T) {
	cmd := newAddRPCCmd()
	for _, name := range []string{"template", "template-dir"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("add rpc missing --%s flag", name)
		}
	}
}

func TestAddBFFCmdHasPresetAndTemplateFlags(t *testing.T) {
	cmd := newAddBFFCmd()
	for _, name := range []string{"preset", "template", "template-dir"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("add bff missing --%s flag", name)
		}
	}
}

func TestAddRPCPresetAndTemplateMutuallyExclusive(t *testing.T) {
	// Verify that --preset + --template produces an error
}
```

- [ ] **Step 5: Run CLI tests**

Run: `go test ./internal/cli/... -count=1 -v`
Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/add.go internal/cli/root.go internal/cli/root_test.go internal/cli/add_test.go
git commit -m "feat(cli): add --template/--template-dir to add rpc/bff and wire to micro.Generate"
```

---

### Task 6: MCP tool schema and handler extensions

**Files:**
- Modify: `internal/mcp/tools.go:39-40` (InputSchema for ncgo_add_rpc, ncgo_add_bff)
- Modify: `internal/mcp/tool_rpc.go:12-61` (callAddRPC args)
- Modify: `internal/mcp/tool_bff.go:12-59` (callAddBFF args)
- Modify: `internal/mcp/tool_new.go:133-149` (runNewMicro)
- Modify: `internal/mcp/server_new_tools_test.go`

**Interfaces:**
- Consumes: `registry.ResolveTemplateDir` from Task 3
- Consumes: `rpc.Options.TemplateDir`, `bff.Options.TemplateDir`, `bff.Options.Preset`, `micro.Options.TemplateDir`

- [ ] **Step 1: Extend ncgo_add_rpc InputSchema**

Modify `internal/mcp/tools.go` ncgo_add_rpc entry:

```go
{Name: "ncgo_add_rpc", Description: "Add a Kitex RPC service to a micro workspace.", InputSchema: schemaObject([]string{"name", "root"}, rootField("Micro workspace root containing ncgo.workspace"), stringField("name", "RPC service name, e.g. \"payment-rpc\""), stringField("module", "Go module path; defaults to <workspace.module>/services/<name>"), stringField("dir", "Service directory relative to Root; defaults to services/<name>"), boolField("noGenerate", "Skip kitex invocation"), boolField("dryRun", "Preview without modifying files"), enumField("preset", []string{"rule-center"}), stringField("template", "Template package name from registry (kitex or micro kind)"), stringField("templateDir", "Template package local directory path (kitex or micro kind)"), outputTextJSONField())},
```

- [ ] **Step 2: Extend ncgo_add_bff InputSchema**

```go
{Name: "ncgo_add_bff", Description: "Add a Hertz BFF service to a micro workspace.", InputSchema: schemaObject([]string{"name", "root"}, rootField("Micro workspace root containing ncgo.workspace"), stringField("name", "BFF service name, e.g. \"user-api\""), stringField("module", "Go module path; defaults to <workspace.module>/services/<name>"), stringField("dir", "Service directory relative to Root; defaults to services/<name>"), boolField("noGenerate", "Skip hz invocation"), boolField("dryRun", "Preview without modifying files"), enumField("preset", []string{"rule-center"}), stringField("template", "Template package name from registry (hertz or micro kind)"), stringField("templateDir", "Template package local directory path (hertz or micro kind)"), outputTextJSONField())},
```

- [ ] **Step 3: Extend callAddRPC handler**

Modify `internal/mcp/tool_rpc.go` callAddRPC args struct:

```go
var args struct {
	Name        string `json:"name"`
	Root        string `json:"root"`
	Module      string `json:"module"`
	Dir         string `json:"dir"`
	NoGenerate  bool   `json:"noGenerate"`
	DryRun      bool   `json:"dryRun"`
	Preset      string `json:"preset"`
	Template    string `json:"template"`
	TemplateDir string `json:"templateDir"`
	Output      string `json:"output"`
}
```

Resolve and validate:

```go
templateDir, err := registry.ResolveTemplateDir(args.Template, args.TemplateDir)
if err != nil {
	return textResult(err.Error(), true), nil
}
if args.Preset != "" && templateDir != "" {
	return textResult("--preset and --template/--templateDir are mutually exclusive", true), nil
}
```

Pass to `rpc.Options`:

```go
res, err := rpc.Add(ctx, rpc.Options{
	// ... existing fields ...
	Preset:      args.Preset,
	TemplateDir: templateDir,
})
```

- [ ] **Step 4: Extend callAddBFF handler**

Modify `internal/mcp/tool_bff.go` callAddBFF args struct:

```go
var args struct {
	Name        string `json:"name"`
	Root        string `json:"root"`
	Module      string `json:"module"`
	Dir         string `json:"dir"`
	NoGenerate  bool   `json:"noGenerate"`
	DryRun      bool   `json:"dryRun"`
	Preset      string `json:"preset"`
	Template    string `json:"template"`
	TemplateDir string `json:"templateDir"`
	Output      string `json:"output"`
}
```

Same resolution and validation pattern as callAddRPC. Pass `Preset` and `TemplateDir` to `bff.Options`.

- [ ] **Step 5: Extend runNewMicro in MCP tool_new.go**

Modify `internal/mcp/tool_new.go` callNew args struct:

```go
var args struct {
	Name           string   `json:"name"`
	Module         string   `json:"module"`
	Dir            string   `json:"dir"`
	Mode           string   `json:"mode"`
	Kind           string   `json:"kind"`
	DB             string   `json:"db"`
	Infra          []string `json:"infra"`
	NoGenerate     bool     `json:"noGenerate"`
	Preset         string   `json:"preset"`
	RuleCenterAddr string   `json:"ruleCenterAddr"`
	Template       string   `json:"template"`
	TemplateDir    string   `json:"templateDir"`
	Output         string   `json:"output"`
}
```

Update `runNewMicro` signature and pass templateDir:

```go
case manifest.ModeMicro:
	templateDir, err := registry.ResolveTemplateDir(args.Template, args.TemplateDir)
	if err != nil {
		return textResult(err.Error(), true), nil
	}
	res, err = runNewMicro(args.Name, args.Module, dir, ncgoVersion, assetsVersion, templateDir)
```

Update `runNewMicro` function:

```go
func runNewMicro(name, module, dir, ncgoVersion, assetsVersion, templateDir string) (*newResult, error) {
	res, err := micro.Generate(micro.Options{
		Name:          name,
		Module:        module,
		Dir:           dir,
		AssetsVersion: assetsVersion,
		NCGOVersion:   ncgoVersion,
		TemplateDir:   templateDir,
	})
	// ...
}
```

Also update ncgo_new InputSchema to add `template` and `templateDir` fields (they may already exist — check).

- [ ] **Step 6: Add MCP integration tests**

Add to `internal/mcp/server_new_tools_test.go`:

```go
func TestServeToolCallAddRPCWithTemplate(t *testing.T) {
	// Create template package fixture
	// Call ncgo_add_rpc with template/templateDir parameter
	// Verify success and that template was applied
}

func TestServeToolCallAddBFFWithPresetAndTemplate(t *testing.T) {
	// Verify preset+template mutual exclusion via MCP
}
```

- [ ] **Step 7: Run MCP tests**

Run: `go test ./internal/mcp/... -count=1 -v`
Expected: ALL PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/tool_rpc.go internal/mcp/tool_bff.go internal/mcp/tool_new.go internal/mcp/server_new_tools_test.go
git commit -m "feat(mcp): extend ncgo_add_rpc/ncgo_add_bff/ncgo_new with template/preset parameters"
```

---

### Task 7: Golden tests and smoke tests

**Files:**
- Modify: `internal/scaffold/mono/golden_test.go` or `internal/scaffold/micro/golden_test.go` (if exists)
- Modify: `scripts/smoke.sh`

- [ ] **Step 1: Add golden test for micro workspace with template**

Create a fixture under `internal/scaffold/micro/testdata/` (e.g., `micro-template-pkg/`) with:
- `template.yaml` (`kind: micro`)
- `workspace/compose.yaml.tpl`
- `kitex-template/a.yaml`
- `hertz-template/a.yaml`

Write a golden test that generates with `--template-dir` pointing to the fixture and compares output to a checked-in snapshot.

- [ ] **Step 2: Add golden test for add rpc with micro template package**

Create fixture and golden test verifying `add rpc --template-dir <micro-pkg>` extracts kitex templates.

- [ ] **Step 3: Add smoke test scenarios**

Add to `scripts/smoke.sh`:

```bash
# Scenario: ncgo add rpc with --template-dir (no-generate to avoid kitex dependency)
echo "=== add rpc with template-dir ==="
# ... setup workspace, create template fixture, run ncgo add rpc --template-dir <fixture> --no-generate ...

# Scenario: ncgo new --mode micro with --template-dir
echo "=== new micro with template-dir ==="
# ... create template fixture, run ncgo new --mode micro --template-dir <fixture> ...
```

- [ ] **Step 4: Run golden tests and smoke tests**

Run: `go test ./internal/scaffold/micro/... -run TestGolden -count=1 -v`
Run: `./scripts/smoke.sh`
Expected: ALL PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/micro/testdata/ scripts/smoke.sh
git commit -m "test: add golden tests and smoke scenarios for micro template consumption"
```

---

### Task 8: Documentation (EN + ZH aligned)

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/examples.md`
- Modify: `docs/examples.zh-CN.md`

- [ ] **Step 1: Update README command tables**

Add to `ncgo add rpc` flag list (both EN and ZH):

```
--template <name>       Template package name from registry (kitex or micro kind)
--template-dir <dir>    Template package local directory path
```

Add to `ncgo add bff` flag list:

```
--preset <name>         Preset template name (e.g., rule-center)
--template <name>       Template package name from registry (hertz or micro kind)
--template-dir <dir>    Template package local directory path
```

Update `ncgo new --mode micro` description to mention `--template`/`--template-dir` support.

- [ ] **Step 2: Add micro template consumption section to examples**

Add a new section in `docs/examples.md` (and mirror in `docs/examples.zh-CN.md`):

```markdown
## Micro workspace template consumption

Create a micro workspace with a custom template:

    ncgo new my-workspace --mode micro --module github.com/acme/my-workspace --template my-micro

Add services using the same micro template package:

    ncgo add rpc user-rpc --template my-micro
    ncgo add bff web-bff --template my-micro

Or use standalone mono template packages:

    ncgo add rpc user-rpc --template base-kitex
    ncgo add bff web-bff --template base-hertz

### Micro template package structure

A micro template package (`kind: micro`) bundles workspace skeleton templates and service-layer templates:

    my-micro/
    ├── template.yaml          # kind: micro
    ├── workspace/             # workspace skeleton templates
    │   ├── compose.yaml.tpl   # .tpl suffix → variable substitution
    │   └── scripts/
    ├── kitex-template/        # RPC service templates (used by add rpc)
    ├── hertz-template/        # BFF service templates (used by add bff)
    └── idl/
        ├── kitex/             # Kitex IDL templates
        └── hertz/             # Hertz IDL templates
```

- [ ] **Step 3: Run markdown diagnostics**

Run markdown lint on changed files. Verify EN/ZH alignment.

- [ ] **Step 4: Commit**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: document micro template consumption and add rpc/bff --template flags"
```

---

## Self-Review Checklist

After completing all tasks, verify:

1. **Spec coverage:** All items in design spec §4-§10 have corresponding tasks.
2. **Placeholder scan:** No TBD/TODO in the plan.
3. **Type consistency:** `TemplateDir`, `Preset`, `ResolveTemplateDir`, `OverlayWorkspaceTemplates`, `RenderData` names consistent across tasks.
4. **Backward compat:** Existing mono tests still pass without changes (verified in Task 1).
5. **Contract surfaces:** CLI flags, MCP schemas, golden tests all updated together.
