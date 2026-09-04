package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTemplateDirBothEmpty(t *testing.T) {
	dir, err := ResolveTemplateDir("", "")
	if err != nil || dir != "" {
		t.Errorf("both empty: dir=%q err=%v, want empty/nil", dir, err)
	}
}

func TestResolveTemplateDirLocalPath(t *testing.T) {
	tmp := t.TempDir()
	dir, err := ResolveTemplateDir("", tmp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	abs, _ := filepath.Abs(tmp)
	if dir != abs {
		t.Errorf("dir=%q, want %q", dir, abs)
	}
}

func TestResolveTemplateDirFromCache(t *testing.T) {
	cacheRoot := t.TempDir()
	name := "base-kitex"
	pkgDir := filepath.Join(cacheRoot, name)
	_ = os.MkdirAll(pkgDir, 0o755)
	_ = os.WriteFile(filepath.Join(pkgDir, "template.yaml"), []byte("kind: kitex\n"), 0o644)

	client := NewClient("", nil)
	client.Root = cacheRoot

	dir, err := resolveFromClient(client, name)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if dir != pkgDir {
		t.Errorf("dir=%q, want %q", dir, pkgDir)
	}
}

func TestResolveTemplateDirNotInCache(t *testing.T) {
	cacheRoot := t.TempDir()
	client := NewClient("", nil)
	client.Root = cacheRoot

	_, err := resolveFromClient(client, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not in cache") {
		t.Errorf("want cache miss error, got %v", err)
	}
}

func TestResolveTemplateDirMutualExclusion(t *testing.T) {
	_, err := ResolveTemplateDir("base-kitex", "/some/path")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("want mutual exclusion error, got %v", err)
	}
}
