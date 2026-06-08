package micro

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// goldenOpts returns deterministic options for micro workspace golden tests.
func goldenOpts(t *testing.T) Options {
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

// TestGenerateGoldenMicro locks the output tree of a micro workspace.
func TestGenerateGoldenMicro(t *testing.T) {
	opts := goldenOpts(t)
	res, err := Generate(opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden.Tree(t, "micro-default", res.Dir)
	// Verify workspace
	w, err := manifest.LoadWorkspace(res.Dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if w.Mode != manifest.ModeMicro || w.Name != opts.Name || w.Module != opts.Module {
		t.Errorf("workspace mismatch: %+v", w)
	}
	if len(w.Services) != 0 {
		t.Errorf("new micro workspace should start with no services, got %d", len(w.Services))
	}
	// Verify key files
	for _, p := range []string{"ncgo.workspace", "README.md", "compose.yaml", ".pre-commit-config.yaml", "services/.gitkeep"} {
		if _, err := os.Stat(filepath.Join(res.Dir, p)); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
}
