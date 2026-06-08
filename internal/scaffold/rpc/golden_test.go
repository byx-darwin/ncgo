package rpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/testutil/golden"
)

// goldenOpts returns deterministic options for RPC golden tests.
func goldenOpts(t *testing.T) (string, Options) {
	t.Helper()
	root := seedWorkspace(t, nil)
	return root, Options{
		Root:          root,
		Name:          "user-rpc",
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.3.0-test",
		NoGenerate:    true,
		Now:           time.Date(2026, 4, 29, 8, 30, 0, 0, time.UTC),
	}
}

// TestGenerateGoldenRPC locks the output tree of a Kitex RPC service added to a micro workspace.
func TestGenerateGoldenRPC(t *testing.T) {
	root, opts := goldenOpts(t)
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	golden.Tree(t, "rpc-default", res.ServiceDir)
	// Verify workspace updated
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if len(w.Services) != 1 {
		t.Fatalf("expected 1 service in workspace, got %d", len(w.Services))
	}
	svc := w.Services[0]
	if svc.Name != opts.Name || svc.Kind != manifest.KindKitex {
		t.Errorf("workspace service mismatch: %+v", svc)
	}
	// Verify key files exist
	for _, p := range []string{".ncgo/manifest.yaml", ".pre-commit-config.yaml", "idl/userrpc.proto", "Dockerfile", "compose.yaml", "template/kitex-template/main.yaml"} {
		if _, err := os.Stat(filepath.Join(res.ServiceDir, p)); err != nil {
			t.Errorf("service missing %s: %v", p, err)
		}
	}
}

// TestGenerateGoldenRPCWithPreset locks the RPC service output with rule-center preset.
func TestGenerateGoldenRPCWithPreset(t *testing.T) {
	root := seedWorkspace(t, nil)
	opts := Options{
		Root:          root,
		Name:          "user-rpc",
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.3.0-test",
		NoGenerate:    true,
		Preset:        "rule-center",
		Now:           time.Date(2026, 4, 29, 8, 30, 0, 0, time.UTC),
	}
	res, err := Add(context.Background(), opts)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	golden.Tree(t, "rpc-rulecenter", res.ServiceDir)
}
