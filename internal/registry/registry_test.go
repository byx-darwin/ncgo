package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ncgoexec "github.com/byx-darwin/ncgo/internal/exec"
)

// copyDir recursively copies the contents of src into dst (creating dst if
// needed), mirroring the on-disk result of `git clone <src-mirror> <dst>`.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// fakeGit simulates the git invocations used by ensureCache.
type fakeGit struct {
	fixture  string // directory copied on "clone"
	cloneErr error
	pulls    int
}

func (f *fakeGit) Run(_ context.Context, c ncgoexec.Cmd) (ncgoexec.Result, error) {
	if c.Name != "git" {
		return ncgoexec.Result{}, fmt.Errorf("unexpected command %q", c.Name)
	}
	switch c.Args[0] {
	case "clone":
		if f.cloneErr != nil {
			return ncgoexec.Result{}, f.cloneErr
		}
		dst := c.Args[len(c.Args)-1]
		if err := copyDir(f.fixture, dst); err != nil {
			return ncgoexec.Result{}, err
		}
		return ncgoexec.Result{}, os.MkdirAll(filepath.Join(dst, ".git"), 0o755)
	case "pull":
		f.pulls++
		return ncgoexec.Result{}, nil
	}
	return ncgoexec.Result{}, fmt.Errorf("unexpected git args %v", c.Args)
}

// runnerFunc adapts a plain function to the exec.Runner interface.
type runnerFunc func() (ncgoexec.Result, error)

func (f runnerFunc) Run(_ context.Context, _ ncgoexec.Cmd) (ncgoexec.Result, error) {
	return f()
}

func TestClientListFixture(t *testing.T) {
	fixture := t.TempDir()
	_ = os.MkdirAll(filepath.Join(fixture, "base-kitex"), 0o755)
	_ = os.WriteFile(filepath.Join(fixture, "base-kitex", "template.yaml"),
		[]byte("name: base-kitex\nkind: kitex\ndescription: base\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(fixture, "docs"), 0o755) // noise: no template.yaml

	c := NewClient("https://example.invalid/registry.git", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	entries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "base-kitex" || entries[0].Kind != "kitex" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestClientListSortedAndSkipsDotDirs(t *testing.T) {
	fixture := t.TempDir()
	for _, name := range []string{"zeta", "alpha"} {
		_ = os.MkdirAll(filepath.Join(fixture, name), 0o755)
		_ = os.WriteFile(filepath.Join(fixture, name, "template.yaml"),
			[]byte("name: "+name+"\nkind: hertz\n"), 0o644)
	}
	_ = os.MkdirAll(filepath.Join(fixture, ".git"), 0o755) // dot-dir: skipped

	c := NewClient("u", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	entries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 || entries[0].Name != "alpha" || entries[1].Name != "zeta" {
		t.Errorf("entries = %+v, want sorted alpha,zeta", entries)
	}
}

func TestClientListSkipsBrokenTemplateYaml(t *testing.T) {
	fixture := t.TempDir()
	_ = os.MkdirAll(filepath.Join(fixture, "good"), 0o755)
	_ = os.WriteFile(filepath.Join(fixture, "good", "template.yaml"),
		[]byte("name: good\nkind: hertz\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(fixture, "broken"), 0o755)
	_ = os.WriteFile(filepath.Join(fixture, "broken", "template.yaml"),
		[]byte(": not: valid: yaml: ["), 0o644) // malformed -> skipped, not fatal

	c := NewClient("u", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	entries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "good" {
		t.Errorf("entries = %+v, want only 'good'", entries)
	}
}

func TestClientPullReturnsExistingDir(t *testing.T) {
	fixture := t.TempDir()
	_ = os.MkdirAll(filepath.Join(fixture, "base-hertz"), 0o755)
	_ = os.WriteFile(filepath.Join(fixture, "base-hertz", "template.yaml"),
		[]byte("name: base-hertz\nkind: hertz\n"), 0o644)

	c := NewClient("https://example.invalid/registry.git", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	dir, err := c.Pull(context.Background(), "base-hertz")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if dir != filepath.Join(c.Root, "base-hertz") {
		t.Errorf("dir = %q, want %q", dir, filepath.Join(c.Root, "base-hertz"))
	}
}

func TestClientPullNotFound(t *testing.T) {
	fixture := t.TempDir() // empty registry
	c := NewClient("u", &fakeGit{fixture: fixture})
	c.Root = t.TempDir()
	_, err := c.Pull(context.Background(), "nope")
	if err == nil || !strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("want not-found error, got %v", err)
	}
}

func TestClientRegistryUnavailable(t *testing.T) {
	c := NewClient("u", &fakeGit{cloneErr: errors.New("dial tcp: timeout")})
	c.Root = filepath.Join(t.TempDir(), "cache")
	_, err := c.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Errorf("want registry unavailable, got %v", err)
	}
}

func TestClientGitMissing(t *testing.T) {
	c := NewClient("u", runnerFunc(func() (ncgoexec.Result, error) {
		return ncgoexec.Result{}, &ncgoexec.NotFoundError{Name: "git"}
	}))
	c.Root = filepath.Join(t.TempDir(), "cache")
	_, err := c.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "git is required") {
		t.Errorf("want git-required error, got %v", err)
	}
}

func TestClientExistingCachePulls(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755) // cache already cloned
	_ = os.MkdirAll(filepath.Join(root, "base-hertz"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "base-hertz", "template.yaml"),
		[]byte("name: base-hertz\nkind: hertz\n"), 0o644)

	fake := &fakeGit{}
	c := NewClient("u", fake)
	c.Root = root
	entries, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if fake.pulls != 1 {
		t.Errorf("pulls = %d, want 1 (pull --ff-only on existing cache)", fake.pulls)
	}
	if len(entries) != 1 || entries[0].Name != "base-hertz" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestResolveURLPrecedence(t *testing.T) {
	if got := ResolveURL("https://flag"); got != "https://flag" {
		t.Errorf("flag: %s", got)
	}
	t.Setenv(EnvOverride, "https://env")
	if got := ResolveURL(""); got != "https://env" {
		t.Errorf("env: %s", got)
	}
	_ = os.Unsetenv(EnvOverride)
	if got := ResolveURL(""); got != DefaultURL {
		t.Errorf("default: %s", got)
	}
}

func TestLocalPath(t *testing.T) {
	c := NewClient("u", nil)
	c.Root = filepath.Join(t.TempDir(), "cache")
	if got := c.LocalPath("base-hertz"); got != filepath.Join(c.Root, "base-hertz") {
		t.Errorf("LocalPath = %q, want %q", got, filepath.Join(c.Root, "base-hertz"))
	}
}

func TestCacheDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	want := filepath.Join(os.Getenv("HOME"), ".ncgo", "template-registry")
	if got != want {
		t.Errorf("CacheDir() = %q, want %q", got, want)
	}
}

func TestMigrateOldCache_MigratesOldCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Simulate old cache with .git marker
	oldBase, _ := os.UserCacheDir()
	oldRoot := filepath.Join(oldBase, "ncgo", "template-registry")
	_ = os.MkdirAll(filepath.Join(oldRoot, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(oldRoot, ".git", "config"), []byte("test"), 0o644)

	newRoot := filepath.Join(home, ".ncgo", "template-registry")
	if err := migrateOldCache(newRoot); err != nil {
		t.Fatalf("migrateOldCache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newRoot, ".git", "config")); err != nil {
		t.Errorf("expected migrated .git/config at new root, got error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(oldRoot, ".git")); err == nil {
		t.Errorf("old cache should have been moved away")
	}
}

func TestMigrateOldCache_NoOldCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	newRoot := filepath.Join(home, ".ncgo", "template-registry")
	if err := migrateOldCache(newRoot); err != nil {
		t.Fatalf("migrateOldCache with no old cache: %v", err)
	}
	// New root should not exist either
	if _, err := os.Stat(newRoot); err == nil {
		t.Errorf("new root should not be created when there's nothing to migrate")
	}
}

func TestMigrateOldCache_NewCacheExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create new cache with .git
	newRoot := filepath.Join(home, ".ncgo", "template-registry")
	_ = os.MkdirAll(filepath.Join(newRoot, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(newRoot, ".git", "config"), []byte("new"), 0o644)

	// Create old cache too
	oldBase, _ := os.UserCacheDir()
	oldRoot := filepath.Join(oldBase, "ncgo", "template-registry")
	_ = os.MkdirAll(filepath.Join(oldRoot, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(oldRoot, ".git", "config"), []byte("old"), 0o644)

	if err := migrateOldCache(newRoot); err != nil {
		t.Fatalf("migrateOldCache: %v", err)
	}
	// New cache should be preserved
	data, _ := os.ReadFile(filepath.Join(newRoot, ".git", "config"))
	if string(data) != "new" {
		t.Errorf("new cache should not be overwritten, got %q", string(data))
	}
	// Old cache should still exist (not moved)
	if _, err := os.Stat(oldRoot); err != nil {
		t.Errorf("old cache should still exist when new cache takes precedence")
	}
}

func TestMigrateOldCache_SamePath(t *testing.T) {
	// When oldRoot == newRoot, should be a no-op
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Set up so UserCacheDir returns the same as home (simulating edge case)
	// This is hard to simulate directly, so just verify no panic
	root := filepath.Join(home, ".ncgo", "template-registry")
	if err := migrateOldCache(root); err != nil {
		t.Fatalf("migrateOldCache same path: %v", err)
	}
}
