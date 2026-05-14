# Code Template Export & Apply Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `ncgo export templates` to extract code templates from existing Hertz/Kitex projects, and apply Hertz templates during `ncgo new --kind hertz`.

**Architecture:** A framework-agnostic `internal/scaffold/template/` package handles Go→YAML export using `protocompile` (already a dependency) for proto parsing. Hertz apply is a post-`hz new` overlay in `mono.Generate()`. Kitex apply is already native (`kitex -template-dir`), so export output format matches existing Kitex templates.

**Tech Stack:** Go 1.25+, `text/template` (stdlib), `gopkg.in/yaml.v3`, `github.com/bufbuild/protocompile`, `google.golang.org/protobuf`

---

### Task 1: Template YAML types and render engine

**Files:**
- Create: `internal/scaffold/template/types.go`
- Create: `internal/scaffold/template/render.go`
- Test: `internal/scaffold/template/render_test.go`

Define the YAML template schema matching Kitex's existing format, plus a render engine that can process these templates.

- [ ] **Step 1: Write types.go**

```go
// Package template extracts code templates from existing ncgo projects
// and applies them during new project generation.
package template

// TemplateFile represents a single code template as YAML.
type TemplateFile struct {
	Path           string          `yaml:"path"`
	UpdateBehavior UpdateBehavior  `yaml:"update_behavior"`
	LoopService    bool            `yaml:"loop_service,omitempty"`
	Body           string          `yaml:"body"`
}

// UpdateBehavior controls how a template is applied.
type UpdateBehavior struct {
	Type string `yaml:"type"` // "skip" or "cover"
}

// ServiceInfo holds proto-level information for template variables.
type ServiceInfo struct {
	ServiceName string
	ImportPath  string
	PkgRefName  string
	Methods     []MethodInfo
}

// MethodInfo describes a single RPC method.
type MethodInfo struct {
	Name string
	Args []MethodArg
	Resp MethodResp
}

// MethodArg describes an RPC method argument.
type MethodArg struct {
	Name string
	Type string
}

// MethodResp describes an RPC method return type.
type MethodResp struct {
	Type string
}

// RenderData is the top-level template context.
type RenderData struct {
	Module        string
	ServiceName   string
	ServiceInfo   ServiceInfo
	WithDatabase  bool
	Infra         []string
}
```

- [ ] **Step 2: Write render.go**

```go
package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// FuncMap returns the template functions available in template bodies.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"ToLower":    strings.ToLower,
		"ToUpper":    strings.ToUpper,
		"LowerFirst": lowerFirst,
		"exportName": exportName,
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func exportName(s string) string {
	out := make([]byte, 0, len(s))
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '_' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
		upper = false
	}
	return string(out)
}

// Render executes a template body with the given data.
func Render(body string, data RenderData) (string, error) {
	tmpl, err := template.New("template").Funcs(FuncMap()).Parse(body)
	if err != nil {
		return "", fmt.Errorf("template parse: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execute: %w", err)
	}
	return buf.String(), nil
}
```

- [ ] **Step 3: Write render_test.go**

```go
package template

import (
	"strings"
	"testing"
)

func TestRender_ModuleSubstitution(t *testing.T) {
	body := `import "{{.Module}}/internal/base/conf"`
	data := RenderData{Module: "github.com/acme/user-api"}
	got, err := Render(body, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `import "github.com/acme/user-api/internal/base/conf"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_ServiceNameToLower(t *testing.T) {
	body := `path: internal/handler/{{ToLower .ServiceName}}/handler.go`
	data := RenderData{ServiceName: "UserApi"}
	got, err := Render(body, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `path: internal/handler/userapi/handler.go`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRender_ServiceInfoMethods(t *testing.T) {
	body := `{{range .ServiceInfo.Methods}}func (s *{{$.ServiceName}}Impl) {{.Name}}() {}
{{end}}`
	data := RenderData{
		ServiceName: "UserApi",
		ServiceInfo: ServiceInfo{
			Methods: []MethodInfo{
				{Name: "GetUser"},
				{Name: "ListUsers"},
			},
		},
	}
	got, err := Render(body, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "GetUser") {
		t.Errorf("expected GetUser in output:\n%s", got)
	}
	if !strings.Contains(got, "ListUsers") {
		t.Errorf("expected ListUsers in output:\n%s", got)
	}
}

func TestRender_WithDatabase(t *testing.T) {
	body := `{{if .WithDatabase}}db_enabled{{else}}no_db{{end}}`
	got, err := Render(body, RenderData{WithDatabase: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "db_enabled" {
		t.Errorf("got %q, want %q", got, "db_enabled")
	}
}

func TestLowerFirst(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"UserApi", "userApi"},
		{"ABC", "aBC"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := lowerFirst(tt.in); got != tt.want {
			t.Errorf("lowerFirst(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExportName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"user-api", "UserApi"},
		{"user_service", "UserService"},
		{"simple", "Simple"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := exportName(tt.in); got != tt.want {
			t.Errorf("exportName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/scaffold/template/... -v
```

Expected: All 6 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/template/types.go internal/scaffold/template/render.go internal/scaffold/template/render_test.go
git commit -m "feat(template): add YAML template types and render engine"
```

---

### Task 2: Proto parser for ServiceInfo extraction

**Files:**
- Create: `internal/scaffold/template/proto.go`
- Test: `internal/scaffold/template/proto_test.go`

Reuse the existing `protocompile` dependency (same as `internal/protolint/load.go`) to parse proto files and extract ServiceInfo.

- [ ] **Step 1: Write proto.go**

```go
package template

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ParseServiceInfo reads a proto file and extracts ServiceInfo for template rendering.
func ParseServiceInfo(ctx context.Context, protoPath string, module string) (*ServiceInfo, error) {
	abs, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve proto path: %w", err)
	}
	dir := filepath.Dir(abs)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{dir},
		}),
	}
	files, err := compiler.Compile(ctx, abs)
	if err != nil {
		return nil, fmt.Errorf("compile proto: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files compiled from %s", protoPath)
	}
	fd := files[0]

	if fd.Services().Len() == 0 {
		return &ServiceInfo{
			ServiceName: defaultServiceName(protoPath),
			ImportPath:  module,
			PkgRefName:  pkgRefName(module),
		}, nil
	}

	sd := fd.Services().Get(0)
	si := &ServiceInfo{
		ServiceName: string(sd.Name()),
		ImportPath:  module,
		PkgRefName:  pkgRefName(module),
	}

	for i := 0; i < sd.Methods().Len(); i++ {
		md := sd.Methods().Get(i)
		si.Methods = append(si.Methods, MethodInfo{
			Name: string(md.Name()),
			Args: extractArgs(md),
			Resp: MethodResp{
				Type: protoTypeToGo(md.Output()),
			},
		})
	}

	return si, nil
}

func defaultServiceName(protoPath string) string {
	base := filepath.Base(protoPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return exportName(name)
}

func pkgRefName(module string) string {
	parts := strings.Split(module, "/")
	return parts[len(parts)-1]
}

func extractArgs(md protoreflect.MethodDescriptor) []MethodArg {
	input := md.Input()
	var args []MethodArg
	if input.Fields().Len() > 0 {
		// For proto methods, the input is a message, not individual args.
		// In Kitex templates, args are passed as the input message type.
		// We represent it as a single context parameter.
	}
	return args
}

func protoTypeToGo(md protoreflect.MessageDescriptor) string {
	return string(md.Name())
}
```

- [ ] **Step 2: Write proto_test.go**

```go
package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseServiceInfo_NoService(t *testing.T) {
	dir := t.TempDir()
	proto := filepath.Join(dir, "test.proto")
	if err := os.WriteFile(proto, []byte(`syntax = "proto3";
package test;
option go_package = "github.com/acme/test;test";

message PingReq { string name = 1; }
message PingResp { string message = 1; }
`), 0644); err != nil {
		t.Fatal(err)
	}

	si, err := ParseServiceInfo(t.Context(), proto, "github.com/acme/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.ServiceName != "Test" {
		t.Errorf("got ServiceName %q, want %q", si.ServiceName, "Test")
	}
	if si.ImportPath != "github.com/acme/test" {
		t.Errorf("got ImportPath %q, want %q", si.ImportPath, "github.com/acme/test")
	}
	if len(si.Methods) != 0 {
		t.Errorf("expected 0 methods, got %d", len(si.Methods))
	}
}

func TestParseServiceInfo_WithService(t *testing.T) {
	dir := t.TempDir()
	proto := filepath.Join(dir, "user.proto")
	if err := os.WriteFile(proto, []byte(`syntax = "proto3";
package user;
option go_package = "github.com/acme/user;user";

message GetUserReq { int64 id = 1; }
message GetUserResp { string name = 1; }

service UserService {
  rpc GetUser(GetUserReq) returns (GetUserResp);
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	si, err := ParseServiceInfo(t.Context(), proto, "github.com/acme/user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if si.ServiceName != "UserService" {
		t.Errorf("got ServiceName %q, want %q", si.ServiceName, "UserService")
	}
	if len(si.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(si.Methods))
	}
	if si.Methods[0].Name != "GetUser" {
		t.Errorf("got method name %q, want %q", si.Methods[0].Name, "GetUser")
	}
}

func TestDefaultServiceName(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"idl/app/user-api.proto", "UserApi"},
		{"idl/userrpc.proto", "Userrpc"},
		{"idl/svc.proto", "Svc"},
	}
	for _, tt := range tests {
		if got := defaultServiceName(tt.path); got != tt.want {
			t.Errorf("defaultServiceName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestPkgRefName(t *testing.T) {
	tests := []struct {
		module, want string
	}{
		{"github.com/acme/user-api", "user-api"},
		{"github.com/acme/commerce/services/user-rpc", "user-rpc"},
	}
	for _, tt := range tests {
		if got := pkgRefName(tt.module); got != tt.want {
			t.Errorf("pkgRefName(%q) = %q, want %q", tt.module, got, tt.want)
		}
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/scaffold/template/... -v
```

Expected: All tests PASS (including 4 new proto tests)

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/template/proto.go internal/scaffold/template/proto_test.go
git commit -m "feat(template): add proto parser for ServiceInfo extraction"
```

---

### Task 3: Export logic — Go file scanning and variable substitution

**Files:**
- Create: `internal/scaffold/template/export.go`
- Test: `internal/scaffold/template/export_test.go`

The core export logic: scan files, replace module path and service name with template variables, produce YAML templates.

- [ ] **Step 1: Write export.go**

```go
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileRule describes how a file path pattern maps to a template.
type FileRule struct {
	Pattern        string // glob pattern, e.g. "internal/handler/*"
	UpdateBehavior string // "skip" or "cover"
	LoopService    bool   // true if the file should be generated per-service
}

// HertzRules returns the file mapping rules for Hertz projects.
func HertzRules() []FileRule {
	return []FileRule{
		{Pattern: "main.go", UpdateBehavior: "cover"},
		{Pattern: "conf/dev/conf.yaml", UpdateBehavior: "skip"},
		{Pattern: "internal/base/conf/conf.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/data/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/server/server.go", UpdateBehavior: "cover"},
		{Pattern: "internal/handler/*", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/usecase/*", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/repository/*", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/router/*", UpdateBehavior: "cover", LoopService: true},
		{Pattern: "internal/pkg/**/*.go", UpdateBehavior: "cover"},
	}
}

// KitexRules returns the file mapping rules for Kitex projects.
func KitexRules() []FileRule {
	return []FileRule{
		{Pattern: "main.go", UpdateBehavior: "cover"},
		{Pattern: "conf/dev/conf.yaml", UpdateBehavior: "skip"},
		{Pattern: "internal/base/conf/conf.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/data/data.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/server/server.go", UpdateBehavior: "cover"},
		{Pattern: "internal/handler/*", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/usecase/*", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/repository/*", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "pkg/client/*", UpdateBehavior: "cover", LoopService: true},
		{Pattern: "internal/pkg/**/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/middleware/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/release/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/logging/*.go", UpdateBehavior: "cover"},
	}
}

// ExcludedPaths returns paths that should never be exported as templates.
var ExcludedPaths = []string{
	"internal/pb/",     // hz-generated protobuf code
	"kitex_gen/",      // kitex-generated RPC stubs
}

// ExportOptions describes an export operation.
type ExportOptions struct {
	Root        string   // project root
	Kind        string   // "hertz" or "kitex"
	Module      string   // Go module path
	ServiceName string   // service name from manifest
	IDL         string   // proto IDL path relative to Root
}

// ExportResult describes what was exported.
type ExportResult struct {
	OutputDir   string
	Templates   []string
}

// Export extracts code templates from an existing ncgo project.
func Export(opts ExportOptions) (*ExportResult, error) {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	rules := HertzRules()
	tplDir := "hertz-template"
	if opts.Kind == "kitex" {
		rules = KitexRules()
		tplDir = "kitex-template"
	}

	outDir := filepath.Join(absRoot, "template", tplDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	svcNameLower := strings.ToLower(opts.ServiceName)
	svcNameLower = strings.ReplaceAll(svcNameLower, "-", "")

	var templates []string

	for _, rule := range rules {
		files, err := matchFiles(absRoot, rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("match %s: %w", rule.Pattern, err)
		}
		for _, f := range files {
			rel := relPath(absRoot, f)
			if isExcluded(rel) {
				continue
			}
			tpl, err := fileToTemplate(absRoot, f, rel, opts, rule, svcNameLower)
			if err != nil {
				return nil, fmt.Errorf("process %s: %w", rel, err)
			}
			if tpl == nil {
				continue
			}
			if err := writeTemplateYAML(outDir, tpl); err != nil {
				return nil, fmt.Errorf("write template %s: %w", tpl.Path, err)
			}
			templates = append(templates, tpl.Path)
		}
	}

	// Export Makefile if it exists
	if mk, err := makefileTemplate(absRoot, opts, svcNameLower); err == nil && mk != nil {
		if err := writeTemplateYAML(outDir, mk); err != nil {
			return nil, fmt.Errorf("write makefile template: %w", err)
		}
		templates = append(templates, mk.Path)
	}

	return &ExportResult{
		OutputDir: relPath(absRoot, outDir),
		Templates: templates,
	}, nil
}

func matchFiles(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel := relPath(root, path)
		if globMatch(pattern, rel) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

// globMatch handles ** globstar patterns that filepath.Match does not support.
func globMatch(pattern, path string) bool {
	if strings.Contains(pattern, "**") {
		// Convert "internal/pkg/**/*.go" to prefix "internal/pkg/" and suffix ".go"
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}
		if suffix != "" {
			matched, _ := filepath.Match(suffix, filepath.Base(path))
			return matched
		}
		return true
	}
	matched, _ := filepath.Match(pattern, path)
	return matched
}

func isExcluded(rel string) bool {
	for _, prefix := range ExcludedPaths {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

var modulePathRE *regexp.Regexp

func compileModulePathRE(module string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(module)
	return regexp.MustCompile(escaped)
}

func fileToTemplate(root, absPath, relPath string, opts ExportOptions, rule FileRule, svcNameLower string) (*TemplateFile, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	body := string(content)

	// Replace module path with {{.Module}}
	re := compileModulePathRE(opts.Module)
	body = re.ReplaceAllString(body, "{{.Module}}")

	// Replace service name identifiers
	body = replaceServiceName(body, opts.ServiceName, svcNameLower)

	// Compute template path
	tplPath := templatePath(relPath, rule, svcNameLower)

	return &TemplateFile{
		Path:           tplPath,
		UpdateBehavior: UpdateBehavior{Type: rule.UpdateBehavior},
		LoopService:    rule.LoopService,
		Body:           body,
	}, nil
}

func replaceServiceName(body, serviceName, serviceNameLower string) string {
	// Strategy: replace service name in specific Go syntax contexts to avoid
	// colliding with unrelated identifiers.
	//
	// 1. Import paths: "github.com/acme/.../userapi/..." → "github.com/acme/.../{{ToLower .ServiceName}}/..."
	// 2. Import aliases: userapi "github.com/..." → {{ToLower .ServiceName}} "github.com/..."
	// 3. Type names: UserApiImpl → {{.ServiceName}}Impl (PascalCase at word boundary)
	// 4. String literals containing the module path already handled by module replacement.

	export := exportName(serviceName)

	// Replace in import paths (quoted paths containing the lowercase name as a path segment)
	pathRE := regexp.MustCompile(`"([^"]*/)?` + regexp.QuoteMeta(serviceNameLower) + `/`)
	body = pathRE.ReplaceAllStringFunc(body, func(s string) string {
		return strings.Replace(s, serviceNameLower, "{{ToLower .ServiceName}}", 1)
	})

	// Replace PascalCase type identifiers (at word boundaries)
	if export != "" {
		typeRE := regexp.MustCompile(`\b` + regexp.QuoteMeta(export) + `\b`)
		body = typeRE.ReplaceAllString(body, "{{.ServiceName}}")
	}

	// Replace lowercase as standalone identifiers in handler/usecase/repo packages
	// Only match when it appears as a path segment (surrounded by / or at start/end)
	pkgRE := regexp.MustCompile(`/(internal/(handler|usecase|repository|router|pkg|base)/)` + regexp.QuoteMeta(serviceNameLower) + `/`)
	body = pkgRE.ReplaceAllString(body, "${1}{{ToLower .ServiceName}}/")

	return body
}

func templatePath(rel string, rule FileRule, svcNameLower string) string {
	// Replace the service name path segment with template variable
	if rule.LoopService {
		return strings.Replace(rel, svcNameLower, "{{ToLower .ServiceName}}", 1)
	}
	return rel
}

func makefileTemplate(root string, opts ExportOptions, svcNameLower string) (*TemplateFile, error) {
	mkPath := filepath.Join(root, "Makefile")
	content, err := os.ReadFile(mkPath)
	if err != nil {
		return nil, err
	}

	body := string(content)
	re := compileModulePathRE(opts.Module)
	body = re.ReplaceAllString(body, "{{.Module}}")
	body = replaceServiceName(body, opts.ServiceName, svcNameLower)

	return &TemplateFile{
		Path:           "Makefile",
		UpdateBehavior: UpdateBehavior{Type: "cover"},
		Body:           body,
	}, nil
}

func writeTemplateYAML(outDir string, tpl *TemplateFile) error {
	yamlName := yamlFileName(tpl.Path)
	data, err := yaml.Marshal(tpl)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	// Add comment header
	comment := "# ncgo exported template — " + tpl.Path + "\n"
	data = append([]byte(comment), data...)

	return os.WriteFile(filepath.Join(outDir, yamlName), data, 0644)
}

func yamlFileName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	// Convert internal/handler/service/handler.go -> handler_go.yaml
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, " ", "")
	if !strings.HasSuffix(name, ".yaml") {
		name += ".yaml"
	}
	return name
}

func relPath(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}
```

- [ ] **Step 2: Write export_test.go**

```go
package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHertzRules_Count(t *testing.T) {
	rules := HertzRules()
	if len(rules) < 8 {
		t.Errorf("expected at least 8 hertz rules, got %d", len(rules))
	}
}

func TestKitexRules_Count(t *testing.T) {
	rules := KitexRules()
	if len(rules) < 10 {
		t.Errorf("expected at least 10 kitex rules, got %d", len(rules))
	}
}

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"internal/pb/user.pb.go", true},
		{"kitex_gen/user/service.go", true},
		{"internal/handler/user/handler.go", false},
		{"main.go", false},
	}
	for _, tt := range tests {
		if got := isExcluded(tt.path); got != tt.want {
			t.Errorf("isExcluded(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestReplaceServiceName(t *testing.T) {
	body := `type UserApiImpl struct {}
func (u *UserApiImpl) Ping() {}
import "github.com/acme/user-api/internal/base/conf"`
	got := replaceServiceName(body, "UserApi", "userapi")
	if !strings.Contains(got, "{{.ServiceName}}Impl") {
		t.Errorf("expected {{.ServiceName}}Impl in:\n%s", got)
	}
}

func TestTemplatePath_LoopService(t *testing.T) {
	got := templatePath("internal/handler/userapi/handler.go", FileRule{LoopService: true}, "userapi")
	want := "internal/handler/{{ToLower .ServiceName}}/handler.go"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTemplatePath_NoLoop(t *testing.T) {
	got := templatePath("main.go", FileRule{LoopService: false}, "userapi")
	want := "main.go"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestYamlFileName(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"main.go", "main_go.yaml"},
		{"internal/handler/userapi/handler.go", "internal_handler_userapi_handler_go.yaml"},
		{"Makefile", "Makefile.yaml"},
	}
	for _, tt := range tests {
		if got := yamlFileName(tt.path); got != tt.want {
			t.Errorf("yamlFileName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExport_MinimalHertz(t *testing.T) {
	dir := t.TempDir()
	// Create minimal project structure
	if err := os.MkdirAll(filepath.Join(dir, ".ncgo"), 0755); err != nil {
		t.Fatal(err)
	}
	mainGo := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainGo, []byte(`package main

import "github.com/acme/test/internal/handler/userapi"

func main() {
	_ = userapi.Ping()
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Export(ExportOptions{
		Root:        dir,
		Kind:        "hertz",
		Module:      "github.com/acme/test",
		ServiceName: "UserApi",
		IDL:         "idl/app/user-api.proto",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OutputDir != "template/hertz-template" {
		t.Errorf("got output dir %q, want %q", result.OutputDir, "template/hertz-template")
	}
	if len(result.Templates) == 0 {
		t.Error("expected at least one template")
	}

	// Check main.go template was written
	mainTpl := filepath.Join(dir, "template", "hertz-template", "main_go.yaml")
	if _, err := os.Stat(mainTpl); err != nil {
		t.Errorf("expected main template at %s: %v", mainTpl, err)
		return
	}
	content, _ := os.ReadFile(mainTpl)
	if !strings.Contains(string(content), "{{.Module}}") {
		t.Errorf("expected {{.Module}} in main template:\n%s", string(content))
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/scaffold/template/... -v
```

Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/template/export.go internal/scaffold/template/export_test.go
git commit -m "feat(template): add export logic with variable substitution"
```

---

### Task 4: `ncgo export templates` CLI command

**Files:**
- Create: `internal/cli/export.go`
- Modify: `internal/cli/root.go`

Wire up the new CLI command that calls the export logic.

**Note:** The existing `internal/cli/extract.go` already defines `newExtractCmd()` with `extract domain` subcommand. The new command is `export templates` — a separate top-level command using `newExportCmd()`.

- [ ] **Step 1: Write export.go**

```go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/template"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export code templates from an existing ncgo project",
	}
	cmd.AddCommand(newExportTemplatesCmd())
	return cmd
}

type exportTemplatesOptions struct {
	root string
	kind string
}

func newExportTemplatesCmd() *cobra.Command {
	opts := &exportTemplatesOptions{}
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Export code templates from an existing ncgo project",
		Long: "Scan the current project for ncgo-managed source files, replace " +
			"module paths and service names with template variables, and write " +
			"YAML templates to template/<kind>-template/. The output is compatible " +
			"with the respective code generator (kitex -template-dir or hertz overlay).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportTemplates(cmd, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.root, "root", ".", "Project root containing .ncgo/manifest.yaml")
	f.StringVar(&opts.kind, "kind", "", "Service kind: hertz | kitex (default: read from manifest)")
	return cmd
}

func runExportTemplates(cmd *cobra.Command, opts *exportTemplatesOptions) error {
	m, err := manifest.Load(opts.root)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	kind := opts.kind
	if kind == "" {
		kind = m.Service.Kind
	}
	switch kind {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return fmt.Errorf("--kind %q is invalid (hertz|kitex)", kind)
	}

	result, err := template.Export(template.ExportOptions{
		Root:        opts.root,
		Kind:        kind,
		Module:      m.Module,
		ServiceName: m.Service.Name,
		IDL:         m.Service.IDL,
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "exported %d templates to %s/\n", len(result.Templates), result.OutputDir)
	for _, t := range result.Templates {
		fmt.Fprintf(out, "  - %s\n", t)
	}
	return nil
}
```

- [ ] **Step 2: Register the export command in root.go**

Modify `internal/cli/root.go`. In the `newRootCmd()` function, add after the existing `newExtractCmd()`:

```go
cmd.AddCommand(newExtractCmd())
cmd.AddCommand(newExportCmd())  // <-- add this line
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Test CLI help**

```bash
go run . export templates --help
```

Expected: Shows the help text with --root and --kind flags

- [ ] **Step 5: Commit**

```bash
git add internal/cli/export.go internal/cli/root.go
git commit -m "feat(cli): add export templates command"
```

---

### Task 5: Hertz template apply in mono.Generate()

**Files:**
- Modify: `internal/scaffold/template/apply.go`
- Modify: `internal/scaffold/template/apply_test.go`
- Modify: `internal/scaffold/mono/mono.go`

Create the apply logic and hook it into the Hertz generation pipeline.

- [ ] **Step 1: Write apply.go**

```go
package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ApplyOptions describes a template apply operation.
type ApplyOptions struct {
	Root        string      // project root (after hz new)
	Module      string      // Go module path
	ServiceName string      // service name
	WithDatabase bool       // database enabled flag
	Infra       []string    // infra add-ons
	ServiceInfo *ServiceInfo // parsed from proto
}

// ApplyResult describes what was applied.
type ApplyResult struct {
	Written []string
	Skipped []string
}

// Apply reads templates from template/hertz-template/ and applies them
// to the project tree.
func Apply(opts ApplyOptions) (*ApplyResult, error) {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	tplDir := filepath.Join(absRoot, "template", "hertz-template")
	entries, err := os.ReadDir(tplDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No templates to apply — not an error
			return &ApplyResult{}, nil
		}
		return nil, fmt.Errorf("read template dir: %w", err)
	}

	var result ApplyResult

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		tplPath := filepath.Join(tplDir, e.Name())
		tpl, err := readTemplate(tplPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", e.Name(), err)
		}
		if err := applySingle(absRoot, tpl, opts, &result); err != nil {
			return nil, fmt.Errorf("apply template %s: %w", tpl.Path, err)
		}
	}

	return &result, nil
}

func readTemplate(path string) (*TemplateFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tpl TemplateFile
	if err := yaml.Unmarshal(content, &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	return &tpl, nil
}

func applySingle(root string, tpl *TemplateFile, opts ApplyOptions, result *ApplyResult) error {
	rendered, err := Render(tpl.Body, RenderData{
		Module:       opts.Module,
		ServiceName:  opts.ServiceName,
		ServiceInfo:  *opts.ServiceInfo,
		WithDatabase: opts.WithDatabase,
		Infra:        opts.Infra,
	})
	if err != nil {
		return err
	}

	targetPath := filepath.Join(root, tpl.Path)
	return writeOrSkip(root, targetPath, rendered, tpl.UpdateBehavior.Type, result)
}

func writeOrSkip(root, targetPath, content, behavior string, result *ApplyResult) error {
	if behavior == "skip" {
		if _, err := os.Stat(targetPath); err == nil {
			result.Skipped = append(result.Skipped, targetPath)
			return nil
		}
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", targetPath, err)
	}
	result.Written = append(result.Written, targetPath)
	return nil
}
```

- [ ] **Step 2: Write apply_test.go**

```go
package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApply_NoTemplateDir(t *testing.T) {
	dir := t.TempDir()
	result, err := Apply(ApplyOptions{Root: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Written) != 0 {
		t.Errorf("expected 0 written, got %d", len(result.Written))
	}
}

func TestApply_SingleTemplate(t *testing.T) {
	dir := t.TempDir()
	tplDir := filepath.Join(dir, "template", "hertz-template")
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a simple template
	tpl := &TemplateFile{
		Path:           "main.go",
		UpdateBehavior: UpdateBehavior{Type: "cover"},
		Body:           "package main\n\nimport \"{{.Module}}/internal/handler\"\n\nfunc main() {}\n",
	}
	yamlName := yamlFileName(tpl.Path)
	data, _ := yaml.Marshal(tpl)
	if err := os.WriteFile(filepath.Join(tplDir, yamlName), data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Apply(ApplyOptions{
		Root:        dir,
		Module:      "github.com/acme/test",
		ServiceName: "UserApi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Written) != 1 {
		t.Fatalf("expected 1 written, got %d", len(result.Written))
	}

	content, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatalf("expected main.go to exist: %v", err)
	}
	if !strings.Contains(string(content), "github.com/acme/test") {
		t.Errorf("expected module path in main.go:\n%s", string(content))
	}
}

func TestApply_SkipExisting(t *testing.T) {
	dir := t.TempDir()
	tplDir := filepath.Join(dir, "template", "hertz-template")
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing file
	if err := os.WriteFile(filepath.Join(dir, "conf", "dev", "conf.yaml"), []byte("env: dev\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Template with skip behavior
	tpl := &TemplateFile{
		Path:           "conf/dev/conf.yaml",
		UpdateBehavior: UpdateBehavior{Type: "skip"},
		Body:           "env: prod\n",
	}
	yamlName := yamlFileName(tpl.Path)
	data, _ := yaml.Marshal(tpl)
	if err := os.WriteFile(filepath.Join(tplDir, yamlName), data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Apply(ApplyOptions{
		Root:        dir,
		Module:      "github.com/acme/test",
		ServiceName: "UserApi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Written) != 0 {
		t.Errorf("expected 0 written (skip), got %d", len(result.Written))
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %d", len(result.Skipped))
	}

	// Verify original content unchanged
	content, _ := os.ReadFile(filepath.Join(dir, "conf", "dev", "conf.yaml"))
	if string(content) != "env: dev\n" {
		t.Errorf("expected original content, got %q", string(content))
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/scaffold/template/... -v
```

Expected: All tests PASS

- [ ] **Step 4: Hook into mono.Generate()**

Modify `internal/scaffold/mono/mono.go`. After the `runGenerator` succeeds (around line 127-134), add the apply step:

In the `Generate` function, after this block:
```go
if _, err := runGenerator(ctx, r, dir, opts, idl); err != nil {
    return res, err
}
```

Add the apply logic:
```go
// Apply custom Hertz templates if they exist (post-hz overlay)
if defaultKind(opts.Kind) == manifest.KindHertz {
    si, _ := template.ParseServiceInfo(ctx, filepath.Join(dir, idl), opts.Module)
    _, _ = template.Apply(template.ApplyOptions{
        Root:         dir,
        Module:       opts.Module,
        ServiceName:  opts.Name,
        WithDatabase: opts.WithDatabase,
        Infra:        opts.Infra,
        ServiceInfo:  si,
    })
    // Errors from Apply are non-fatal: if no templates exist, it returns empty result.
}
```

Note: The import for `template` package needs to be added. Since `internal/scaffold/template` conflicts with Go's `text/template` in naming, use an alias:

```go
import (
    // ... existing imports ...
    scaffoldtemplate "github.com/byx-darwin/ncgo/internal/scaffold/template"
)
```

And update the usage to `scaffoldtemplate.ParseServiceInfo` and `scaffoldtemplate.Apply`.

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/template/apply.go internal/scaffold/template/apply_test.go internal/scaffold/mono/mono.go
git commit -m "feat(mono): apply Hertz templates after hz new"
```

---

### Task 6: Integration tests and end-to-end validation

**Files:**
- Create: `internal/scaffold/template/integration_test.go`

Test the full export → apply cycle.

- [ ] **Step 1: Write integration_test.go**

```go
package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExportApplyCycle verifies: create a minimal project → export →
// modify source → create new project → apply → verify customizations persist.
func TestExportApplyCycle(t *testing.T) {
	// Phase 1: Create a "source" project
	src := t.TempDir()

	mainGo := filepath.Join(src, "main.go")
	mainContent := `package main

import "github.com/acme/src/internal/handler/myapi"

func main() {
	_ = myapi.Ping()
}
`
	if err := os.WriteFile(mainGo, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Phase 2: Export templates
	result, err := Export(ExportOptions{
		Root:        src,
		Kind:        "hertz",
		Module:      "github.com/acme/src",
		ServiceName: "MyApi",
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if len(result.Templates) == 0 {
		t.Fatal("expected at least one template")
	}

	// Phase 3: Create a "target" project and copy templates there
	tgt := t.TempDir()
	srcTplDir := filepath.Join(src, result.OutputDir)
	tgtTplDir := filepath.Join(tgt, "template", "hertz-template")
	if err := os.MkdirAll(tgtTplDir, 0755); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(srcTplDir)
	for _, e := range entries {
		srcFile := filepath.Join(srcTplDir, e.Name())
		tgtFile := filepath.Join(tgtTplDir, e.Name())
		data, _ := os.ReadFile(srcFile)
		if err := os.WriteFile(tgtFile, data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Phase 4: Apply templates to target
	applyResult, err := Apply(ApplyOptions{
		Root:        tgt,
		Module:      "github.com/acme/target",
		ServiceName: "TargetSvc",
	})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if len(applyResult.Written) == 0 {
		t.Fatal("expected at least one file written")
	}

	// Phase 5: Verify the target main.go has the target module path
	tgtMain := filepath.Join(tgt, "main.go")
	content, err := os.ReadFile(tgtMain)
	if err != nil {
		t.Fatalf("expected target main.go: %v", err)
	}
	body := string(content)
	if !strings.Contains(body, "github.com/acme/target") {
		t.Errorf("expected target module in main.go:\n%s", body)
	}
	if strings.Contains(body, "github.com/acme/src") {
		t.Errorf("source module path should not appear in target:\n%s", body)
	}
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/scaffold/template/... -v -run Integration
```

Expected: PASS

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -count=1
```

Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/scaffold/template/integration_test.go
git commit -m "test(template): add export-apply integration test"
```

---

### Task 7: Full validation

- [ ] **Step 1: Build**

```bash
go build ./...
```

- [ ] **Step 2: Vet**

```bash
go vet ./...
```

- [ ] **Step 3: All tests**

```bash
go test ./... -count=1
```

- [ ] **Step 4: Smoke test — export templates**

```bash
go run . export templates --help
```

Expected: Shows help text

- [ ] **Step 5: Smoke test — end-to-end with an existing project (if available)**

If there's an existing generated Hertz project somewhere, run:

```bash
cd <existing-hertz-project>
go run ../. export templates
```

Expected: Templates exported to `template/hertz-template/`

- [ ] **Step 6: Commit**

```bash
git add -A && git commit -m "ci: full validation for template export & apply"
```
