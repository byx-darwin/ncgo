package doctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func seedComposeWorkspace(t *testing.T, goMod string) (root string, w *manifest.Workspace) {
	t.Helper()
	root = t.TempDir()
	w = &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/x/commerce",
		Services: []manifest.WorkspaceService{
			{Name: "authority", Kind: manifest.KindKitex, Dir: "services/authority"},
			{Name: "orders", Kind: manifest.KindKitex, Dir: "services/orders"},
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.SaveWorkspace(root, w); err != nil {
		t.Fatalf("save workspace: %v", err)
	}
	authorityDir := filepath.Join(root, "services", "authority")
	ordersDir := filepath.Join(root, "services", "orders")
	if err := os.MkdirAll(authorityDir, 0o755); err != nil {
		t.Fatalf("mkdir authority: %v", err)
	}
	if err := os.MkdirAll(ordersDir, 0o755); err != nil {
		t.Fatalf("mkdir orders: %v", err)
	}
	if goMod != "" {
		if err := os.WriteFile(filepath.Join(authorityDir, "go.mod"), []byte(goMod), 0o644); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
	}
	return root, w
}

func writeComposeYAML(t *testing.T, root, context, dockerfile string) {
	t.Helper()
	body := "name: commerce\nservices:\n  authority:\n    build:\n      context: " + context + "\n      dockerfile: " + dockerfile + "\n"
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write compose.yaml: %v", err)
	}
}

func writeAuthorityDockerfile(t *testing.T, root string, copyLines ...string) {
	t.Helper()
	body := "FROM golang:1.26.5-alpine AS builder\n"
	for _, line := range copyLines {
		body += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, "services", "authority", "Dockerfile"), []byte(body), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
}

func TestComposeConsistencyChecksNoReplaceEmitsNoChecks(t *testing.T) {
	root, w := seedComposeWorkspace(t, "")
	checks := composeConsistencyChecks(root, w)
	if len(checks) != 0 {
		t.Fatalf("expected no checks, got %+v", checks)
	}
}

func TestComposeConsistencyChecksCorrectComposeAndDockerfilePass(t *testing.T) {
	goMod := "module github.com/x/commerce/services/authority\n\ngo 1.25\n\nreplace github.com/x/commerce/services/orders => ../orders\n"
	root, w := seedComposeWorkspace(t, goMod)
	writeComposeYAML(t, root, ".", "services/authority/Dockerfile")
	writeAuthorityDockerfile(t, root, "COPY services/orders/ services/orders/", "COPY services/authority/ services/authority/")
	checks := composeConsistencyChecks(root, w)
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %+v", checks)
	}
	for _, c := range checks {
		if !c.OK {
			t.Errorf("check %s failed: %s", c.ID, c.Message)
		}
	}
}

func TestComposeConsistencyChecksStaleComposeContextFails(t *testing.T) {
	goMod := "module github.com/x/commerce/services/authority\n\ngo 1.25\n\nreplace github.com/x/commerce/services/orders => ../orders\n"
	root, w := seedComposeWorkspace(t, goMod)
	writeComposeYAML(t, root, "./services/authority", "Dockerfile")
	writeAuthorityDockerfile(t, root, "COPY services/orders/ services/orders/", "COPY services/authority/ services/authority/")
	checks := composeConsistencyChecks(root, w)
	var found bool
	for _, c := range checks {
		if c.ID == "compose.context.authority" {
			found = true
			if c.OK {
				t.Errorf("expected compose.context.authority to fail")
			}
		}
	}
	if !found {
		t.Fatalf("expected compose.context.authority check, got %+v", checks)
	}
}

func TestComposeConsistencyChecksMissingDockerfileCopyFails(t *testing.T) {
	goMod := "module github.com/x/commerce/services/authority\n\ngo 1.25\n\nreplace github.com/x/commerce/services/orders => ../orders\n"
	root, w := seedComposeWorkspace(t, goMod)
	writeComposeYAML(t, root, ".", "services/authority/Dockerfile")
	writeAuthorityDockerfile(t, root, "COPY services/authority/ services/authority/")
	checks := composeConsistencyChecks(root, w)
	var found bool
	for _, c := range checks {
		if c.ID == "compose.dockerfile.authority" {
			found = true
			if c.OK {
				t.Errorf("expected compose.dockerfile.authority to fail")
			}
		}
	}
	if !found {
		t.Fatalf("expected compose.dockerfile.authority check, got %+v", checks)
	}
}
