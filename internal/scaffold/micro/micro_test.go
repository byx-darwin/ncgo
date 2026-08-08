package micro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func baseOpts(t *testing.T) Options {
	t.Helper()
	return Options{
		Name:          "shop",
		Module:        "github.com/acme/shop",
		Dir:           filepath.Join(t.TempDir(), "shop"),
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.3.0-test",
		Now:           time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC),
	}
}

func TestGenerateProducesMicroWorkspace(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, p := range []string{"ncgo.workspace", "README.md", "compose.yaml", ".pre-commit-config.yaml", "scripts/run-go-module-checks.sh", "services/.gitkeep"} {
		if _, err := os.Stat(filepath.Join(res.Dir, p)); err != nil {
			t.Fatalf("expected %s: %v", p, err)
		}
	}
	w, err := manifest.LoadWorkspace(res.Dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if w.Mode != manifest.ModeMicro || w.Name != opts.Name || w.Module != opts.Module {
		t.Errorf("workspace mismatch: %+v", w)
	}
	if len(w.Services) != 0 {
		t.Errorf("new micro workspace should start with no services, got %v", w.Services)
	}
	readme, err := os.ReadFile(filepath.Join(res.Dir, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if !strings.Contains(string(readme), ".pre-commit-config.yaml") {
		t.Errorf("workspace README missing pre-commit guidance")
	}
	if !strings.Contains(string(readme), "compose.yaml") {
		t.Errorf("workspace README missing compose guidance")
	}
	composeBody, err := os.ReadFile(filepath.Join(res.Dir, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	if !strings.Contains(string(composeBody), "services: {}") {
		t.Errorf("empty workspace compose should start with services: {}\n---\n%s", composeBody)
	}
}

func TestGenerateRejectsNonEmptyDir(t *testing.T) {
	opts := baseOpts(t)
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(opts.Dir, "stray"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed stray: %v", err)
	}
	if _, err := Generate(opts); err == nil {
		t.Fatalf("expected error for non-empty dir")
	}
}

func TestGenerateWithTemplateDirOverlaysWorkspace(t *testing.T) {
	pkgDir := t.TempDir()
	os.WriteFile(filepath.Join(pkgDir, "template.yaml"),
		[]byte("name: test-micro\nkind: micro\ndescription: test\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(pkgDir, "workspace"), 0o755)
	os.WriteFile(filepath.Join(pkgDir, "workspace", "custom.txt.tpl"),
		[]byte("module={{.Module}} name={{.ServiceName}}\n"), 0o644)
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
	for _, p := range []string{"ncgo.workspace", "README.md", "compose.yaml"} {
		if _, err := os.Stat(filepath.Join(res.Dir, p)); err != nil {
			t.Errorf("built-in %s missing: %v", p, err)
		}
	}
	custom, err := os.ReadFile(filepath.Join(res.Dir, "custom.txt"))
	if err != nil {
		t.Fatalf("read custom.txt: %v", err)
	}
	if !strings.Contains(string(custom), "module=github.com/acme/shop") || !strings.Contains(string(custom), "name=shop") {
		t.Errorf("custom.txt not rendered: %s", custom)
	}
}

func TestValidateRejectsBadInputs(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Options)
		want string
	}{
		{"bad-name", func(o *Options) { o.Name = "Bad_Name" }, "name"},
		{"empty-module", func(o *Options) { o.Module = "" }, "module"},
		{"flat-module", func(o *Options) { o.Module = "shop" }, "module"},
		{"empty-dir", func(o *Options) { o.Dir = "" }, "Dir"},
		{"empty-version", func(o *Options) { o.NCGOVersion = "" }, "NCGOVersion"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := baseOpts(t)
			tc.mut(&o)
			_, err := Generate(o)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Generate error = %v, want containing %q", err, tc.want)
			}
		})
	}
}
