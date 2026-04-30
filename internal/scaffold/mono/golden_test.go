package mono

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// goldenOpts returns a deterministic Options for snapshot tests. Only the
// service name, module, and database flag should vary across snapshots; the
// clock and version metadata are pinned so the manifest hashes the same on
// every machine.
func goldenOpts(t *testing.T, name string, withDB bool) Options {
	t.Helper()
	return Options{
		Name:          name,
		Module:        "github.com/x/" + name,
		Dir:           filepath.Join(t.TempDir(), name),
		WithDatabase:  withDB,
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.0.0-test",
		Now:           time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		NoGenerate:    true,
	}
}

// TestGenerateGoldenDefault locks the entire output tree of `ncgo new --mode
// mono` (without database) so future template tweaks must explicitly bless
// the diff via -update-golden.
func TestGenerateGoldenDefault(t *testing.T) {
	opts := goldenOpts(t, "demo", false)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden.Tree(t, "mono-default", res.Dir)
}

// TestGenerateGoldenWithDatabase covers the postgres path; only data.json is
// expected to differ from the default snapshot, but the whole tree is
// captured so we notice if other files start branching on WithDatabase.
func TestGenerateGoldenWithDatabase(t *testing.T) {
	opts := goldenOpts(t, "demo", true)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden.Tree(t, "mono-with-database", res.Dir)
}

// TestGenerateGoldenKitexDefault locks the deterministic prepare-phase
// output for `ncgo new --mode mono --kind kitex --no-generate`.
func TestGenerateGoldenKitexDefault(t *testing.T) {
	opts := goldenOpts(t, "demo", false)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden.Tree(t, "mono-kitex-default", res.Dir)
}
