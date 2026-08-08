package mono

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

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

// templatePkgOptions returns deterministic Options for a kitex template-package
// generation. NCGOVersion/AssetsVersion/Now are pinned so validate() passes and
// the manifest is deterministic (not asserted here, but stable for debugging).
func templatePkgOptions(t *testing.T, templateDir string) Options {
	t.Helper()
	return Options{
		Name:          "demo",
		Module:        "github.com/acme/demo",
		Kind:          manifest.KindKitex,
		Dir:           filepath.Join(t.TempDir(), "svc"),
		TemplateDir:   templateDir,
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.0.0-test",
		Now:           time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		NoGenerate:    true,
		Runner:        &fakeRunner{},
	}
}

func TestGenerateTemplatePackageKitex(t *testing.T) {
	opts := templatePkgOptions(t, seedTemplatePackage(t, true))
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.TemplateIDLFallback {
		t.Error("unexpected IDL fallback")
	}
	// package yaml replaced embedded set: exactly the package's file remains
	entries, _ := os.ReadDir(filepath.Join(opts.Dir, "template", "kitex-template"))
	if len(entries) != 1 || entries[0].Name() != "main_go.yaml" {
		t.Errorf("template dir = %v", entries)
	}
	// IDL rendered onto the kitex default path with the target service name
	body, err := os.ReadFile(filepath.Join(opts.Dir, "idl", "demo.proto"))
	if err != nil {
		t.Fatalf("rendered idl missing: %v", err)
	}
	if !strings.Contains(string(body), "service demo {}") {
		t.Errorf("service name not rendered:\n%s", body)
	}
}

func TestGenerateTemplatePackageIDLFallback(t *testing.T) {
	opts := templatePkgOptions(t, seedTemplatePackage(t, false))
	res, err := Generate(context.Background(), opts)
	if err != nil || !res.TemplateIDLFallback {
		t.Fatalf("want fallback, got %+v, %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(opts.Dir, "idl", "demo.proto")); err != nil {
		t.Errorf("built-in placeholder should apply: %v", err)
	}
}

func TestGenerateTemplatePackagePresetConflict(t *testing.T) {
	opts := templatePkgOptions(t, t.TempDir())
	opts.Preset = "rule-center"
	_, err := Generate(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("want mutual exclusion error, got %v", err)
	}
}

func TestGenerateTemplatePackageKindMismatch(t *testing.T) {
	pkg := t.TempDir()
	os.WriteFile(filepath.Join(pkg, "template.yaml"), []byte("kind: hertz\n"), 0o644)
	os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(pkg, "kitex-template", "a.yaml"), []byte("path: main.go\n"), 0o644)
	opts := templatePkgOptions(t, pkg)
	_, err := Generate(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "has kind") {
		t.Errorf("want kind mismatch error, got %v", err)
	}
}

func TestGenerateTemplatePackageParseError(t *testing.T) {
	pkg := t.TempDir()
	os.WriteFile(filepath.Join(pkg, "template.yaml"), []byte("{{invalid"), 0o644)
	os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(pkg, "kitex-template", "a.yaml"), []byte("path: main.go\n"), 0o644)
	opts := templatePkgOptions(t, pkg)
	_, err := Generate(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("want parse error, got %v", err)
	}
}
