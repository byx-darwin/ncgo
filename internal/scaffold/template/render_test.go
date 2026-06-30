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

func TestHasInfra(t *testing.T) {
	tests := []struct {
		name  string
		infra []string
		query string
		want  bool
	}{
		{"empty slice", nil, "redis", false},
		{"empty non-nil", []string{}, "redis", false},
		{"match first", []string{"redis", "kafka"}, "redis", true},
		{"match last", []string{"kafka", "es"}, "es", true},
		{"match middle", []string{"redis", "kafka", "es"}, "kafka", true},
		{"no match", []string{"redis", "kafka"}, "es", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasInfra(tt.infra, tt.query); got != tt.want {
				t.Errorf("hasInfra(%v, %q) = %v, want %v", tt.infra, tt.query, got, tt.want)
			}
		})
	}
}

func TestRender_HasInfra(t *testing.T) {
	body := `{{if hasInfra .Infra "redis"}}redis_enabled{{else}}no_redis{{end}}`

	got, err := Render(body, RenderData{Infra: []string{"redis", "kafka"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "redis_enabled" {
		t.Errorf("got %q, want %q", got, "redis_enabled")
	}

	got, err = Render(body, RenderData{Infra: []string{"kafka"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "no_redis" {
		t.Errorf("got %q, want %q", got, "no_redis")
	}
}
