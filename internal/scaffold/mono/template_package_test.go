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

// seedRuleCenterLikePackage builds a kitex template package that mirrors the
// rule-center preset: it skips the default per-layer templates, carries its own
// schema/*.sql files, a custom layout.yaml, and a ratelimit_shared_* fragment
// (as the preset would provide).
func seedRuleCenterLikePackage(t *testing.T) string {
	t.Helper()
	pkg := t.TempDir()
	os.WriteFile(filepath.Join(pkg, "template.yaml"), []byte("name: rule-center-like\nkind: kitex\nskip_default_templates:\n  - handler.yaml\n  - server.yaml\n  - usecase.yaml\n  - repository.yaml\n"), 0644)
	os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0755)
	os.WriteFile(filepath.Join(pkg, "kitex-template", "test.yaml"), []byte("path: main.go\nupdate_behavior:\n  type: cover\nbody: |\n  package main\n"), 0644)
	os.WriteFile(filepath.Join(pkg, "kitex-template", "ratelimit_shared_resolver.yaml"), []byte("path: internal/service/ratelimit/resolver.go\nupdate_behavior:\n  type: cover\nbody: |\n  package ratelimit\n"), 0644)
	os.MkdirAll(filepath.Join(pkg, "schema"), 0755)
	os.WriteFile(filepath.Join(pkg, "schema", "000002_rate_limit_rules.sql"), []byte("-- {{.Module}}\nCREATE TABLE rate_limit_rules (id bigint);"), 0644)
	os.WriteFile(filepath.Join(pkg, "layout.yaml"), []byte("templates:\n  - path: main.go\n"), 0644)
	os.MkdirAll(filepath.Join(pkg, "idl"), 0755)
	os.WriteFile(filepath.Join(pkg, "idl", "demo.proto"), []byte("syntax = \"proto3\";\nservice {{.ServiceName}} {}\n"), 0644)
	return pkg
}

func TestGenerateTemplatePackageRuleCenterLike(t *testing.T) {
	opts := templatePkgOptions(t, seedRuleCenterLikePackage(t))
	if _, err := Generate(context.Background(), opts); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// With a non-empty skip_default_templates the overlay MERGES: the package's
	// own templates land in the template dir, retained non-skipped embedded
	// defaults stay, and the skipped per-layer defaults are dropped.
	kitexTpl := filepath.Join(opts.Dir, "template", "kitex-template")
	if _, err := os.Stat(filepath.Join(kitexTpl, "test.yaml")); err != nil {
		t.Errorf("package template test.yaml missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(kitexTpl, "ratelimit_shared_resolver.yaml")); err != nil {
		t.Errorf("package template ratelimit_shared_resolver.yaml missing: %v", err)
	}
	// Non-skipped embedded defaults must be retained by the merge, proving
	// preset equivalence (the preset path keeps these too).
	for _, kept := range []string{"main.yaml", "client.yaml", "conf.yaml", "data.yaml", "interceptor.yaml", "makefile.yaml", "migration_init.yaml", "rpcerror.yaml", "ratelimit_middleware.yaml"} {
		if _, err := os.Stat(filepath.Join(kitexTpl, kept)); err != nil {
			t.Errorf("retained embedded default %s missing after merge: %v", kept, err)
		}
	}
	if _, err := os.Stat(filepath.Join(kitexTpl, "handler.yaml")); err == nil {
		t.Error("default handler.yaml should be skipped")
	}
	if _, err := os.Stat(filepath.Join(kitexTpl, "server.yaml")); err == nil {
		t.Error("default server.yaml should be skipped")
	}
	if _, err := os.Stat(filepath.Join(kitexTpl, "usecase.yaml")); err == nil {
		t.Error("default usecase.yaml should be skipped")
	}
	if _, err := os.Stat(filepath.Join(kitexTpl, "repository.yaml")); err == nil {
		t.Error("default repository.yaml should be skipped")
	}
	// Package schema is copied and rendered with the module variable.
	schema, err := os.ReadFile(filepath.Join(opts.Dir, "internal", "db", "schema", "000002_rate_limit_rules.sql"))
	if err != nil {
		t.Fatalf("schema missing: %v", err)
	}
	schemaBody := string(schema)
	if strings.Contains(schemaBody, "{{.Module}}") {
		t.Errorf("module placeholder not replaced:\n%s", schemaBody)
	}
	if !strings.Contains(schemaBody, opts.Module) {
		t.Errorf("module not rendered into schema:\n%s", schemaBody)
	}
	// Package layout.yaml is copied into the template dir.
	layout, err := os.ReadFile(filepath.Join(opts.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("layout.yaml missing: %v", err)
	}
	if !strings.Contains(string(layout), "templates:") {
		t.Errorf("layout.yaml content mismatch:\n%s", layout)
	}
	// Package IDL is rendered onto the kitex default path.
	idlBody, err := os.ReadFile(filepath.Join(opts.Dir, "idl", "demo.proto"))
	if err != nil {
		t.Fatalf("rendered idl missing: %v", err)
	}
	if !strings.Contains(string(idlBody), "service demo {}") {
		t.Errorf("service name not rendered:\n%s", idlBody)
	}
}

// seedHertzTemplatePackage builds a minimal hertz template package with a
// root-level Makefile (containing template variables) and a non-root conf
// template, so the overlay can be checked to write root templates and leave
// non-root ones to the later template.Apply() pass.
func seedHertzTemplatePackage(t *testing.T) string {
	t.Helper()
	pkg := t.TempDir()
	os.MkdirAll(filepath.Join(pkg, "hertz-template"), 0o755)
	os.WriteFile(filepath.Join(pkg, "hertz-template", "makefile_yaml.yaml"), []byte(
		"path: Makefile\nupdate_behavior:\n  type: cover\nbody: |\n"+
			"  APP_NAME = {{.ServiceName | ToLower}}-http\n"+
			"  MODULE   = {{.Module}}\n"+
			"  CUSTOM_MAKEFILE = from-package\n"), 0o644)
	os.WriteFile(filepath.Join(pkg, "hertz-template", "conf_go.yaml"), []byte(
		"path: internal/base/conf/conf.go\nupdate_behavior:\n  type: cover\nbody: |\n"+
			"  package conf\n"), 0o644)
	return pkg
}

func TestGenerateTemplatePackageHertzRootOverlay(t *testing.T) {
	opts := Options{
		Name:          "demo",
		Module:        "github.com/acme/demo",
		Kind:          manifest.KindHertz,
		Dir:           filepath.Join(t.TempDir(), "svc"),
		TemplateDir:   seedHertzTemplatePackage(t),
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.0.0-test",
		Now:           time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		NoGenerate:    true,
		Runner:        &fakeRunner{},
	}
	if _, err := Generate(context.Background(), opts); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// The package's root-level Makefile must win over the embedded one and be
	// rendered with the target module/service-name variables.
	mk, err := os.ReadFile(filepath.Join(opts.Dir, "Makefile"))
	if err != nil {
		t.Fatalf("project Makefile missing: %v", err)
	}
	s := string(mk)
	if !strings.Contains(s, "APP_NAME = demo-http") {
		t.Errorf("service name not rendered:\n%s", s)
	}
	if !strings.Contains(s, "MODULE   = github.com/acme/demo") {
		t.Errorf("module not rendered:\n%s", s)
	}
	if !strings.Contains(s, "CUSTOM_MAKEFILE = from-package") {
		t.Errorf("expected package root Makefile to overwrite embedded one:\n%s", s)
	}
	// Non-root package templates are applied later by template.Apply(); they
	// must not be written to the project root at scaffold time.
	if _, err := os.Stat(filepath.Join(opts.Dir, "internal", "base", "conf", "conf.go")); err == nil {
		t.Error("non-root template should not be written at scaffold time")
	}
}
