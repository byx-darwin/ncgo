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

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern, path string
		want          bool
	}{
		{"main.go", "main.go", true},
		{"main.go", "main_test.go", false},
		{"internal/handler/*", "internal/handler/user", true},                     // * matches single segment
		{"internal/handler/*", "internal/handler/user/handler.go", false},         // * doesn't cross segments
		{"internal/handler/**/*.go", "internal/handler/userapi/handler.go", true}, // ** matches nested dirs
		{"internal/handler/**/*.go", "internal/handler/userapi/handler_test.go", true},
		{"internal/handler/**/*.go", "internal/handler/file.go", true}, // ** matches zero nested dirs
		{"internal/pkg/**/*.go", "internal/pkg/interceptor/auth.go", true},
		{"internal/pkg/**/*.go", "internal/pkg/rpcerror/error.go", true},
		{"internal/pkg/**/*.go", "internal/other/file.go", false},
		{"pkg/client/**/*.go", "pkg/client/userrpc/client.go", true},
		{"*.go", "file.go", true},
		{"*.go", "file.txt", false},
	}
	for _, tt := range tests {
		if got := globMatch(tt.pattern, tt.path); got != tt.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

func TestReplaceServiceName(t *testing.T) {
	body := `type UserApiImpl struct {}
func (u *UserApiImpl) Ping() {}`
	got := replaceServiceName(body, "UserApi")
	if !strings.Contains(got, "{{.ServiceName}}Impl") {
		t.Errorf("expected {{.ServiceName}}Impl in:\n%s", got)
	}
	if strings.Contains(got, "UserApiImpl") {
		t.Errorf("should not contain original UserApiImpl in:\n%s", got)
	}
}

func TestReplaceServiceName_ImportPath(t *testing.T) {
	body := `import "github.com/acme/test/internal/handler/userapi"`
	got := replaceServiceName(body, "UserApi")
	// replaceServiceName substitutes bounded lowercase tokens in path segments
	// and package qualifiers, so the userapi import path is variabilized here;
	// templatePath separately handles parameterizing the OUTPUT file path.
	if strings.Contains(got, "userapi") {
		t.Errorf("import path should be variabilized, got:\n%s", got)
	}
	if !strings.Contains(got, "{{ToLower .ServiceName}}") {
		t.Errorf("expected lowercase substitution in import path, got:\n%s", got)
	}
}

func TestIDLTemplatePath_SubstitutesBaseOnly(t *testing.T) {
	tests := []struct {
		name string
		rel  string
		svc  string
		want string
	}{
		{
			name: "hertz default app dir stays literal even when it equals service name",
			rel:  "idl/app/app.proto",
			svc:  "app",
			want: "idl/app/{{ToLower .ServiceName}}.proto",
		},
		{
			name: "hertz standard userapi",
			rel:  "idl/app/userapi.proto",
			svc:  "UserApi",
			want: "idl/app/{{ToLower .ServiceName}}.proto",
		},
		{
			name: "kitex root idl",
			rel:  "idl/userapi.proto",
			svc:  "UserApi",
			want: "idl/{{ToLower .ServiceName}}.proto",
		},
		{
			name: "parent dir not rewritten",
			rel:  "idl/userapi/userapi.proto",
			svc:  "UserApi",
			want: "idl/userapi/{{ToLower .ServiceName}}.proto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idlTemplatePath(tt.rel, ExportOptions{ServiceName: tt.svc}); got != tt.want {
				t.Errorf("idlTemplatePath(%q, %q) = %q, want %q", tt.rel, tt.svc, got, tt.want)
			}
		})
	}
}

func TestTemplatePath_LoopService(t *testing.T) {
	got := templatePath("internal/handler/userapi/handler.go", FileRule{LoopService: true}, "userapi", "userapi")
	want := "internal/handler/{{ToLower .ServiceName}}/handler.go"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTemplatePath_NoLoop(t *testing.T) {
	got := templatePath("main.go", FileRule{LoopService: false}, "userapi", "userapi")
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

func TestServiceNameLower(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"UserApi", "userapi"},
		{"User-Api", "userapi"},
		{"UserService", "userservice"},
	}
	for _, tt := range tests {
		if got := serviceNameLower(tt.in); got != tt.want {
			t.Errorf("serviceNameLower(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

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

func writeFileExport(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// loadExportedTemplateByPath walks template/<kind>-template/*.yaml and returns
// the TemplateFile whose Path matches want, or fails.
func loadExportedTemplateByPath(t *testing.T, root, kind, want string) *TemplateFile {
	t.Helper()
	dir := filepath.Join(root, "template", kind+"-template")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read template dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		tpl, err := readTemplateYAML(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read yaml %s: %v", e.Name(), err)
		}
		if tpl.Path == want {
			return tpl
		}
	}
	t.Fatalf("no exported template with path %q in %s", want, dir)
	return nil
}

func TestExport_DDDLayers_Hertz(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	// aggregate "user" — name deliberately != service name "UserApi"
	writeFileExport(t, dir, "internal/domain/user/entity.go", `package user

// User is an aggregate root in module github.com/acme/test.
type User struct{ ID string }
`)
	writeFileExport(t, dir, "internal/domain/user/repository.go", `package user

import "context"

type Repository interface {
	Save(ctx context.Context, u *User) error
}
`)
	writeFileExport(t, dir, "internal/application/user/user_service.go", `package user

import "github.com/acme/test/internal/domain/user"

type Service struct{ repo user.Repository }
`)

	if _, err := Export(ExportOptions{Root: dir, Kind: "hertz",
		Module: "github.com/acme/test", ServiceName: "UserApi"}); err != nil {
		t.Fatalf("export: %v", err)
	}

	// domain entity: aggregate path preserved literally, module variabilized, skip.
	ent := loadExportedTemplateByPath(t, dir, "hertz", "internal/domain/user/entity.go")
	if ent.UpdateBehavior.Type != "skip" {
		t.Errorf("domain entity UpdateBehavior = %q, want skip", ent.UpdateBehavior.Type)
	}
	if ent.LoopService {
		t.Errorf("domain entity LoopService = true, want false (aggregate-organized)")
	}
	if !strings.Contains(ent.Body, "module {{.Module}}") {
		t.Errorf("module not variabilized in domain entity:\n%s", ent.Body)
	}

	// application service: path preserved, import module variabilized.
	app := loadExportedTemplateByPath(t, dir, "hertz", "internal/application/user/user_service.go")
	if app.UpdateBehavior.Type != "skip" {
		t.Errorf("application service UpdateBehavior = %q, want skip", app.UpdateBehavior.Type)
	}
	if !strings.Contains(app.Body, "{{.Module}}/internal/domain/user") {
		t.Errorf("import module not variabilized in application service:\n%s", app.Body)
	}
}

func TestExport_DDDLayers_Kitex(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFileExport(t, dir, "internal/domain/role/entity.go", `package role

// Role is an aggregate root in module github.com/acme/test.
type Role struct{ ID string }
`)
	writeFileExport(t, dir, "internal/application/role/role_service.go", `package role

import "github.com/acme/test/internal/domain/role"

type Service struct{ _ role.Role }
`)

	if _, err := Export(ExportOptions{Root: dir, Kind: "kitex",
		Module: "github.com/acme/test", ServiceName: "UserRpc"}); err != nil {
		t.Fatalf("export: %v", err)
	}

	ent := loadExportedTemplateByPath(t, dir, "kitex", "internal/domain/role/entity.go")
	if ent.UpdateBehavior.Type != "skip" || ent.LoopService {
		t.Errorf("kitex domain entity behavior wrong: skip=%q loop=%v", ent.UpdateBehavior.Type, ent.LoopService)
	}
	if !strings.Contains(ent.Body, "module {{.Module}}") {
		t.Errorf("module not variabilized in kitex domain entity:\n%s", ent.Body)
	}
}

func TestExport_IDL(t *testing.T) {
	dir := t.TempDir()
	writeFileExport(t, dir, "main.go", "package main\n")
	writeFileExport(t, dir, "idl/app/userapi.proto",
		"syntax = \"proto3\";\npackage app;\n"+
			"option go_package = \"github.com/acme/test/kitex_gen/userapi\";\n"+
			"service UserApi {\n  rpc Ping(PingReq) returns (PingResp);\n}\n")
	writeFileExport(t, dir, "idl/openapi/openapi.proto", "syntax = \"proto3\";\n")
	writeFileExport(t, dir, "idl/validate/validate.proto", "syntax = \"proto3\";\n")

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
	if _, err := os.Stat(filepath.Join(dir, "template", "idl", "validate")); !os.IsNotExist(err) {
		t.Error("idl/validate must be excluded from export")
	}
}

func TestExport_MinimalHertz(t *testing.T) {
	dir := t.TempDir()

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
