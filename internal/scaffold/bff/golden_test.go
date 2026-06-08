package bff

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// goldenOpts returns deterministic options for BFF golden tests.
func goldenOpts(t *testing.T) (string, Options) {
	t.Helper()
	root := seedWorkspace(t, nil)
	return root, Options{
		Root:          root,
		Name:          "web-bff",
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.3.0-test",
		NoGenerate:    true,
		Now:           time.Date(2026, 4, 29, 8, 30, 0, 0, time.UTC),
	}
}

// TestGenerateGoldenBFF locks the output tree of a BFF service added to a micro workspace.
func TestGenerateGoldenBFF(t *testing.T) {
	root, opts := goldenOpts(t)
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	golden.Tree(t, "bff-default", res.ServiceDir)
	// Verify workspace updated
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(w.Services) != 1 {
		t.Fatalf("expected 1 service in workspace, got %d", len(w.Services))
	}
	svc := w.Services[0]
	if svc.Name != opts.Name || svc.Kind != manifest.KindHertz {
		t.Errorf("workspace service mismatch: %+v", svc)
	}
	svcDir := filepath.Join(root, svc.Dir)
	if svcDir != res.ServiceDir {
		t.Errorf("workspace dir %q != result dir %q", svcDir, res.ServiceDir)
	}
	// Verify key files exist
	for _, p := range []string{".ncgo/manifest.yaml", ".pre-commit-config.yaml", "idl/app/web-bff.proto", "Dockerfile", "compose.yaml"} {
		if _, err := os.Stat(filepath.Join(res.ServiceDir, p)); err != nil {
			t.Errorf("service missing %s: %v", p, err)
		}
	}
}
