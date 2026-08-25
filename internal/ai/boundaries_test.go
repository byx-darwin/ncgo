package ai

import (
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
	mayEdit, doNotEdit := EditBoundaries(source)
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
	}
	mayEdit, doNotEdit := EditBoundaries(source)
	if len(mayEdit) == 0 {
		t.Fatal("EditBoundaries returned empty mayEdit for workspace")
	}
	if len(doNotEdit) == 0 {
		t.Fatal("EditBoundaries returned empty doNotEdit for workspace")
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

func boundariesToStr(entries []boundaryEntry) string {
	var parts []string
	for _, e := range entries {
		parts = append(parts, e.Path)
	}
	return strings.Join(parts, ",")
}
