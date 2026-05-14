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
