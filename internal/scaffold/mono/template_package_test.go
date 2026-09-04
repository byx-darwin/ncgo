package mono

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"gopkg.in/yaml.v3"
)

// seedTemplatePackage builds a minimal kitex template package.
func seedTemplatePackage(t *testing.T, withIDL bool) string {
	t.Helper()
	pkg := t.TempDir()
	_ = os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "kitex-template", "main_go.yaml"), []byte(
		"path: main.go\nupdate_behavior:\n  type: cover\nbody: |\n  package main\n"), 0o644)
	if withIDL {
		_ = os.MkdirAll(filepath.Join(pkg, "idl"), 0o755)
		_ = os.WriteFile(filepath.Join(pkg, "idl", "{{ToLower .ServiceName}}.proto"),
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
	if !strings.Contains(string(body), "service Demo {}") {
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
	_ = os.WriteFile(filepath.Join(pkg, "template.yaml"), []byte("kind: hertz\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "kitex-template", "a.yaml"), []byte("path: main.go\n"), 0o644)
	opts := templatePkgOptions(t, pkg)
	_, err := Generate(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "does not match expected kind") {
		t.Errorf("want kind mismatch error, got %v", err)
	}
}

func TestGenerateTemplatePackageParseError(t *testing.T) {
	pkg := t.TempDir()
	_ = os.WriteFile(filepath.Join(pkg, "template.yaml"), []byte("{{invalid"), 0o644)
	_ = os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "kitex-template", "a.yaml"), []byte("path: main.go\n"), 0o644)
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
	_ = os.WriteFile(filepath.Join(pkg, "template.yaml"), []byte("name: rule-center-like\nkind: kitex\nskip_default_templates:\n  - handler.yaml\n  - server.yaml\n  - usecase.yaml\n  - repository.yaml\n"), 0644)
	_ = os.MkdirAll(filepath.Join(pkg, "kitex-template"), 0755)
	_ = os.WriteFile(filepath.Join(pkg, "kitex-template", "test.yaml"), []byte("path: main.go\nupdate_behavior:\n  type: cover\nbody: |\n  package main\n"), 0644)
	_ = os.WriteFile(filepath.Join(pkg, "kitex-template", "ratelimit_shared_resolver.yaml"), []byte("path: internal/service/ratelimit/resolver.go\nupdate_behavior:\n  type: cover\nbody: |\n  package ratelimit\n"), 0644)
	_ = os.MkdirAll(filepath.Join(pkg, "schema"), 0755)
	_ = os.WriteFile(filepath.Join(pkg, "schema", "000002_rate_limit_rules.sql"), []byte("-- {{.Module}}\nCREATE TABLE rate_limit_rules (id bigint);"), 0644)
	_ = os.WriteFile(filepath.Join(pkg, "layout.yaml"), []byte("templates:\n  - path: main.go\n"), 0644)
	_ = os.MkdirAll(filepath.Join(pkg, "idl"), 0755)
	_ = os.WriteFile(filepath.Join(pkg, "idl", "demo.proto"), []byte("syntax = \"proto3\";\nservice {{.ServiceName}} {}\n"), 0644)
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
	if !strings.Contains(string(idlBody), "service Demo {}") {
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
	_ = os.MkdirAll(filepath.Join(pkg, "hertz-template"), 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "hertz-template", "makefile_yaml.yaml"), []byte(
		"path: Makefile\nupdate_behavior:\n  type: cover\nbody: |\n"+
			"  APP_NAME = {{.ServiceName | ToLower}}-http\n"+
			"  MODULE   = {{.Module}}\n"+
			"  CUSTOM_MAKEFILE = from-package\n"), 0o644)
	_ = os.WriteFile(filepath.Join(pkg, "hertz-template", "conf_go.yaml"), []byte(
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

// seedRuleCenterTemplatePackage builds a kitex template package that mirrors
// the byx-darwin/ncgo-templates rule-center package, sourcing every file from
// the embedded assets so the fixture stays in sync with the preset source of
// truth. It carries skip_default_templates, 8 ratelimit_*.yaml + 5
// ratelimit_shared_*.yaml kitex templates, the full RuleService proto, the
// rate_limit_rules SQL schema, and the rule-center layout.yaml.
func seedRuleCenterTemplatePackage(t *testing.T) string {
	t.Helper()
	srcFS := assets.FS()
	pkg := t.TempDir()

	write := func(rel string, body []byte) {
		t.Helper()
		full := filepath.Join(pkg, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("seed rule-center pkg: mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatalf("seed rule-center pkg: write %s: %v", full, err)
		}
	}

	// template.yaml opts the overlay into MERGE semantics (retained non-skipped
	// embedded defaults + package templates) and drops the default per-layer
	// templates the rule-center preset also drops.
	write("template.yaml", []byte("name: rule-center\nkind: kitex\ndescription: official rule-center template (test fixture)\nversion: 1\nskip_default_templates:\n  - handler.yaml\n  - server.yaml\n  - usecase.yaml\n  - repository.yaml\n"))

	// kitex-template: the 8 embedded ratelimit_*.yaml files.
	for _, name := range []string{
		"ratelimit_handler.yaml",
		"ratelimit_server.yaml",
		"ratelimit_usecase.yaml",
		"ratelimit_repository.yaml",
		"ratelimit_proto.yaml",
		"ratelimit_middleware.yaml",
		"ratelimit_middleware_test.yaml",
		"ratelimit_sqlc_queries.yaml",
	} {
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+name)
		if err != nil {
			t.Fatalf("seed rule-center pkg: read embedded %s: %v", name, err)
		}
		write("kitex-template/"+name, b)
	}

	// kitex-template: the 5 shared ratelimit fragments as ratelimit_shared_*.yaml.
	for asset, target := range map[string]string{
		"ratelimit/resolver.yaml":           "ratelimit_shared_resolver.yaml",
		"ratelimit/resolver_test.yaml":      "ratelimit_shared_resolver_test.yaml",
		"ratelimit/store.yaml":              "ratelimit_shared_store.yaml",
		"ratelimit/store_test.yaml":         "ratelimit_shared_store_test.yaml",
		"ratelimit/rule_center_client.yaml": "ratelimit_shared_rule_center_client.yaml",
	} {
		b, err := fs.ReadFile(srcFS, asset)
		if err != nil {
			t.Fatalf("seed rule-center pkg: read embedded %s: %v", asset, err)
		}
		write("kitex-template/"+target, b)
	}

	// idl/rulecenter.proto: the full RuleService proto (identical to the
	// preset's ratelimit_proto.yaml body, written under the dashless filename
	// the default kitex IDL path uses for a service named "rulecenter").
	proto, err := ruleCenterIDLBody(srcFS)
	if err != nil {
		t.Fatalf("seed rule-center pkg: idl body: %v", err)
	}
	write("idl/rulecenter.proto", proto)

	// schema/000002_rate_limit_rules.sql: pure SQL body extracted from the
	// embedded yaml (the same bytes the preset writes for
	// internal/db/schema/000002_rate_limit_rules.sql).
	schema, err := ruleCenterSchemaBody(srcFS)
	if err != nil {
		t.Fatalf("seed rule-center pkg: schema body: %v", err)
	}
	write("schema/000002_rate_limit_rules.sql", schema)

	// layout.yaml: the rule-center custom layout verbatim.
	layout, err := fs.ReadFile(srcFS, "kitex/layout-rulecenter.yaml")
	if err != nil {
		t.Fatalf("seed rule-center pkg: read embedded layout: %v", err)
	}
	write("layout.yaml", layout)

	return pkg
}

// ruleCenterSchemaBody extracts the pure-SQL body of the embedded rule-center
// schema yaml (path/update_behavior/body block scalar), mirroring the pattern
// ruleCenterIDLBody uses for ratelimit_proto.yaml. The body is static SQL with
// no placeholders.
func ruleCenterSchemaBody(srcFS fs.FS) ([]byte, error) {
	b, err := fs.ReadFile(srcFS, "kitex/schema/000002_rate_limit_rules.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded rule-center schema yaml: %w", err)
	}
	var tpl struct {
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &tpl); err != nil {
		return nil, fmt.Errorf("parse rule-center schema yaml: %w", err)
	}
	return []byte(tpl.Body), nil
}

// TestGenerateTemplateRuleCenterEquivalentToPreset proves that generating with
// `--template <rule-center package>` (B) produces a scaffold equivalent to the
// `--preset rule-center` scaffold (A), modulo the documented IDL filename
// difference: preset writes idl/rule-center.proto, template writes
// idl/rulecenter.proto (same content). The manifest's Service.IDL field
// records the differing path; everything else must match.
func TestGenerateTemplateRuleCenterEquivalentToPreset(t *testing.T) {
	pinned := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	generate := func(name string, preset, templateDir string) (string, Options) {
		opts := Options{
			Name:          "rulecenter",
			Module:        "github.com/x/rulecenter",
			Kind:          manifest.KindKitex,
			Dir:           filepath.Join(t.TempDir(), "rulecenter"),
			Preset:        preset,
			TemplateDir:   templateDir,
			AssetsVersion: "test-assets",
			NCGOVersion:   "0.0.0-test",
			Now:           pinned,
			NoGenerate:    true,
			Runner:        &fakeRunner{},
		}
		res, err := Generate(context.Background(), opts)
		if err != nil {
			t.Fatalf("Generate %s: %v", name, err)
		}
		return res.Dir, opts
	}

	dirA, _ := generate("preset", "rule-center", "")
	dirB, _ := generate("template", "", seedRuleCenterTemplatePackage(t))

	// 1. kitex-template file set is IDENTICAL (sorted names), proving the merge
	// semantics retained the same defaults and overlaid the same package files.
	namesA := sortedTemplateNames(t, filepath.Join(dirA, "template", "kitex-template"))
	namesB := sortedTemplateNames(t, filepath.Join(dirB, "template", "kitex-template"))
	if len(namesA) == 0 {
		t.Fatal("preset kitex-template dir is empty")
	}
	if !slices.Equal(namesA, namesB) {
		t.Errorf("kitex-template file sets differ:\npreset:   %v\ntemplate: %v", namesA, namesB)
	}
	for _, want := range []string{"main.yaml", "client.yaml", "ratelimit_handler.yaml", "ratelimit_shared_resolver.yaml"} {
		if !slices.Contains(namesA, want) {
			t.Errorf("preset kitex-template missing %q", want)
		}
		if !slices.Contains(namesB, want) {
			t.Errorf("template kitex-template missing %q", want)
		}
	}
	// Every kitex-template file must also be byte-identical, not just the same
	// name set (all sources are embedded assets, so this holds by construction).
	assertDirEqual(t, filepath.Join(dirA, "template", "kitex-template"), filepath.Join(dirB, "template", "kitex-template"))

	// 2. Skipped default per-layer templates are absent in BOTH.
	for _, skip := range []string{"handler.yaml", "server.yaml", "usecase.yaml", "repository.yaml"} {
		if slices.Contains(namesA, skip) {
			t.Errorf("preset retained skipped default %q", skip)
		}
		if slices.Contains(namesB, skip) {
			t.Errorf("template retained skipped default %q", skip)
		}
	}

	// 3. Custom layout is identical.
	assertFileEqual(t, dirA, dirB, "template/layout.yaml")

	// 4. Schema file is identical and non-empty.
	assertFileEqual(t, dirA, dirB, "internal/db/schema/000002_rate_limit_rules.sql")
	schema := readTreeFile(t, dirA, "internal/db/schema/000002_rate_limit_rules.sql")
	if !strings.Contains(string(schema), "CREATE TABLE") {
		t.Errorf("preset schema missing CREATE TABLE:\n%s", schema)
	}

	// 5. IDL content is identical under the two different filenames, and both
	// carry the RuleService RPC surface.
	idlA := readTreeFile(t, dirA, "idl/rule-center.proto")
	idlB := readTreeFile(t, dirB, "idl/rulecenter.proto")
	if !bytes.Equal(idlA, idlB) {
		t.Errorf("IDL content differs (preset idl/rule-center.proto vs template idl/rulecenter.proto)\n--- preset ---\n%s\n--- template ---\n%s", idlA, idlB)
	}
	for name, body := range map[string][]byte{"preset": idlA, "template": idlB} {
		if !strings.Contains(string(body), "service RuleService") {
			t.Errorf("%s IDL missing service RuleService:\n%s", name, body)
		}
		if !strings.Contains(string(body), "rpc GetRule") {
			t.Errorf("%s IDL missing rpc GetRule:\n%s", name, body)
		}
	}

	// 6. The ONLY manifest difference is Service.IDL.
	mA, err := manifest.Load(dirA)
	if err != nil {
		t.Fatalf("load preset manifest: %v", err)
	}
	mB, err := manifest.Load(dirB)
	if err != nil {
		t.Fatalf("load template manifest: %v", err)
	}
	if mA.Ncgo != mB.Ncgo {
		t.Errorf("manifest ncgo meta differs: preset=%+v template=%+v", mA.Ncgo, mB.Ncgo)
	}
	if mA.Mode != mB.Mode {
		t.Errorf("manifest mode differs: %q vs %q", mA.Mode, mB.Mode)
	}
	if mA.Module != mB.Module {
		t.Errorf("manifest module differs: %q vs %q", mA.Module, mB.Module)
	}
	if mA.Service.Name != mB.Service.Name {
		t.Errorf("manifest service name differs: %q vs %q", mA.Service.Name, mB.Service.Name)
	}
	if mA.Service.Kind != mB.Service.Kind {
		t.Errorf("manifest service kind differs: %q vs %q", mA.Service.Kind, mB.Service.Kind)
	}
	if mA.Service.WithDatabase != mB.Service.WithDatabase {
		t.Errorf("manifest with_database differs: %v vs %v", mA.Service.WithDatabase, mB.Service.WithDatabase)
	}
	if !mA.GeneratedAt.Equal(mB.GeneratedAt) {
		t.Errorf("manifest generated_at differs: %v vs %v", mA.GeneratedAt, mB.GeneratedAt)
	}
	if want, got := "idl/rule-center.proto", mA.Service.IDL; got != want {
		t.Errorf("preset manifest idl = %q, want %q", got, want)
	}
	if want, got := "idl/rulecenter.proto", mB.Service.IDL; got != want {
		t.Errorf("template manifest idl = %q, want %q", got, want)
	}
}

// TestGenerateTemplatePackageIDLNameCoupling proves that a template package whose
// IDL has a FIXED filename (rule-center ships idl/rulecenter.proto) defines the
// project IDL path even when the service name lowercases to something else. Without
// the coupling, Generate would set the project IDL to the default <name>.proto path
// (idl/mysvc.proto for a service named my-svc), write a stale empty placeholder there,
// and point the manifest and kitex command at it while the overlay writes the real
// proto to idl/rulecenter.proto — a broken, miswired scaffold.
func TestGenerateTemplatePackageIDLNameCoupling(t *testing.T) {
	opts := templatePkgOptions(t, seedRuleCenterTemplatePackage(t))
	opts.Name = "my-svc"
	opts.Module = "github.com/acme/my-svc"
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// 1. The package's real proto lands at idl/rulecenter.proto (overlay write).
	idl := readTreeFile(t, opts.Dir, "idl/rulecenter.proto")
	for _, marker := range []string{"service RuleService", "rpc GetRule"} {
		if !strings.Contains(string(idl), marker) {
			t.Errorf("idl/rulecenter.proto missing %q:\n%s", marker, idl)
		}
	}

	// 2. No stale empty placeholder at the default <name>.proto path.
	if _, err := os.Stat(filepath.Join(opts.Dir, "idl", "mysvc.proto")); err == nil {
		t.Error("idl/mysvc.proto placeholder should not exist")
	}

	// 3. The manifest records the package IDL path, not the default <name>.proto.
	m, err := manifest.Load(opts.Dir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if want, got := "idl/rulecenter.proto", m.Service.IDL; got != want {
		t.Errorf("manifest Service.IDL = %q, want %q", got, want)
	}

	// 4. nextSteps (the user-facing kitex command) targets the package IDL only.
	foundReal, foundStale := false, false
	for _, step := range res.NextSteps {
		if strings.Contains(step, "idl/rulecenter.proto") {
			foundReal = true
		}
		if strings.Contains(step, "idl/mysvc.proto") {
			foundStale = true
		}
	}
	if !foundReal {
		t.Errorf("nextSteps missing idl/rulecenter.proto: %v", res.NextSteps)
	}
	if foundStale {
		t.Errorf("nextSteps references stale idl/mysvc.proto: %v", res.NextSteps)
	}
}

// sortedTemplateNames returns the sorted basenames of a template directory.
func sortedTemplateNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names
}

// assertFileEqual asserts the same relative file has identical content under
// two generated roots.
func assertFileEqual(t *testing.T, dirA, dirB, rel string) {
	t.Helper()
	a := readTreeFile(t, dirA, rel)
	b := readTreeFile(t, dirB, rel)
	if !bytes.Equal(a, b) {
		t.Errorf("%s differs between preset and template\n--- preset ---\n%s\n--- template ---\n%s", rel, a, b)
	}
}

// assertDirEqual asserts every file under dirA and dirB is byte-identical.
func assertDirEqual(t *testing.T, dirA, dirB string) {
	t.Helper()
	filesA := sortedTreeFiles(t, dirA)
	filesB := sortedTreeFiles(t, dirB)
	if !slices.Equal(filesA, filesB) {
		t.Errorf("dir file sets differ:\nA: %v\nB: %v", filesA, filesB)
		return
	}
	for _, rel := range filesA {
		a, err := os.ReadFile(filepath.Join(dirA, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		b, err := os.ReadFile(filepath.Join(dirB, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("dir file %s differs", rel)
		}
	}
}

// sortedTreeFiles returns the sorted slash-relative paths of every regular
// file under root.
func sortedTreeFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(out)
	return out
}

// readTreeFile reads a slash-relative file under a generated root.
func readTreeFile(t *testing.T, root, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s under %s: %v", rel, root, err)
	}
	return b
}
