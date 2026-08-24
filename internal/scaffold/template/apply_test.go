package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

	if err := os.MkdirAll(filepath.Join(dir, "conf", "dev"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "conf", "dev", "conf.yaml"), []byte("env: dev\n"), 0644); err != nil {
		t.Fatal(err)
	}

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

	content, _ := os.ReadFile(filepath.Join(dir, "conf", "dev", "conf.yaml"))
	if string(content) != "env: dev\n" {
		t.Errorf("expected original content, got %q", string(content))
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test-dup-verify", "test_dup_verify"},
		{"TestDupVerify", "test_dup_verify"},
		{"TestDupVerifyService", "test_dup_verify_service"},
		{"my-service", "my_service"},
		{"MyService", "my_service"},
		{"simple", "simple"},
		{"ALREADY_SNAKE", "already_snake"},
		{"HTTPServer", "http_server"},
		{"parseXML", "parse_xml"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
