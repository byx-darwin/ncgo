package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleWorkspace() *Workspace {
	return &Workspace{
		Ncgo:   Meta{Version: "0.3.0-dev", AssetsVersion: "test-assets"},
		Mode:   ModeMicro,
		Name:   "shop",
		Module: "github.com/acme/shop",
		Services: []WorkspaceService{{
			Name: "shop-rpc", Kind: KindKitex, Dir: "services/shop-rpc",
		}},
	}
}

func TestWorkspaceSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	in := sampleWorkspace()
	if err := SaveWorkspace(root, in); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	if _, err := os.Stat(WorkspacePath(root)); err != nil {
		t.Fatalf("workspace file not created: %v", err)
	}
	out, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if out.Name != in.Name || out.Module != in.Module || len(out.Services) != 1 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
	if out.GeneratedAt.IsZero() {
		t.Errorf("GeneratedAt should be stamped on SaveWorkspace")
	}
}

func TestWorkspaceSavePreservesGeneratedAt(t *testing.T) {
	root := t.TempDir()
	want := time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC)
	in := sampleWorkspace()
	in.GeneratedAt = want
	if err := SaveWorkspace(root, in); err != nil {
		t.Fatalf("SaveWorkspace: %v", err)
	}
	out, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	if !out.GeneratedAt.Equal(want) {
		t.Errorf("GeneratedAt = %v, want %v", out.GeneratedAt, want)
	}
}

func TestWorkspacePath(t *testing.T) {
	got := WorkspacePath("/x/y")
	want := filepath.Join("/x/y", WorkspaceFileName)
	if got != want {
		t.Errorf("WorkspacePath = %q, want %q", got, want)
	}
}

func TestWorkspaceValidateRejectsBadInputs(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Workspace)
		want string
	}{
		{"missing mode", func(w *Workspace) { w.Mode = "" }, "mode is required"},
		{"bad mode", func(w *Workspace) { w.Mode = ModeMono }, "mode \"mono\""},
		{"missing name", func(w *Workspace) { w.Name = "" }, "name is required"},
		{"missing module", func(w *Workspace) { w.Module = "" }, "module is required"},
		{"missing version", func(w *Workspace) { w.Ncgo.Version = "" }, "ncgo.version"},
		{"bad service kind", func(w *Workspace) { w.Services[0].Kind = "grpc" }, "services[0].kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := sampleWorkspace()
			tc.mut(w)
			err := w.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error = %v, want containing %q", err, tc.want)
			}
		})
	}
}
