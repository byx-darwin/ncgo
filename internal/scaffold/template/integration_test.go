package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExportApplyCycle verifies: create a minimal project → export →
// copy templates to a new project → apply → verify customizations persist.
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

	confGo := filepath.Join(src, "internal", "base", "conf")
	if err := os.MkdirAll(confGo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confGo, "conf.go"), []byte(`package conf

type Config struct {
	Port int
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Phase 2: Export templates from source
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

	// Phase 4: Apply templates to target (no proto services, so non-loop only)
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

// TestExportKitexCycle verifies the same cycle works for Kitex kind.
func TestExportKitexCycle(t *testing.T) {
	src := t.TempDir()

	mainGo := filepath.Join(src, "main.go")
	if err := os.WriteFile(mainGo, []byte(`package main

import "github.com/acme/kitex-svc/internal/handler/userrpc"

func main() {
	_ = userrpc.Ping()
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Export(ExportOptions{
		Root:        src,
		Kind:        "kitex",
		Module:      "github.com/acme/kitex-svc",
		ServiceName: "UserRpc",
	})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if result.OutputDir != "template/kitex-template" {
		t.Errorf("got output dir %q, want %q", result.OutputDir, "template/kitex-template")
	}
	if len(result.Templates) == 0 {
		t.Fatal("expected at least one template")
	}

	// Verify main template has module substitution
	mainTpl := filepath.Join(src, "template", "kitex-template", "main_go.yaml")
	content, _ := os.ReadFile(mainTpl)
	if !strings.Contains(string(content), "{{.Module}}") {
		t.Errorf("expected {{.Module}} in main template:\n%s", string(content))
	}
}
