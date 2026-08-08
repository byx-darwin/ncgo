package template

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPackageHappy(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: d\nversion: \"1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "main_go.yaml"), []byte("path: main.go\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "idl"), 0o755)
	os.WriteFile(filepath.Join(dir, "idl", "svc.proto"), []byte("syntax = \"proto3\";\n"), 0o644)

	pkg, err := LoadPackage(dir, "kitex")
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	if !pkg.HasMeta || pkg.Meta.Name != "base-kitex" || pkg.Meta.Kind != "kitex" {
		t.Errorf("meta = %+v", pkg.Meta)
	}
	if len(pkg.Templates) != 1 || len(pkg.IDLs) != 1 {
		t.Errorf("templates=%v idls=%v", pkg.Templates, pkg.IDLs)
	}
}

func TestLoadPackageNoMeta(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "hertz-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "hertz-template", "main_go.yaml"), []byte("path: main.go\n"), 0o644)
	pkg, err := LoadPackage(dir, "hertz")
	if err != nil || pkg.HasMeta {
		t.Fatalf("expected no-meta success, got %+v, %v", pkg, err)
	}
}

func TestLoadPackageKindMismatch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"), []byte("kind: hertz\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "kitex-template"), 0o755)
	os.WriteFile(filepath.Join(dir, "kitex-template", "a.yaml"), []byte("path: main.go\n"), 0o644)
	_, err := LoadPackage(dir, "kitex")
	if err == nil || !strings.Contains(err.Error(), "has kind") {
		t.Errorf("want kind mismatch error, got %v", err)
	}
}

func TestLoadPackageParseError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "template.yaml"), []byte("{{invalid"), 0o644)
	_, err := LoadPackage(dir, "kitex")
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("want parse error, got %v", err)
	}
}

func TestLoadPackageMissingTemplateDir(t *testing.T) {
	_, err := LoadPackage(t.TempDir(), "kitex")
	if err == nil || !strings.Contains(err.Error(), "has no kitex-template/ directory") {
		t.Errorf("want missing dir error, got %v", err)
	}
}

func TestReadPackageMetaMissing(t *testing.T) {
	_, err := ReadPackageMeta(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want fs.ErrNotExist, got %v", err)
	}
}
