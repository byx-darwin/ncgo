# rule-center 模版包自包含实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `ncgo new --kind kitex --template rule-center` 与 `--preset rule-center` 消费体验等价——模版包自包含所有产物（schema、shared fragments、layout），通过 `template.yaml` 的 `skip_default_templates` 字段显式声明跳过默认模板。

**Architecture:** 扩展模版包规范，增加 `skip_default_templates`、`schema/`、`layout.yaml` 支持。`overlayTemplatePackage` 增强后负责复制这些额外产物并传递跳过列表给 `writeKitexTemplate`。所有文件用 `scaffoldtemplate.Render` 渲染变量（`{{.Module}}` + `{{.ServiceName}}`）。

**Tech Stack:** Go 1.25+, gopkg.in/yaml.v3, embedded FS

## Global Constraints

- 所有新字段可选，缺省行为与现有完全一致（向后兼容）
- 变量渲染用 `scaffoldtemplate.Render`（`{{.Module}}` + `{{.ServiceName}}`）
- Golden 测试覆盖等价性
- 文档 EN/ZH 同步

---

### Task 1: 扩展 PackageMeta 和 Package 结构

**Files:**
- Modify: `internal/scaffold/template/package.go:14-31`
- Test: `internal/scaffold/template/package_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `PackageMeta.SkipDefaultTemplates []string`, `Package.SchemaDir string`, `Package.Schemas []string`, `Package.LayoutFile string`

- [ ] **Step 1: Write failing test for PackageMeta extension**

```go
// internal/scaffold/template/package_test.go
func TestLoadPackage_SkipDefaultTemplates(t *testing.T) {
    dir := t.TempDir()
    // Create template.yaml with skip_default_templates
    meta := `name: test-pkg
kind: kitex
description: test
version: 1
skip_default_templates:
  - handler.yaml
  - server.yaml
`
    if err := os.WriteFile(filepath.Join(dir, "template.yaml"), []byte(meta), 0644); err != nil {
        t.Fatal(err)
    }
    // Create kitex-template dir with a dummy yaml
    tplDir := filepath.Join(dir, "kitex-template")
    if err := os.MkdirAll(tplDir, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(tplDir, "test.yaml"), []byte("path: test.go\nbody: test\n"), 0644); err != nil {
        t.Fatal(err)
    }

    pkg, err := LoadPackage(dir, "kitex")
    if err != nil {
        t.Fatalf("LoadPackage: %v", err)
    }
    if !pkg.HasMeta {
        t.Fatal("expected HasMeta=true")
    }
    if len(pkg.Meta.SkipDefaultTemplates) != 2 {
        t.Fatalf("expected 2 skip_default_templates, got %d", len(pkg.Meta.SkipDefaultTemplates))
    }
    if pkg.Meta.SkipDefaultTemplates[0] != "handler.yaml" {
        t.Errorf("expected handler.yaml, got %s", pkg.Meta.SkipDefaultTemplates[0])
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestLoadPackage_SkipDefaultTemplates -v`
Expected: FAIL — `pkg.Meta.SkipDefaultTemplates` undefined

- [ ] **Step 3: Extend PackageMeta struct**

```go
// internal/scaffold/template/package.go
type PackageMeta struct {
    Name                 string   `yaml:"name"`
    Kind                 string   `yaml:"kind"`
    Description          string   `yaml:"description"`
    Version              string   `yaml:"version"`
    SkipDefaultTemplates []string `yaml:"skip_default_templates"` // NEW
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/template/... -run TestLoadPackage_SkipDefaultTemplates -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/template/package.go internal/scaffold/template/package_test.go
git commit -m "feat(template): add SkipDefaultTemplates to PackageMeta"
```

---

### Task 2: 扩展 Package 结构支持 schema 和 layout

**Files:**
- Modify: `internal/scaffold/template/package.go:22-31`
- Modify: `internal/scaffold/template/package.go:51-99` (LoadPackage)
- Test: `internal/scaffold/template/package_test.go`

**Interfaces:**
- Consumes: `PackageMeta.SkipDefaultTemplates` from Task 1
- Produces: `Package.SchemaDir`, `Package.Schemas`, `Package.LayoutFile`

- [ ] **Step 1: Write failing test for schema and layout**

```go
func TestLoadPackage_SchemaAndLayout(t *testing.T) {
    dir := t.TempDir()
    // Create minimal template.yaml
    if err := os.WriteFile(filepath.Join(dir, "template.yaml"), []byte("name: test\nkind: kitex\n"), 0644); err != nil {
        t.Fatal(err)
    }
    // Create kitex-template dir
    tplDir := filepath.Join(dir, "kitex-template")
    if err := os.MkdirAll(tplDir, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(tplDir, "test.yaml"), []byte("path: test.go\nbody: test\n"), 0644); err != nil {
        t.Fatal(err)
    }
    // Create schema dir with SQL file
    schemaDir := filepath.Join(dir, "schema")
    if err := os.MkdirAll(schemaDir, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(schemaDir, "000002_test.sql"), []byte("CREATE TABLE test;"), 0644); err != nil {
        t.Fatal(err)
    }
    // Create layout.yaml
    if err := os.WriteFile(filepath.Join(dir, "layout.yaml"), []byte("templates:"), 0644); err != nil {
        t.Fatal(err)
    }

    pkg, err := LoadPackage(dir, "kitex")
    if err != nil {
        t.Fatalf("LoadPackage: %v", err)
    }
    if pkg.SchemaDir == "" {
        t.Error("expected SchemaDir to be set")
    }
    if len(pkg.Schemas) != 1 {
        t.Errorf("expected 1 schema, got %d", len(pkg.Schemas))
    }
    if pkg.LayoutFile == "" {
        t.Error("expected LayoutFile to be set")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/template/... -run TestLoadPackage_SchemaAndLayout -v`
Expected: FAIL — `pkg.SchemaDir` undefined

- [ ] **Step 3: Extend Package struct**

```go
type Package struct {
    Dir         string // absolute package root
    Meta        PackageMeta
    HasMeta     bool     // false when template.yaml is absent
    TemplateDir string   // absolute <kind>-template directory
    Templates   []string // absolute .yaml paths under TemplateDir
    IDLDir      string   // absolute idl directory (may not exist)
    IDLs        []string // absolute .proto paths under IDLDir
    // NEW fields
    SchemaDir  string   // absolute schema directory (may not exist)
    Schemas    []string // absolute .sql paths under SchemaDir
    LayoutFile string   // absolute layout.yaml path (may not exist)
}
```

- [ ] **Step 4: Update LoadPackage to parse schema and layout**

```go
func LoadPackage(dir, kind string) (*Package, error) {
    // ... existing code ...

    // Parse schema directory
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

    // Parse layout.yaml
    layoutPath := filepath.Join(abs, "layout.yaml")
    if _, err := os.Stat(layoutPath); err == nil {
        pkg.LayoutFile = layoutPath
    }

    return pkg, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/scaffold/template/... -run TestLoadPackage_SchemaAndLayout -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/template/package.go internal/scaffold/template/package_test.go
git commit -m "feat(template): add SchemaDir/Schemas/LayoutFile to Package"
```

---

### Task 3: 扩展 Options 结构

**Files:**
- Modify: `internal/scaffold/mono/mono.go:41-57`
- Test: 无需单独测试，后续集成测试覆盖

**Interfaces:**
- Consumes: `PackageMeta.SkipDefaultTemplates` from Task 1
- Produces: `Options.SkipDefaultTemplates []string`

- [ ] **Step 1: Extend Options struct**

```go
// internal/scaffold/mono/mono.go
type Options struct {
    // ... existing fields ...
    TemplateDir          string   // external template package dir; replaces embedded <kind>-template and IDL placeholder
    SkipDefaultTemplates []string // NEW: from template package's skip_default_templates
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/scaffold/mono/mono.go
git commit -m "feat(mono): add SkipDefaultTemplates to Options"
```

---

### Task 4: 修改 writeKitexTemplate 支持跳过列表

**Files:**
- Modify: `internal/scaffold/mono/files.go:162-269` (writeKitexTemplate)
- Test: `internal/scaffold/mono/files_test.go`

**Interfaces:**
- Consumes: `Options.SkipDefaultTemplates` from Task 3
- Produces: 跳过指定默认模板的逻辑

- [ ] **Step 1: Write failing test for skip logic**

```go
// internal/scaffold/mono/files_test.go
func TestWriteKitexTemplate_SkipDefaultTemplates(t *testing.T) {
    dir := t.TempDir()
    opts := Options{
        Kind:                 "kitex",
        Module:               "example.com/test",
        Name:                 "test",
        SkipDefaultTemplates: []string{"handler.yaml", "server.yaml"},
    }
    if err := writeKitexTemplate(dir, "", opts); err != nil {
        t.Fatalf("writeKitexTemplate: %v", err)
    }
    tplDir := filepath.Join(dir, "template", "kitex-template")
    // handler.yaml and server.yaml should NOT exist
    if _, err := os.Stat(filepath.Join(tplDir, "handler.yaml")); err == nil {
        t.Error("handler.yaml should be skipped")
    }
    if _, err := os.Stat(filepath.Join(tplDir, "server.yaml")); err == nil {
        t.Error("server.yaml should be skipped")
    }
    // usecase.yaml should exist
    if _, err := os.Stat(filepath.Join(tplDir, "usecase.yaml")); err != nil {
        t.Error("usecase.yaml should exist")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/mono/... -run TestWriteKitexTemplate_SkipDefaultTemplates -v`
Expected: FAIL — handler.yaml exists

- [ ] **Step 3: Modify writeKitexTemplate signature and logic**

```go
// Change signature from:
// func writeKitexTemplate(dir string, preset string, module string) error
// To:
func writeKitexTemplate(dir string, opts Options) error {
    preset := opts.Preset
    module := opts.Module
    // ... existing code ...

    for _, e := range entries {
        if e.IsDir() {
            continue
        }
        name := e.Name()
        // Rule-center template files are only copied when preset is "rule-center".
        if preset != "rule-center" && strings.HasPrefix(name, "ratelimit_") {
            continue
        }
        // Rule-center preset provides its own ratelimit_handler/server/usecase/repository
        // templates under the rulecenter/ dirs; skip the default per-layer templates
        // so they don't generate duplicate ruleservice/ scaffolding.
        if preset == "rule-center" && (name == "handler.yaml" || name == "server.yaml" || name == "usecase.yaml" || name == "repository.yaml") {
            continue
        }
        // NEW: Check SkipDefaultTemplates from template package
        if slices.Contains(opts.SkipDefaultTemplates, name) {
            continue
        }
        // ... rest of the loop ...
    }
    // ... rest of function ...
}
```

- [ ] **Step 4: Update writeTemplate call site**

```go
func writeTemplate(dir string, opts Options) error {
    if defaultKind(opts.Kind) == manifest.KindKitex {
        return writeKitexTemplate(dir, opts) // Changed from: writeKitexTemplate(dir, opts.Preset, opts.Module)
    }
    return writeHertzTemplate(dir, opts)
}
```

- [ ] **Step 5: Add slices import**

```go
import (
    // ... existing imports ...
    "slices"
)
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/scaffold/mono/... -run TestWriteKitexTemplate_SkipDefaultTemplates -v`
Expected: PASS

- [ ] **Step 7: Run all mono tests to ensure no regression**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: All tests PASS

- [ ] **Step 8: Commit**

```bash
git add internal/scaffold/mono/files.go internal/scaffold/mono/files_test.go
git commit -m "feat(mono): writeKitexTemplate respects SkipDefaultTemplates"
```

---

### Task 5: 增强 overlayTemplatePackage

**Files:**
- Modify: `internal/scaffold/mono/files.go:457-549` (overlayTemplatePackage)
- Test: `internal/scaffold/mono/overlay_test.go`

**Interfaces:**
- Consumes: `Package.SchemaDir`, `Package.Schemas`, `Package.LayoutFile`, `Package.Meta.SkipDefaultTemplates` from Task 2
- Produces: 设置 `opts.SkipDefaultTemplates`，复制 schema 和 layout

- [ ] **Step 1: Write failing test for overlay enhancement**

```go
// internal/scaffold/mono/overlay_test.go
func TestOverlayTemplatePackage_SchemaAndLayout(t *testing.T) {
    // Create a template package with schema and layout
    pkgDir := t.TempDir()
    // template.yaml
    meta := `name: test-pkg
kind: kitex
skip_default_templates:
  - handler.yaml
`
    if err := os.WriteFile(filepath.Join(pkgDir, "template.yaml"), []byte(meta), 0644); err != nil {
        t.Fatal(err)
    }
    // kitex-template dir
    tplDir := filepath.Join(pkgDir, "kitex-template")
    if err := os.MkdirAll(tplDir, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(tplDir, "test.yaml"), []byte("path: test.go\nbody: |\n  package test\n  // {{.Module}}\n"), 0644); err != nil {
        t.Fatal(err)
    }
    // schema dir
    schemaDir := filepath.Join(pkgDir, "schema")
    if err := os.MkdirAll(schemaDir, 0755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(schemaDir, "000002_test.sql"), []byte("CREATE TABLE {{.Module}};"), 0644); err != nil {
        t.Fatal(err)
    }
    // layout.yaml
    if err := os.WriteFile(filepath.Join(pkgDir, "layout.yaml"), []byte("templates:\n  - path: test.go\n"), 0644); err != nil {
        t.Fatal(err)
    }

    // Create target dir
    targetDir := t.TempDir()
    if err := os.MkdirAll(filepath.Join(targetDir, "template", "kitex-template"), 0755); err != nil {
        t.Fatal(err)
    }

    opts := Options{
        Kind:        "kitex",
        Module:      "example.com/mymodule",
        Name:        "mymodule",
        TemplateDir: pkgDir,
    }
    fallback, err := overlayTemplatePackage(targetDir, opts)
    if err != nil {
        t.Fatalf("overlayTemplatePackage: %v", err)
    }
    if fallback {
        t.Error("expected fallback=false")
    }
    // Check SkipDefaultTemplates was set
    if len(opts.SkipDefaultTemplates) != 1 || opts.SkipDefaultTemplates[0] != "handler.yaml" {
        t.Errorf("expected SkipDefaultTemplates=[handler.yaml], got %v", opts.SkipDefaultTemplates)
    }
    // Check schema was copied
    schemaPath := filepath.Join(targetDir, "internal", "db", "schema", "000002_test.sql")
    if _, err := os.Stat(schemaPath); err != nil {
        t.Errorf("schema file not copied: %v", err)
    }
    // Check schema was rendered
    content, _ := os.ReadFile(schemaPath)
    if !strings.Contains(string(content), "example.com/mymodule") {
        t.Error("schema not rendered with Module")
    }
    // Check layout was copied
    layoutPath := filepath.Join(targetDir, "template", "layout.yaml")
    if _, err := os.Stat(layoutPath); err != nil {
        t.Errorf("layout file not copied: %v", err)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/mono/... -run TestOverlayTemplatePackage_SchemaAndLayout -v`
Expected: FAIL — schema file not copied

- [ ] **Step 3: Enhance overlayTemplatePackage**

```go
func overlayTemplatePackage(dir string, opts Options) (bool, error) {
    kind := defaultKind(opts.Kind)
    pkg, err := scaffoldtemplate.LoadPackage(opts.TemplateDir, kind)
    if err != nil {
        return false, err
    }

    // NEW: Pass skip_default_templates to opts
    opts.SkipDefaultTemplates = pkg.Meta.SkipDefaultTemplates

    // ... existing template copying logic ...

    // NEW: Copy schema files
    if len(pkg.Schemas) > 0 {
        schemaTarget := filepath.Join(dir, "internal", "db", "schema")
        if err := os.MkdirAll(schemaTarget, 0o755); err != nil {
            return false, fmt.Errorf("scaffold: mkdir %s: %w", schemaTarget, err)
        }
        for _, src := range pkg.Schemas {
            b, err := os.ReadFile(src)
            if err != nil {
                return false, fmt.Errorf("scaffold: read schema %s: %w", src, err)
            }
            rendered, err := scaffoldtemplate.Render(string(b), scaffoldtemplate.RenderData{
                Module:      opts.Module,
                ServiceName: opts.Name,
            })
            if err != nil {
                return false, fmt.Errorf("scaffold: render schema %s: %w", src, err)
            }
            target := filepath.Join(schemaTarget, filepath.Base(src))
            if err := os.WriteFile(target, []byte(rendered), 0o644); err != nil {
                return false, fmt.Errorf("scaffold: write %s: %w", target, err)
            }
        }
    }

    // NEW: Copy layout.yaml
    if pkg.LayoutFile != "" {
        b, err := os.ReadFile(pkg.LayoutFile)
        if err != nil {
            return false, fmt.Errorf("scaffold: read layout %s: %w", pkg.LayoutFile, err)
        }
        layoutTarget := filepath.Join(dir, "template", "layout.yaml")
        if err := os.MkdirAll(filepath.Dir(layoutTarget), 0o755); err != nil {
            return false, fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(layoutTarget), err)
        }
        if err := os.WriteFile(layoutTarget, b, 0o644); err != nil {
            return false, fmt.Errorf("scaffold: write %s: %w", layoutTarget, err)
        }
    }

    // ... existing IDL logic ...
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/mono/... -run TestOverlayTemplatePackage_SchemaAndLayout -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/mono/files.go internal/scaffold/mono/overlay_test.go
git commit -m "feat(mono): overlayTemplatePackage copies schema and layout"
```

---

### Task 6: 集成测试 — rule-center 模版包等价性

**Files:**
- Create: `internal/scaffold/mono/testdata/template-rule-center/` (test fixture)
- Test: `internal/scaffold/mono/golden_test.go`

**Interfaces:**
- Consumes: All previous tasks
- Produces: Golden test verifying `--template rule-center` ≡ `--preset rule-center`

- [ ] **Step 1: Create test fixture — rule-center template package**

Create `internal/scaffold/mono/testdata/template-rule-center/` with:
- `template.yaml` (with `skip_default_templates`)
- `kitex-template/` (with ratelimit_*.yaml files copied from embedded assets)
- `idl/rulecenter.proto` (copied from embedded ratelimit_proto.yaml)
- `schema/000002_rate_limit_rules.sql` (copied from embedded asset)
- `layout.yaml` (copied from embedded layout-rulecenter.yaml)

- [ ] **Step 2: Write golden test**

```go
// internal/scaffold/mono/golden_test.go
func TestGenerate_TemplateRuleCenter_EquivalentToPreset(t *testing.T) {
    if testing.Short() {
        t.Skip("golden test requires embedded assets")
    }
    // Generate with --preset rule-center
    presetDir := t.TempDir()
    presetOpts := Options{
        Name:       "rulecenter",
        Module:     "example.com/rulecenter",
        Kind:       "kitex",
        Dir:        presetDir,
        Preset:     "rule-center",
        NoGenerate: true,
        Now:        time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
    }
    if _, err := Generate(context.Background(), presetOpts); err != nil {
        t.Fatalf("preset Generate: %v", err)
    }

    // Generate with --template rule-center
    templateDir := t.TempDir()
    templateOpts := Options{
        Name:        "rulecenter",
        Module:      "example.com/rulecenter",
        Kind:        "kitex",
        Dir:         templateDir,
        TemplateDir: "testdata/template-rule-center",
        NoGenerate:  true,
        Now:         time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
    }
    if _, err := Generate(context.Background(), templateOpts); err != nil {
        t.Fatalf("template Generate: %v", err)
    }

    // Compare key files
    keyFiles := []string{
        "internal/db/schema/000002_rate_limit_rules.sql",
        "template/layout.yaml",
        "idl/rule-center.proto",
    }
    for _, f := range keyFiles {
        presetContent, err := os.ReadFile(filepath.Join(presetDir, f))
        if err != nil {
            t.Fatalf("read preset %s: %v", f, err)
        }
        templateContent, err := os.ReadFile(filepath.Join(templateDir, f))
        if err != nil {
            t.Fatalf("read template %s: %v", f, err)
        }
        if string(presetContent) != string(templateContent) {
            t.Errorf("file %s differs between preset and template", f)
        }
    }

    // Check that handler.yaml etc. are skipped
    tplDir := filepath.Join(templateDir, "template", "kitex-template")
    for _, skip := range []string{"handler.yaml", "server.yaml", "usecase.yaml", "repository.yaml"} {
        if _, err := os.Stat(filepath.Join(tplDir, skip)); err == nil {
            t.Errorf("file %s should be skipped", skip)
        }
    }

    // Check ratelimit_shared_*.yaml exist
    for _, name := range []string{"resolver", "resolver_test", "store", "store_test", "rule_center_client"} {
        path := filepath.Join(tplDir, "ratelimit_shared_"+name+".yaml")
        if _, err := os.Stat(path); err != nil {
            t.Errorf("expected %s to exist", path)
        }
    }
}
```

- [ ] **Step 3: Run test and iterate until it passes**

Run: `go test ./internal/scaffold/mono/... -run TestGenerate_TemplateRuleCenter_EquivalentToPreset -v`
Expected: PASS after all previous tasks are complete

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/mono/testdata/template-rule-center/ internal/scaffold/mono/golden_test.go
git commit -m "test(mono): golden test for --template rule-center equivalence"
```

---

### Task 7: 文档更新

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`
- Modify: `docs/examples.md`
- Modify: `docs/examples.zh-CN.md`

**Interfaces:**
- Consumes: All previous tasks
- Produces: Updated documentation

- [ ] **Step 1: Update README.md — template.yaml spec**

Add to template package spec section:

```markdown
### template.yaml

```yaml
name: base-kitex
kind: kitex
description: Official Kitex base service template
version: 1
skip_default_templates:  # Optional: skip these default templates
  - handler.yaml
  - server.yaml
```

**Fields:**
- `name`: Template package name (matches directory name)
- `kind`: Service kind (`hertz` | `kitex`)
- `description`: Human-readable description
- `version`: Template version (incremented by registry maintainers)
- `skip_default_templates`: Optional list of default templates to skip (e.g., when template provides its own per-layer templates)
```

- [ ] **Step 2: Update README.zh-CN.md — same content in Chinese**

- [ ] **Step 3: Update docs/examples.md — template package structure**

Add section:

```markdown
### Template Package with Schema and Layout

Template packages can include additional artifacts for preset-equivalent behavior:

```
rule-center/
├── template.yaml                # Metadata with skip_default_templates
├── kitex-template/              # Per-file templates
│   ├── ratelimit_handler.yaml
│   ├── ratelimit_shared_*.yaml  # Shared fragments
│   └── ...
├── idl/                         # IDL files
│   └── rulecenter.proto
├── schema/                      # SQL schema files (rendered with {{.Module}})
│   └── 000002_rate_limit_rules.sql
└── layout.yaml                  # Custom layout (replaces default)
```

**Variable rendering:** All template files (including `schema/*.sql` and `kitex-template/*.yaml`) are rendered with `{{.Module}}` and `{{.ServiceName}}` using the same `scaffoldtemplate.Render` engine as IDL files.
```

- [ ] **Step 4: Update docs/examples.zh-CN.md — same content in Chinese**

- [ ] **Step 5: Run markdown diagnostics**

Run: markdown linter on all 4 files
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: document template package skip_default_templates, schema, layout"
```

---

### Task 8: 最终验证

**Files:**
- All previous files

**Interfaces:**
- Consumes: All previous tasks
- Produces: Full validation passing

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: All tests PASS

- [ ] **Step 2: Run smoke test**

Run: `./scripts/smoke.sh`
Expected: PASS

- [ ] **Step 3: Run lint**

Run: `go vet ./... && gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: No output (clean)

- [ ] **Step 4: Create PR**

```bash
git push origin feat/56-rule-center-template-self-contained
gh pr create --title "feat(template): rule-center template package self-contained" --body "Closes #56"
```

---

## 实现顺序总结

1. **Task 1-2**: 扩展 `template` 包的结构（PackageMeta, Package）
2. **Task 3**: 扩展 `mono.Options`
3. **Task 4**: 修改 `writeKitexTemplate` 支持跳过列表
4. **Task 5**: 增强 `overlayTemplatePackage` 复制 schema/layout
5. **Task 6**: 集成测试验证等价性
6. **Task 7**: 文档更新
7. **Task 8**: 最终验证和 PR
