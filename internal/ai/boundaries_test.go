package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestEditBoundariesMonoService(t *testing.T) {
	source := syncSource{
		Scope: syncScopeService,
		Service: &manifest.Manifest{
			Domains: []string{"user", "device"},
		},
	}
	mayEdit, doNotEdit := EditBoundaries(source, "")
	if len(mayEdit) == 0 {
		t.Fatal("EditBoundaries returned empty mayEdit for mono service")
	}
	if len(doNotEdit) == 0 {
		t.Fatal("EditBoundaries returned empty doNotEdit for mono service")
	}
	joined := strings.Join([]string{boundariesToStr(mayEdit), boundariesToStr(doNotEdit)}, "")
	if !strings.Contains(joined, "usecase") {
		t.Errorf("boundaries missing usecase paths: %s", joined)
	}
}

func TestEditBoundariesWorkspace(t *testing.T) {
	source := syncSource{
		Scope: syncScopeWorkspace,
		Workspace: &manifest.Workspace{
			Services: []manifest.WorkspaceService{
				{Name: "user-rpc", Kind: "kitex", Dir: "services/user-rpc"},
			},
		},
		WorkspaceServices: []workspaceServiceFacts{
			{Name: "user-rpc", Kind: "kitex", Dir: "services/user-rpc"},
		},
	}
	mayEdit, doNotEdit := EditBoundaries(source, "")
	if len(mayEdit) == 0 {
		t.Fatal("EditBoundaries returned empty mayEdit for workspace")
	}
	if len(doNotEdit) == 0 {
		t.Fatal("EditBoundaries returned empty doNotEdit for workspace")
	}
	joined := strings.Join([]string{boundariesToStr(mayEdit), boundariesToStr(doNotEdit)}, "")
	if !strings.Contains(joined, "internal/usecase/user-rpc/") {
		t.Errorf("boundaries missing workspace-derived usecase path: %s", joined)
	}
}

func TestRenderBoundariesProducesMarkdown(t *testing.T) {
	mayEdit := []boundaryEntry{
		{Path: "internal/usecase/<domain>/", Reason: "Business logic"},
	}
	doNotEdit := []boundaryEntry{
		{Path: "internal/db/gen/", Reason: "sqlc-generated code"},
	}
	got := RenderBoundaries(mayEdit, doNotEdit)
	for _, want := range []string{"## Boundaries", "You may edit", "Do not edit", "usecase", "db/gen"} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderBoundaries missing %q:\n%s", want, got)
		}
	}
}

func TestEditBoundariesUnionsDomainStillOnDiskButNotInManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "repository", "role"), 0o755); err != nil {
		t.Fatalf("mkdir repository/role: %v", err)
	}
	source := syncSource{
		Scope: syncScopeService,
		Service: &manifest.Manifest{
			Domains: []string{"device"}, // "role" was removed from the manifest
		},
	}
	mayEdit, _ := EditBoundaries(source, root)
	var found *boundaryEntry
	for i := range mayEdit {
		if mayEdit[i].Path == "internal/repository/role/" {
			found = &mayEdit[i]
		}
	}
	if found == nil {
		t.Fatalf("expected internal/repository/role/ row despite manifest removal; got %+v", mayEdit)
	}
	if !strings.Contains(found.Reason, "not in manifest") {
		t.Errorf("expected Reason to flag the row as not in manifest, got %q", found.Reason)
	}
}

func TestEditBoundariesDoesNotDuplicateManifestDomains(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "repository", "device"), 0o755); err != nil {
		t.Fatalf("mkdir repository/device: %v", err)
	}
	source := syncSource{
		Scope: syncScopeService,
		Service: &manifest.Manifest{
			Domains: []string{"device"},
		},
	}
	mayEdit, _ := EditBoundaries(source, root)
	count := 0
	for _, e := range mayEdit {
		if e.Path == "internal/repository/device/" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one internal/repository/device/ row, got %d in %+v", count, mayEdit)
	}
}

func TestEditBoundariesWorkspaceScopeIgnoresFilesystem(t *testing.T) {
	root := t.TempDir() // deliberately empty — no internal/repository dirs at all
	source := syncSource{
		Scope: syncScopeWorkspace,
		WorkspaceServices: []workspaceServiceFacts{
			{Name: "user-rpc", Kind: "kitex", Dir: "services/user-rpc"},
		},
	}
	mayEdit, _ := EditBoundaries(source, root)
	var found bool
	for _, e := range mayEdit {
		if e.Path == "internal/usecase/user-rpc/" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected workspace-scope behavior unchanged (service-name-based row), got %+v", mayEdit)
	}
}

func boundariesToStr(entries []boundaryEntry) string {
	var parts []string
	for _, e := range entries {
		parts = append(parts, e.Path)
	}
	return strings.Join(parts, ",")
}
