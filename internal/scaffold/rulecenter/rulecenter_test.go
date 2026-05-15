package rulecenter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeHertzManifest(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".ncgo"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `ncgo:
  version: 0.0.0-test
  assets_version: test
mode: mono
module: github.com/x/demo
service:
  name: demo
  kind: hertz
  with_database: true
  idl: idl/app/demo.proto
`
	if err := os.WriteFile(filepath.Join(dir, ".ncgo", "manifest.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also create a minimal conf/dev/conf.yaml so updateConfForRuleCenter works
	if err := os.MkdirAll(filepath.Join(dir, "conf", "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := `env: dev
rate_limit:
  enabled: true
  source:
    type: config
    cache_ttl_seconds: 60
  backend: memory
`
	if err := os.WriteFile(filepath.Join(dir, "conf", "dev", "conf.yaml"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeKitexManifest(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".ncgo"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `ncgo:
  version: 0.0.0-test
  assets_version: test
mode: mono
module: github.com/x/demo
service:
  name: demo
  kind: kitex
  with_database: true
  idl: idl/demo.proto
`
	if err := os.WriteFile(filepath.Join(dir, ".ncgo", "manifest.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddRequiresAddr(t *testing.T) {
	_, err := Add(Options{Addr: ""})
	if err == nil {
		t.Fatal("expected error for missing addr")
	}
	if !strings.Contains(err.Error(), "addr is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddRejectsKitex(t *testing.T) {
	dir := t.TempDir()
	makeKitexManifest(t, dir)

	_, err := Add(Options{Root: dir, Addr: "localhost:8888"})
	if err == nil {
		t.Fatal("expected error for kitex service")
	}
	if !strings.Contains(err.Error(), "only supported for Hertz") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteRuleCenterClientForceFlag(t *testing.T) {
	dir := t.TempDir()
	makeHertzManifest(t, dir)

	// First call should succeed
	res, err := Add(Options{Root: dir, Addr: "localhost:8888"})
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if len(res.WrittenPaths) == 0 {
		t.Fatal("expected files to be written")
	}

	// Second call without --force should fail
	_, err = Add(Options{Root: dir, Addr: "localhost:8888"})
	if err == nil {
		t.Fatal("expected error for existing file without --force")
	}

	// Second call with --force should succeed
	res2, err := Add(Options{Root: dir, Addr: "localhost:9999", Force: true})
	if err != nil {
		t.Fatalf("Add with --force: %v", err)
	}
	if len(res2.WrittenPaths) == 0 {
		t.Fatal("expected files to be written with --force")
	}
}

func TestWriteRuleCenterClientDryRun(t *testing.T) {
	dir := t.TempDir()
	makeHertzManifest(t, dir)

	res, err := Add(Options{Root: dir, Addr: "localhost:8888", DryRun: true})
	if err != nil {
		t.Fatalf("Add dry-run: %v", err)
	}
	if !res.DryRun {
		t.Fatal("expected DryRun to be true in result")
	}
	if len(res.WrittenPaths) != 0 {
		t.Fatalf("expected no files written in dry-run, got %v", res.WrittenPaths)
	}

	// Verify file was not actually created
	if _, err := os.Stat(filepath.Join(dir, "internal", "pkg", "middleware", "rule_center_client.go")); err == nil {
		t.Fatal("file should not exist in dry-run mode")
	}
}

func TestUpdateConfForRuleCenter(t *testing.T) {
	conf := `env: dev
rate_limit:
  enabled: true
  source:
    type: config
    cache_ttl_seconds: 60
  backend: memory
`
	dir := t.TempDir()
	confPath := filepath.Join(dir, "conf.yaml")
	if err := os.WriteFile(confPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := updateConfForRuleCenter(confPath, "localhost:8888"); err != nil {
		t.Fatalf("updateConfForRuleCenter: %v", err)
	}

	b, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)

	if !strings.Contains(content, "type: rule_center") {
		t.Error("expected source type to be rule_center")
	}
	if !strings.Contains(content, `address: "localhost:8888"`) {
		t.Error("expected rule_center address to be set")
	}
	if !strings.Contains(content, "query_timeout_milliseconds: 200") {
		t.Error("expected query_timeout_milliseconds to be set")
	}
}
