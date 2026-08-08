# 官方模版 Registry 闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 打通 `ncgo export templates` → 官方 registry 审核 → `ncgo new --template/--template-dir` 消费的模版闭环，并修复审计发现的 export 缺陷（Issue #54）。

**Architecture:** 分五层递进——(1) `internal/scaffold/template` 修复 export（小写服务名变量化 + IDL 导出）并新增模版包加载（LoadPackage）；(2) `internal/scaffold/mono` 增加 `Options.TemplateDir`，用外部模版包替换内置 `<kind>-template/` 与 IDL 占位；(3) 新包 `internal/registry` 提供 git-based list/pull 客户端；(4) CLI 新增 `ncgo template list/pull` 与 `ncgo new --template/--template-dir`；(5) MCP 双轨工具 + 描述修正。全程 TDD，golden 锁定缺省路径向后兼容。

**Tech Stack:** Go 1.25+, Cobra, gopkg.in/yaml.v3, text/template（既有渲染器 `template.Render`）, git（经 `internal/exec.Runner` 调用，测试用 fake）, testutil/golden 快照。

**Reference spec:** `docs/superpowers/specs/2026-08-08-template-registry-closed-loop-design.md`（commit 5330691）
**Issue:** #54 · **评审补充要求（comment 5224486974）：** 错误路径验收（registry 不可用 / 缺 idl/ 向后兼容 / template.yaml 解析失败）+ 缺省路径向后兼容（既有 mono golden 不得变更）。

## Global Constraints

- Go 1.25+；所有文件 `gofmt` 干净；`go vet` 无告警。
- CLI 输出文本、MCP `content[0].text`/顶层结构化字段/isError 语义、模版 YAML 格式均为契约面，改动需同步测试与文档。
- 缺省行为（不传 `--template-dir`/`--template`）必须与现版本逐字节一致：`internal/scaffold/mono` 既有 golden（`TestGenerateGoldenDefault` 等 4 项）不允许 `-update-golden`。
- `--preset` 与 `--template-dir`/`--template` 互斥；`--template` 与 `--template-dir` 互斥。
- 错误信息清晰、带上下文；跨包边界 wrap error。
- 提交信息用 conventional commit（feat/fix/test/docs + scope）。
- 文档 EN/ZH 对齐：README.md/README.zh-CN.md、docs/examples.md/docs/examples.zh-CN.md。
- 每个任务独立可测、独立提交；任务间依赖见 Interfaces 块。

## 依赖顺序

```
T1（小写替换）→ T2（IDL 导出）→ T3（包加载）→ T4（mono 消费 + --template-dir）
                                          ↘ T5（registry + template 命令 + --template）→ T6（MCP）
T7（golden + smoke，依赖 T2/T4）→ T8（文档）→ T9（全量验证）
```

复杂度评分（gf-workflow）：每个任务跨模块或改公共 API → 均为 independent subagent + review。

---

### Task 1: export 小写服务名变量化（缺陷修复）

**Files:**
- Modify: `internal/scaffold/template/export.go`（`replaceServiceName`，约 :209-226）
- Test: `internal/scaffold/template/export_test.go`

**Interfaces:**
- Consumes: 无（既有 `serviceNameLower(name) string`、`exportName(s) string`）
- Produces: `replaceServiceName(body, serviceName string) string` 行为增强——小写服务名作为**带边界的词元**（路径段、package 声明、包限定符、引号内路径）被替换为 `{{ToLower .ServiceName}}`。T2 的 IDL 导出与 T7 的 golden 依赖此行为。

**背景**：现状只替换 PascalCase 标识符（`UserApi` → `{{.ServiceName}}`）。body 与 import 路径中的小写服务名（`internal/handler/userrpc`、`package userrpc`）保持字面量，导致换服务名生成时 import 指向不存在的包 → 编译失败（`TestReplaceServiceName_ImportPath` 注释确认此为已知局限）。

- [ ] **Step 1: 写失败测试**

在 `export_test.go` 追加：

```go
func TestReplaceServiceName_LowercaseSegments(t *testing.T) {
	body := "import \"{{.Module}}/internal/handler/userrpc\"\n" +
		"package userrpc\n" +
		"_ = userrpc.Ping()\n"
	got := replaceServiceName(body, "UserRpc")
	if strings.Contains(got, "userrpc") {
		t.Errorf("lowercase service name should be replaced:\n%s", got)
	}
	if strings.Count(got, "{{ToLower .ServiceName}}") != 3 {
		t.Errorf("expected 3 lowercase substitutions, got:\n%s", got)
	}
}

func TestReplaceServiceName_LowercaseNoFalsePositive(t *testing.T) {
	body := "userrpc2 := 1\nmyuserrpc := 2\nUserRpcExtra := 3"
	got := replaceServiceName(body, "UserRpc")
	if !strings.Contains(got, "userrpc2") || !strings.Contains(got, "myuserrpc") {
		t.Errorf("non-boundary lowercase occurrences must be kept:\n%s", got)
	}
	if !strings.Contains(got, "{{.ServiceName}}Extra") {
		t.Errorf("PascalCase substitution must still work:\n%s", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/template/ -run 'TestReplaceServiceName_Lowercase' -count=1`
Expected: FAIL（`userrpc` 未被替换）

- [ ] **Step 3: 最小实现**

`export.go` 的 `replaceServiceName` 末尾（return 前）追加：

```go
	// Replace lowercase service name occurrences that appear as bounded
	// tokens: path segments, package declarations, package qualifiers,
	// and quoted import paths. Non-boundary occurrences (userrpc2,
	// myuserrpc) are left untouched.
	lower := serviceNameLower(serviceName)
	if lower != "" {
		segRE := regexp.MustCompile(`(^|[\s/."'])` + regexp.QuoteMeta(lower) + `($|[\s/."'])`)
		body = segRE.ReplaceAllString(body, "${1}{{ToLower .ServiceName}}${2}")
	}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/scaffold/template/ -count=1`
Expected: PASS（全包，含既有测试）

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/template/export.go internal/scaffold/template/export_test.go
git commit -m "fix(template): substitute lowercase service name tokens on export"
```

---

### Task 2: export 纳入 IDL（含路径/内容变量化）

**Files:**
- Modify: `internal/scaffold/template/export.go`（`Export`、新增 `exportIDLs`/`idlTemplatePath`；`ExportResult` 加字段）
- Modify: `internal/cli/export.go`（输出文本含 IDL 计数）
- Test: `internal/scaffold/template/export_test.go`、`internal/cli/export_test.go`

**Interfaces:**
- Consumes: T1 的 `replaceServiceName`（proto 内 `service UserRpc` → `{{.ServiceName}}`、go_package 内小写名 → `{{ToLower .ServiceName}}`）
- Produces:
  - `ExportResult{ OutputDir string; Templates []string; IDLs []string }`（IDLs 为相对项目根的 `idl/...` 列表，walk 字典序）
  - 导出产物新增 `<root>/template/idl/<参数化相对路径>`；**IDL 文件名中的服务名被参数化为 `{{ToLower .ServiceName}}`**（保证目标项目渲染后与 `defaultIDL` 路径一致：hertz `idl/app/<name>.proto`、kitex `idl/<kitexIDLBase>.proto`）
  - 排除 `idl/openapi/`、`idl/validate/`（hz 标准支持文件，scaffold 内置重写）
  - 源项目无 `idl/` 目录时静默跳过（IDLs 为空）
  - CLI 输出：有 IDL 时 `exported %d templates and %d IDL files to %s/`，否则保持原文本 `exported %d templates to %s/`（既有测试 grep `exported ` 兼容）

- [ ] **Step 1: 写失败测试（包级）**

```go
func TestExport_IDL(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n")
	writeFileExport(t, dir, "idl/app/userapi.proto",
		"syntax = \"proto3\";\npackage app;\n"+
			"option go_package = \"github.com/acme/test/kitex_gen/userapi\";\n"+
			"service UserApi {\n  rpc Ping(PingReq) returns (PingResp);\n}\n")
	writeFileExport(t, dir, "idl/openapi/openapi.proto", "syntax = \"proto3\";\n")

	result, err := Export(ExportOptions{Root: dir, Kind: "hertz",
		Module: "github.com/acme/test", ServiceName: "UserApi"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(result.IDLs) != 1 || result.IDLs[0] != "idl/app/{{ToLower .ServiceName}}.proto" {
		t.Fatalf("IDLs = %v", result.IDLs)
	}
	body, err := os.ReadFile(filepath.Join(dir, "template", "idl", "app", "{{ToLower .ServiceName}}.proto"))
	if err != nil {
		t.Fatalf("exported idl missing: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "service {{.ServiceName}} {") {
		t.Errorf("service name not variabilized:\n%s", s)
	}
	if !strings.Contains(s, "{{.Module}}/kitex_gen/{{ToLower .ServiceName}}") {
		t.Errorf("go_package not variabilized:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(dir, "template", "idl", "openapi")); !os.IsNotExist(err) {
		t.Error("idl/openapi must be excluded from export")
	}
}
```

（`writeFileExport` 为本测试文件内的 helper：mkdir + WriteFile，参照 cli 包 `writeFile` 模式新增。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/template/ -run TestExport_IDL -count=1`
Expected: FAIL（`IDLs` 字段不存在 → 编译失败）

- [ ] **Step 3: 实现**

`ExportResult` 加字段 `IDLs []string`。`Export()` 在 Makefile 导出后追加：

```go
	idls, err := exportIDLs(absRoot, opts)
	if err != nil {
		return nil, fmt.Errorf("export idl: %w", err)
	}

	return &ExportResult{
		OutputDir: relPath(absRoot, outDir),
		Templates: templates,
		IDLs:      idls,
	}, nil
```

新增函数：

```go
// exportIDLs variabilizes the project's service IDL into template/idl/.
// hz standard support files (openapi/, validate/) stay embedded and are
// excluded. A missing idl/ dir is not an error.
func exportIDLs(root string, opts ExportOptions) ([]string, error) {
	idlRoot := filepath.Join(root, "idl")
	if fi, err := os.Stat(idlRoot); err != nil || !fi.IsDir() {
		return nil, nil
	}
	var exported []string
	err := filepath.Walk(idlRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel := relPath(idlRoot, path)
		if strings.HasPrefix(rel, "openapi/") || strings.HasPrefix(rel, "validate/") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		body := regexp.MustCompile(regexp.QuoteMeta(opts.Module)).ReplaceAllString(string(content), "{{.Module}}")
		body = replaceServiceName(body, opts.ServiceName)
		tplRel := idlTemplatePath(rel, opts)
		out := filepath.Join(root, "template", "idl", filepath.FromSlash(tplRel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
			return err
		}
		exported = append(exported, "idl/"+tplRel)
		return nil
	})
	return exported, err
}

// idlTemplatePath parameterizes the service name inside IDL file names so
// consumers render them onto their own default IDL paths (hertz
// idl/app/<name>.proto, kitex idl/<name>.proto).
func idlTemplatePath(rel string, opts ExportOptions) string {
	dashed := strings.ToLower(opts.ServiceName) // "user-rpc"
	lower := serviceNameLower(opts.ServiceName) // "userrpc"
	out := rel
	if dashed != "" {
		out = strings.ReplaceAll(out, dashed, "{{ToLower .ServiceName}}")
	}
	if lower != "" && lower != dashed {
		out = strings.ReplaceAll(out, lower, "{{ToLower .ServiceName}}")
	}
	return out
}
```

- [ ] **Step 4: CLI 输出 + CLI 测试**

`internal/cli/export.go` `runExportTemplates` 输出改为：

```go
	out := cmd.OutOrStdout()
	if len(result.IDLs) > 0 {
		fmt.Fprintf(out, "exported %d templates and %d IDL files to %s/\n", len(result.Templates), len(result.IDLs), result.OutputDir)
	} else {
		fmt.Fprintf(out, "exported %d templates to %s/\n", len(result.Templates), result.OutputDir)
	}
	for _, t := range result.Templates {
		fmt.Fprintf(out, "  - %s\n", t)
	}
	for _, f := range result.IDLs {
		fmt.Fprintf(out, "  - %s\n", f)
	}
	return nil
```

`internal/cli/export_test.go`：给 `seedExportProject` 的两种 kind 各加一个 IDL fixture（hertz：`idl/app/demo.proto`，内容 `service Demo {}` 最小合法 proto 体；kitex：`idl/demo.proto`），并在 `TestRunExportTemplatesHertz` 断言输出含 `IDl files`→ 用 `strings.Contains(text, "IDL files")`，且 `template/idl/app/{{ToLower .ServiceName}}.proto` 存在。

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/scaffold/template/ ./internal/cli/ -count=1`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/scaffold/template/export.go internal/scaffold/template/export_test.go internal/cli/export.go internal/cli/export_test.go
git commit -m "feat(template): export service IDL into template package"
```

---

### Task 3: 模版包加载（LoadPackage / ReadPackageMeta）

**Files:**
- Create: `internal/scaffold/template/package.go`
- Test: `internal/scaffold/template/package_test.go`

**Interfaces:**
- Consumes: 无
- Produces（T4/T5 依赖的精确签名）：

```go
// PackageMeta is the template.yaml metadata describing a template package.
type PackageMeta struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// Package is a template package loaded from disk.
type Package struct {
	Dir         string   // absolute package root
	Meta        PackageMeta
	HasMeta     bool     // false when template.yaml is absent
	TemplateDir string   // absolute <kind>-template directory
	Templates   []string // absolute .yaml paths under TemplateDir
	IDLDir      string   // absolute idl directory (may not exist)
	IDLs        []string // absolute .proto paths under IDLDir
}

func ReadPackageMeta(dir string) (PackageMeta, error) // template.yaml 缺失 → fs.ErrNotExist 可判
func LoadPackage(dir, kind string) (*Package, error)
```

错误文案（测试断言）：
- 目录不存在：`template package %q does not exist`
- template.yaml 解析失败：`template package: parse %s: %w`
- kind 不匹配：`template package %q has kind %q, want %q`
- 缺 `<kind>-template/`：`template package %q has no %s-template/ directory`
- `<kind>-template/` 无 yaml：`template package %q has no .yaml templates in %s-template/`

- [ ] **Step 1: 写失败测试**

```go
package template

func TestLoadPackageHappy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: d\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "main_go.yaml"), []byte("path: main.go\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "idl"), 0o755)
	os.WriteFile(filepath.Join(dir, "idl", "svc.proto"), []byte("syntax = \"proto3\";\n"), 0o644)

	pkg, err := LoadPackage(dir, "kitex")
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	if !pkg.HasMeta || pkg.Meta.Name != "base-kitex" || pkg.Meta.Kind != "kitex" {
		t.Errorf("meta = %+v", pkg.Meta)
	}
	if len(pkg.Templates) != 1 || len(pkg.IDLs) != 1 {
		t.Errorf("templates=%v idls=%v", pkg.Templates, pkg.IDLs)
	}
}

func TestLoadPackageNoMeta(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "hertz-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "hertz-template", "main_go.yaml"), []byte("path: main.go\n"), 0o644)
	pkg, err := LoadPackage(dir, "hertz")
	if err != nil || pkg.HasMeta {
		t.Fatalf("expected no-meta success, got %+v, %v", pkg, err)
	}
}

func TestLoadPackageKindMismatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"), []byte("kind: hertz\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "a.yaml"), []byte("path: main.go\n"), 0o644)
	_, err := LoadPackage(dir, "kitex")
	if err == nil || !strings.Contains(err.Error(), "has kind") {
		t.Errorf("want kind mismatch error, got %v", err)
	}
}

func TestLoadPackageParseError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"), []byte("{{invalid"), 0o644)
	_, err := LoadPackage(dir, "kitex")
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("want parse error, got %v", err)
	}
}

func TestLoadPackageMissingTemplateDir(t *testing.T) {
	_, err := LoadPackage(t.TempDir(), "kitex")
	if err == nil || !strings.Contains(err.Error(), "has no kitex-template/ directory") {
		t.Errorf("want missing dir error, got %v", err)
	}
}

func TestReadPackageMetaMissing(t *testing.T) {
	_, err := ReadPackageMeta(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want fs.ErrNotExist, got %v", err)
	}
}
```

（import：`errors`、`io/fs`、`os`、`path/filepath`、`strings`、`testing`。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/template/ -run 'TestLoadPackage|TestReadPackageMeta' -count=1`
Expected: FAIL（编译失败：符号不存在）

- [ ] **Step 3: 实现 `package.go`**

```go
package template

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadPackageMeta reads <dir>/template.yaml. A missing file returns an error
// satisfying fs.ErrNotExist; callers listing registries skip such dirs.
func ReadPackageMeta(dir string) (PackageMeta, error) {
	path := filepath.Join(dir, "template.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return PackageMeta{}, err
	}
	var m PackageMeta
	if err := yaml.Unmarshal(b, &m); err != nil {
		return PackageMeta{}, fmt.Errorf("template package: parse %s: %w", path, err)
	}
	return m, nil
}

// LoadPackage loads a template package rooted at dir for the given kind.
// template.yaml is optional (HasMeta=false); when present its kind must
// match. The package must contain at least one .yaml in <kind>-template/.
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
		if meta.Kind != "" && meta.Kind != kind {
			return nil, fmt.Errorf("template package %q has kind %q, want %q", dir, meta.Kind, kind)
		}
	case errors.Is(err, fs.ErrNotExist):
		// optional metadata
	default:
		return nil, err
	}
	pkg.TemplateDir = filepath.Join(abs, kind+"-template")
	entries, err := os.ReadDir(pkg.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("template package %q has no %s-template/ directory", dir, kind)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		pkg.Templates = append(pkg.Templates, filepath.Join(pkg.TemplateDir, e.Name()))
	}
	if len(pkg.Templates) == 0 {
		return nil, fmt.Errorf("template package %q has no .yaml templates in %s-template/", dir, kind)
	}
	pkg.IDLDir = filepath.Join(abs, "idl")
	if fi, err := os.Stat(pkg.IDLDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(pkg.IDLDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".proto") {
				pkg.IDLs = append(pkg.IDLs, path)
			}
			return nil
		})
	}
	return pkg, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/scaffold/template/ -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/template/package.go internal/scaffold/template/package_test.go
git commit -m "feat(template): add template package loader"
```

---

### Task 4: mono 消费模版包 + `ncgo new --template-dir`

**Files:**
- Modify: `internal/scaffold/mono/mono.go`（`Options` + `Result` + `Generate` 流程 + `validate`）
- Modify: `internal/scaffold/mono/files.go`（新增 `overlayTemplatePackage`）
- Modify: `internal/cli/root.go`（`newOptions` + flags + `runNewMono`）
- Test: `internal/scaffold/mono/template_package_test.go`（新文件）、`internal/cli/new_test.go`（追加；若无此文件则建 `root_test.go` 追加）

**Interfaces:**
- Consumes: T3 `template.LoadPackage`、T1/T2 导出产物形态（`<pkg>/<kind>-template/*.yaml`、`<pkg>/idl/**`，IDL 文件名含 `{{ToLower .ServiceName}}`）
- Produces:
  - `mono.Options` 新增 `TemplateDir string`；`mono.Result` 新增 `TemplateIDLFallback bool`
  - `mono.Generate` 在 `writeTemplate` 之后、`writeIDLPlaceholder` 之前调用 `overlayTemplatePackage(dir, opts)`（仅当 TemplateDir != ""）
  - CLI flags：`--template-dir`（string，mono）；互斥校验错误文案：`scaffold: --template-dir and --preset are mutually exclusive`（mono.validate）、`--template and --template-dir are mutually exclusive`（runNewMono，T5 引入 --template 后补上）

**行为细则：**
- overlay 先 `os.RemoveAll(<dir>/template/<kind>-template)` 再从包内复制 yaml → **替换**而非合并内置模板
- 包内 IDL：每个 proto 用 `template.Render(body, RenderData{Module: opts.Module, ServiceName: opts.Name})` 渲染 body；**路径替换 token 按 kind 区分**（与 `defaultIDL` 对齐）：hertz → `strings.ToLower(opts.Name)`；kitex → `kitexIDLBase(opts)`（即 `ToLower(exportName(name))`，去横线）。写入 `<dir>/idl/<rel>`；随后 `writeIDLPlaceholder` 因"不覆盖已存在 IDL"自然跳过主 proto（hertz 支持文件 openapi/validate 仍由占位流程写入）
- 包无 `idl/`（或无 proto）→ 返回 fallback=true，`Result.TemplateIDLFallback=true`，CLI 打印 `(template package has no idl/; used built-in IDL placeholder)`（向后兼容评审 AC）

- [ ] **Step 1: 写失败测试（mono 包）**

`template_package_test.go`（复用 mono_test.go 既有 `fakeRunner`）：

```go
package mono

// seedTemplatePackage builds a minimal kitex template package.
func seedTemplatePackage(t *testing.T, withIDL bool) string {
	t.Helper()
	pkg := t.TempDir()
	os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(pkg, "kitex-template", "main_go.yaml"), []byte(
		"path: main.go\nupdate_behavior:\n  type: cover\nbody: |\n  package main\n"), 0o644)
	if withIDL {
		os.MkdirAll(filepath.Join(pkg, "idl"), 0o755)
		os.WriteFile(filepath.Join(pkg, "idl", "{{ToLower .ServiceName}}.proto"),
			[]byte("syntax = \"proto3\";\nservice {{.ServiceName}} {}\n"), 0o644)
	}
	return pkg
}

func TestGenerateTemplatePackageKitex(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "svc")
	res, err := Generate(context.Background(), Options{
		Name: "demo", Module: "github.com/acme/demo", Kind: manifest.KindKitex,
		Dir: dir, TemplateDir: seedTemplatePackage(t, true),
		NoGenerate: true, Runner: &fakeRunner{},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.TemplateIDLFallback {
		t.Error("unexpected IDL fallback")
	}
	// package yaml replaced embedded set: exactly the package's file remains
	entries, _ := os.ReadDir(filepath.Join(dir, "template", "kitex-template"))
	if len(entries) != 1 || entries[0].Name() != "main_go.yaml" {
		t.Errorf("template dir = %v", entries)
	}
	// IDL rendered onto the kitex default path with the target service name
	body, err := os.ReadFile(filepath.Join(dir, "idl", "demo.proto"))
	if err != nil {
		t.Fatalf("rendered idl missing: %v", err)
	}
	if !strings.Contains(string(body), "service demo {}") {
		t.Errorf("service name not rendered:\n%s", body)
	}
}

func TestGenerateTemplatePackageIDLFallback(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "svc")
	res, err := Generate(context.Background(), Options{
		Name: "demo", Module: "github.com/acme/demo", Kind: manifest.KindKitex,
		Dir: dir, TemplateDir: seedTemplatePackage(t, false),
		NoGenerate: true, Runner: &fakeRunner{},
	})
	if err != nil || !res.TemplateIDLFallback {
		t.Fatalf("want fallback, got %+v, %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "idl", "demo.proto")); err != nil {
		t.Errorf("built-in placeholder should apply: %v", err)
	}
}

func TestGenerateTemplatePackagePresetConflict(t *testing.T) {
	_, err := Generate(context.Background(), Options{
		Name: "demo", Module: "github.com/acme/demo", Kind: manifest.KindKitex,
		Dir: filepath.Join(t.TempDir(), "svc"), Preset: "rule-center",
		TemplateDir: t.TempDir(), NoGenerate: true, Runner: &fakeRunner{},
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("want mutual exclusion error, got %v", err)
	}
}
```

另补 kind 不匹配与 template.yaml 解析失败两个用例（断言错误文案 `has kind` / `parse`，fixture 同 Task 3 测试）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/scaffold/mono/ -run TestGenerateTemplatePackage -count=1`
Expected: FAIL（编译失败：Options.TemplateDir 不存在）

- [ ] **Step 3: 实现 mono 侧**

`mono.go`：Options 加 `TemplateDir string // external template package dir; replaces embedded <kind>-template and IDL placeholder`；Result 加 `TemplateIDLFallback bool`。`validate()` 加互斥检查。`Generate()` 在 `writeTemplate` 与 `writeIDLPlaceholder` 之间：

```go
	templateIDLFallback := false
	if opts.TemplateDir != "" {
		fallback, err := overlayTemplatePackage(dir, opts)
		if err != nil {
			return nil, err
		}
		templateIDLFallback = fallback
	}
```

并在构造 `res` 处赋 `TemplateIDLFallback: templateIDLFallback`（注意 `res` 在 NoGenerate 提前返回路径也要带上该字段——把 `res := &Result{...}` 之前声明 fallback 变量即可覆盖两条路径）。

`files.go` 新增（签名与行为见上；完整代码在 Interfaces/行为细则已给出，实现时按此落码）：

```go
// overlayTemplatePackage replaces the embedded <kind>-template YAML and the
// IDL placeholder with the contents of an external template package. It
// reports true when the package carries no IDL and the built-in placeholder
// still applies (backward compatibility with pre-IDL exports).
func overlayTemplatePackage(dir string, opts Options) (bool, error) { ... }
```

- [ ] **Step 4: CLI `--template-dir`**

`root.go`：`newOptions` 加 `templateDir string`；`newNewCmd` 加

```go
f.StringVar(&opts.templateDir, "template-dir", "", "Mono template package directory replacing embedded code templates and the IDL placeholder")
```

`runNewMono` 构造 `mono.Options{...}` 时加 `TemplateDir: opts.templateDir,`。

CLI 测试（追加到 root_test.go）：`ncgo new` 帮助含 `--template-dir`（执行命令 `--help` 捕获输出或直接断言 flag 存在）；mono.Generate 互斥错误经 runNewMono 透传（可选，mono 测试已覆盖）。

- [ ] **Step 5: 运行确认通过（含既有 golden 不变）**

Run: `go test ./internal/scaffold/mono/ ./internal/cli/ -count=1`
Expected: PASS；**特别确认 `TestGenerateGolden*` 4 项未触发更新即通过**（缺省路径向后兼容 AC）

- [ ] **Step 6: 提交**

```bash
git add internal/scaffold/mono/ internal/cli/root.go internal/cli/root_test.go
git commit -m "feat(mono): consume external template package via --template-dir"
```

---

### Task 5: registry 客户端 + `ncgo template list/pull` + `ncgo new --template`

**Files:**
- Create: `internal/registry/registry.go`
- Test: `internal/registry/registry_test.go`
- Create: `internal/cli/template.go`
- Modify: `internal/cli/root.go`（注册 template 命令 + `--template` flag + 解析逻辑）
- Test: `internal/cli/template_test.go`

**Interfaces:**
- Consumes: T3 `template.ReadPackageMeta`；`internal/exec.Runner`
- Produces（T6 MCP 依赖）：

```go
package registry

const (
	DefaultURL  = "https://github.com/byx-darwin/ncgo-templates.git"
	EnvOverride = "NCGO_REGISTRY"
)

func ResolveURL(flagValue string) string          // flag > env > DefaultURL
func CacheDir() (string, error)                   // os.UserCacheDir()/ncgo/template-registry

type Entry struct{ Name, Kind, Description string }

type Client struct {
	URL    string
	Runner ncgoexec.Runner // nil → exec.NewDefault()
	Root   string          // cache root override; empty → CacheDir()
}

func NewClient(url string, runner ncgoexec.Runner) *Client
func (c *Client) LocalPath(name string) string                    // <root>/<name>，不做存在性检查、不触网
func (c *Client) List(ctx context.Context) ([]Entry, error)       // ensure cache → 扫描根目录含 template.yaml 的子目录（按 Name 排序）
func (c *Client) Pull(ctx context.Context, name string) (string, error) // ensure cache → <root>/<name>/template.yaml 必须存在 → 返回该目录
```

错误文案：git 失败 → `registry unavailable (<url>): %v`；git 不在 PATH（`*exec.NotFoundError`）→ `git is required for registry access`；Pull 未找到 → `template %q not found in registry %s (run: ncgo template list)`。

**实现要点**：`ensureCache`：`<root>/.git` 存在 → `git pull --ff-only`（Dir=root）；否则 `os.MkdirAll(filepath.Dir(root))` + `git clone --depth 1 <url> <root>`。Runner 为 nil 时用 `exec.NewDefault()`。

- [ ] **Step 1: 写失败测试（registry 包）**

```go
package registry

// fakeGit simulates the git invocations used by ensureCache.
type fakeGit struct {
	fixture  string // directory copied on "clone"
	cloneErr error
	pulls    int
}

func (f *fakeGit) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	if c.Name != "git" {
		return exec.Result{}, fmt.Errorf("unexpected command %q", c.Name)
	}
	switch c.Args[0] {
	case "clone":
		if f.cloneErr != nil {
			return exec.Result{}, f.cloneErr
		}
		dst := c.Args[len(c.Args)-1]
		if err := copyDir(f.fixture, dst); err != nil { // copyDir：递归复制（测试内实现）
			return exec.Result{}, err
		}
		return exec.Result{}, os.MkdirAll(filepath.Join(dst, ".git"), 0o755)
	case "pull":
		f.pulls++
		return exec.Result{}, nil
	}
	return exec.Result{}, fmt.Errorf("unexpected git args %v", c.Args)
}

func TestClientListFixture(t *testing.T) {
	fixture := t.TempDir()
	os.MkdirAll(filepath.Join(fixture, "base-kitex"), 0o755)
	os.WriteFile(filepath.Join(fixture, "base-kitex", "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: base\n"), 0o644)
	os.MkdirAll(filepath.Join(fixture, "docs"), 0o755) // noise: no template.yaml

	c := NewClient("https://example.invalid/registry.git", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	entries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "base-kitex" || entries[0].Kind != "kitex" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestClientPullNotFound(t *testing.T) {
	fixture := t.TempDir() // empty registry
	c := NewClient("u", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	_, err := c.Pull(context.Background(), "nope")
	if err == nil || !strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("want not-found error, got %v", err)
	}
}

func TestClientRegistryUnavailable(t *testing.T) {
	c := NewClient("u", &fakeGit{cloneErr: errors.New("dial tcp: timeout")})
	c.Root = filepath.Join(t.TempDir(), "cache")
	_, err := c.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Errorf("want registry unavailable, got %v", err)
	}
}

func TestClientGitMissing(t *testing.T) {
	c := NewClient("u", runnerFunc(func() (exec.Result, error) {
		return exec.Result{}, &exec.NotFoundError{Name: "git"}
	}))
	c.Root = filepath.Join(t.TempDir(), "cache")
	_, err := c.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "git is required") {
		t.Errorf("want git-required error, got %v", err)
	}
}

func TestResolveURLPrecedence(t *testing.T) {
	if got := ResolveURL("https://flag"); got != "https://flag" {
		t.Errorf("flag: %s", got)
	}
	t.Setenv(EnvOverride, "https://env")
	if got := ResolveURL(""); got != "https://env" {
		t.Errorf("env: %s", got)
	}
	os.Unsetenv(EnvOverride)
	if got := ResolveURL(""); got != DefaultURL {
		t.Errorf("default: %s", got)
	}
}
```

（`runnerFunc` 为测试内的 `exec.Runner` 适配器：`type runnerFunc func() (exec.Result, error)` + `Run` 方法；`copyDir` 为测试内递归复制 helper。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/registry/ -count=1`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现 `internal/registry/registry.go`**（按 Interfaces 签名与实现要点落码）

- [ ] **Step 4: CLI `template` 子命令**

`internal/cli/template.go`：

```go
package cli

func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "template", Short: "Manage official template registry packages"}
	cmd.AddCommand(newTemplateListCmd(), newTemplatePullCmd())
	return cmd
}
```

- `template list [--registry <url>]` → `runTemplateList(cmd, opts, client *registry.Client)`（client 为 nil 时按 `registry.ResolveURL(opts.registry)` + `exec.NewDefault()` 构造；测试注入 fake-Runner 的 client）。输出每行 `%s\t%s\t%s`（name/kind/description）；空列表输出 `no templates in registry`。
- `template pull <name> [--registry <url>]` → `runTemplatePull(cmd, name, opts, client)`。成功输出 `pulled %s -> %s`。

`root.go`：`cmd.AddCommand(newTemplateCmd())`。

CLI 测试：fakeGit 同包内复用（或 registry 测试已覆盖核心，CLI 层用注入 client 验证输出文本与错误透传）。

- [ ] **Step 5: `ncgo new --template`**

`newOptions` 加 `templateName string`；flag：

```go
f.StringVar(&opts.templateName, "template", "", "Mono template name from the registry cache (run `ncgo template pull` first)")
```

`runNewMono` 在构造 mono.Options 前：

```go
	templateDir := opts.templateDir
	if opts.templateName != "" {
		if templateDir != "" {
			return errors.New("--template and --template-dir are mutually exclusive")
		}
		client := registry.NewClient(registry.ResolveURL(""), goexec.NewDefault())
		dir := client.LocalPath(opts.templateName)
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return fmt.Errorf("template %q not in cache (%s); run: ncgo template pull %s", opts.templateName, dir, opts.templateName)
		}
		templateDir = dir
	}
```

（`goexec` 为 root.go 既有 `internal/exec` import 别名；`registry` 为新增 import。）

CLI 测试：`--template` 缓存缺失时报错文案含 `ncgo template pull`。

- [ ] **Step 6: 运行确认通过**

Run: `go test ./internal/registry/ ./internal/cli/ -count=1`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/registry/ internal/cli/template.go internal/cli/template_test.go internal/cli/root.go internal/cli/root_test.go
git commit -m "feat(cli): add template registry client (template list/pull, new --template)"
```

---

### Task 6: MCP 工具 + export 描述修正

**Files:**
- Modify: `internal/mcp/tools.go`（工具清单 + switch 分发）
- Create: `internal/mcp/tool_template.go`
- Modify: `internal/mcp/tool_export_templates.go`（结构化字段加 `idls`；文本含 IDL 计数）
- Test: `internal/mcp/tool_template_test.go`、既有 `server_test.go`/`server_new_tools_test.go` 相关用例同步

**Interfaces:**
- Consumes: T5 `registry.NewClient/ResolveURL`、既有 `resolveMCPOutput/formatMCPOutput/textResult/schemaObject/stringField/outputTextJSONField`
- Produces:
  - `ncgo_export_templates` 描述修正为 `Export code templates from an existing ncgo project to template/<kind>-template/.`；结构化字段新增 `idls []string`（空时为 `[]`）
  - `ncgo_template_list`：InputSchema fields `registry`(optional)+`output`；顶层字段 `templates: [{name,kind,description}]`
  - `ncgo_template_pull`：InputSchema required `name`，fields `registry`(optional)+`output`；顶层字段 `name`、`dir`
  - 错误路径：registry 不可用 / 模版不存在 → `textResult(err.Error(), true)`（isError=true，不抛 JSON-RPC error）

- [ ] **Step 1: 写失败测试**

```go
package mcp

// TestCallTemplateListFixture: 本地 git fixture 仓库（t.TempDir + template.yaml；
//   若 exec.LookPath("git") 失败则 t.Skip）→ callTemplateList(`{"registry":"<path>"}`)
//   → isError=false；顶层 templates 含 fixture 模版；content[0].text 含模版名
// TestCallTemplateListRegistryUnavailable: registry 指向不存在路径且缓存目录不可 clone
//   → isError=true，文本含 "registry unavailable"
// TestCallTemplatePullMissing: fixture 仓库无该模版 → isError=true，文本含 "not found in registry"
// TestExportTemplatesDescription: tools/list 中 ncgo_export_templates 描述不含 "embedded scaffold"
```

- [ ] **Step 2: 运行确认失败** → **Step 3: 实现**

`tools.go` 清单追加两项（描述/Schema 见 Interfaces），switch 加 `case "ncgo_template_list"/"ncgo_template_pull"`。`tool_template.go` 按 tool_export_templates.go 既有模式实现（`resolveMCPOutput("template_list", args.Output, mcpOutputText, mcpOutputJSON)` + `formatMCPOutput` + 顶层字段注入）。`tool_export_templates.go` fields 加 `"idls": idls`（nil→`[]string{}`），text 分支按 T2 Step4 同款条件句式。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/mcp/ -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/mcp/
git commit -m "feat(mcp): add ncgo_template_list/pull tools; fix export_templates description"
```

---

### Task 7: golden 快照 + smoke 闭环场景

**Files:**
- Create: `internal/scaffold/template/golden_test.go` + `testdata/golden/export-hertz/`、`testdata/golden/export-kitex/` 快照
- Modify: `scripts/smoke.sh`（在 `log "smoke OK"` 之前追加场景）

**Interfaces:**
- Consumes: T1/T2 的导出行为；T4 的 `ncgo new --template-dir`
- Produces: export 输出快照锁定；smoke 端到端验证 export→new 闭环（`--no-generate`，不依赖本机 hz/kitex）

- [ ] **Step 1: golden 测试**

```go
package template

import (
	"path/filepath"
	"testing"

	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

func TestExportGoldenHertz(t *testing.T) {
	root := t.TempDir()
	// 固定 fixture（内容必须逐字节确定）：
	//   main.go（import handler 路径，含小写服务名）
	//   conf/dev/conf.yaml、internal/base/conf/conf.go、internal/base/server/server.go
	//   internal/handler/userapi/handler.go（package userapi + UserApiImpl 方法）
	//   idl/app/userapi.proto（service UserApi + go_package）
	if _, err := Export(ExportOptions{Root: root, Kind: "hertz",
		Module: "github.com/acme/golden", ServiceName: "UserApi"}); err != nil {
		t.Fatalf("export: %v", err)
	}
	golden.Tree(t, filepath.Join("golden", "export-hertz"), filepath.Join(root, "template"))
}
```

Kitex 同款（`export-kitex`，fixture 含 middleware/release 目录）。生成快照：`go test ./internal/scaffold/template/ -run TestExportGolden -update-golden`，然后**人工审阅快照**确认 `{{.Module}}`/`{{.ServiceName}}`/`{{ToLower .ServiceName}}` 替换符合预期（尤其 import 路径与 IDL 文件名）。

- [ ] **Step 2: 快照断言通过**

Run: `go test ./internal/scaffold/template/ -run TestExportGolden -count=1`
Expected: PASS

- [ ] **Step 3: smoke 场景**

`scripts/smoke.sh` 在 `log "smoke OK"` 前插入：

```bash
log "export templates -> new --template-dir closed loop"
EXPORT_SRC="$TMP_DIR/export-src"
write_manifest "$EXPORT_SRC/.ncgo/manifest.yaml" github.com/acme/exportsrc hertz exportsrc
mkdir -p "$EXPORT_SRC/internal/handler/exportsrc" "$EXPORT_SRC/idl/app"
printf 'package main\n' >"$EXPORT_SRC/main.go"
printf 'package exportsrc\n' >"$EXPORT_SRC/internal/handler/exportsrc/handler.go"
# proto service 名必须等于 exportName(manifest name)=Exportsrc，导出才会变量化
cat >"$EXPORT_SRC/idl/app/exportsrc.proto" <<'PROTO'
syntax = "proto3";
package app;
service Exportsrc {}
PROTO
"$BIN" export templates --root "$EXPORT_SRC" >"$TMP_DIR/export.out"
grep -q 'exported ' "$TMP_DIR/export.out"
grep -q 'IDL files' "$TMP_DIR/export.out"
test -f "$EXPORT_SRC/template/idl/app/"'{{ToLower .ServiceName}}'".proto"
"$BIN" new exporttgt --module github.com/acme/exporttgt --kind hertz --no-generate \
  --dir "$TMP_DIR/export-tgt" --template-dir "$EXPORT_SRC/template" >"$TMP_DIR/new.out"
grep -q 'scaffolded exporttgt' "$TMP_DIR/new.out"
test -f "$TMP_DIR/export-tgt/idl/app/exporttgt.proto"
grep -q 'service exporttgt' "$TMP_DIR/export-tgt/idl/app/exporttgt.proto"
# 渲染后的 proto 必须仍是合法 proto（风险缓解：变量化不破坏语法）
"$BIN" protolint --root "$TMP_DIR/export-tgt" --files idl/app/exporttgt.proto >"$TMP_DIR/protolint.out" 2>&1 || {
  cat "$TMP_DIR/protolint.out" >&2; exit 1; }
grep -q 'template package has no idl' "$TMP_DIR/new.out" && exit 1 || true
```

（protolint 若有风格告警属正常退出码语义需按实际确认：若 `ncgo protolint` 对风格问题返回非零，则改为仅断言不含 `parse` 错误。验证时以实际行为为准。）

- [ ] **Step 4: 运行 smoke**

Run: `./scripts/smoke.sh`
Expected: 全程 PASS

- [ ] **Step 5: 提交**

```bash
git add internal/scaffold/template/golden_test.go internal/scaffold/template/testdata/ scripts/smoke.sh
git commit -m "test(template): golden snapshots for export; smoke export->new closed loop"
```

---

### Task 8: 文档同步（EN/ZH 对齐）

**Files:**
- Modify: `README.md`、`README.zh-CN.md`（命令表）
- Modify: `docs/examples.md`、`docs/examples.zh-CN.md`（export 章节扩充 + registry 新章节）

**Interfaces:**
- Consumes: T2/T4/T5/T6 的最终 CLI/MCP 形态
- Produces: 与代码一致的 EN/ZH 文档

- [ ] **Step 1: README 命令表**

EN `README.md` 命令表追加（紧邻 `ncgo export templates` 行）：

```markdown
| `ncgo template list` | List template packages in the official template registry |
| `ncgo template pull <name>` | Pull a template package from the registry into the local cache |
```

并在 `ncgo new` 行/flags 说明中补 `--template-dir <dir>` / `--template <name>`（与 `--preset` 互斥）。ZH `README.zh-CN.md` 对应中文行：

```markdown
| `ncgo template list` | 列出官方模版 registry 中的模版包 |
| `ncgo template pull <name>` | 从 registry 拉取模版包到本地缓存 |
```

- [ ] **Step 2: examples 扩充 §6（Export）**

EN `docs/examples.md` §6 追加：导出产物包含 `template/idl/`（服务名变量化）；换名复制语义说明（模板内容即基础项目起始代码，生成只替换 module/服务名）。ZH 同步。

- [ ] **Step 3: examples 新章节——Template Registry**

EN 新增 `## N. Generate base projects from official templates`：

````markdown
Once a template is reviewed and merged into the official registry:

```bash
ncgo template list                 # browse official templates
ncgo template pull base-kitex      # cache locally
ncgo new my-svc --module github.com/acme/my-svc --kind kitex --template base-kitex
```

Or point at any local template package (the layout `ncgo export templates` produces):

```bash
ncgo new my-svc --module github.com/acme/my-svc --kind kitex \
  --template-dir path/to/base-kitex
```

A template package is a directory with `<kind>-template/*.yaml`, optionally
`idl/*.proto` and a `template.yaml` metadata file (`name`, `kind`,
`description`, `version`). To contribute one: export from a mature project,
add `template.yaml` + `README.md`, open a PR against the registry repository
for official review.

The registry URL defaults to the official repository; override with
`--registry <url>` or `NCGO_REGISTRY`.
````

ZH `docs/examples.zh-CN.md` 同内容中文版本（相同代码块）。

- [ ] **Step 4: markdown 诊断**

Run: 编辑器/既有 markdown lint（如无工具则 `grep -rn 'TODO\|TBD' README*.md docs/examples*.md` 确无残留）
Expected: 无错误

- [ ] **Step 5: 提交**

```bash
git add README.md README.zh-CN.md docs/examples.md docs/examples.zh-CN.md
git commit -m "docs: template registry closed loop (README + examples, EN/ZH)"
```

---

### Task 9: 全量验证

**Files:** 无新增

- [ ] **Step 1: 全量检查**

Run:

```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go build ./... && go build .
go vet ./...
go test ./... -count=1
./scripts/smoke.sh
```

Expected: gofmt 无输出；build/vet/test/smoke 全部 PASS；mono golden 未更新（`git status` 确认 testdata 无改动，除非任务内有意变更 template 包快照）。

- [ ] **Step 2: 验收清单核对（Issue #54 AC + 评审补充）**

逐条核对 Issue #54 的 8 条 AC 与评审 comment 的错误路径/向后兼容要求；未过项回到对应任务修复后重跑本任务。

- [ ] **Step 3: 最终提交（如有收尾改动）**

```bash
git add -A && git commit -m "chore: final verification fixes for template registry loop"
```
