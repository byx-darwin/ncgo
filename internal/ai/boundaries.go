package ai

import "strings"

// boundaryEntry describes one file path in the edit-boundaries table.
type boundaryEntry struct {
	Path   string
	Reason string
}

// EditBoundaries returns the may-edit and do-not-edit file tables for a
// sync source. The may-edit list is derived from the manifest domains;
// the do-not-edit list is static (generated code paths).
func EditBoundaries(source syncSource) (mayEdit, doNotEdit []boundaryEntry) {
	mayEdit = []boundaryEntry{
		{Path: "internal/handler/", Reason: "HTTP/RPC handlers"},
		{Path: "internal/adapter/", Reason: "Outbound RPC clients"},
		{Path: "internal/pkg/middleware/", Reason: "Custom middleware"},
		{Path: "internal/base/server/server.go", Reason: "DI wiring"},
	}
	doNotEdit = []boundaryEntry{
		{Path: "internal/base/data/", Reason: "Generated data layer"},
		{Path: "internal/db/gen/", Reason: "sqlc-generated code"},
		{Path: "internal/router/register.go", Reason: "Generated routes"},
		{Path: "internal/pb/", Reason: "Protobuf generated types"},
		{Path: "CLAUDE.md, AGENTS.md", Reason: "Managed by ncgo ai sync"},
	}
	var domains []string
	switch source.Scope {
	case syncScopeService:
		domains = source.Service.Domains
	case syncScopeWorkspace:
		for _, svc := range source.WorkspaceServices {
			domains = append(domains, svc.Name)
		}
	}
	for _, d := range domains {
		mayEdit = append(mayEdit,
			boundaryEntry{Path: "internal/usecase/" + d + "/", Reason: "Business logic"},
			boundaryEntry{Path: "internal/repository/" + d + "/", Reason: "Data access implementation"},
		)
		doNotEdit = append(doNotEdit,
			boundaryEntry{Path: "internal/usecase/" + d + "/" + d + ".go between anchors", Reason: "Generated method stubs"},
		)
	}
	return mayEdit, doNotEdit
}

// RenderBoundaries renders the may-edit / do-not-edit tables as markdown.
func RenderBoundaries(mayEdit, doNotEdit []boundaryEntry) string {
	var b strings.Builder
	b.WriteString("## Boundaries\n\n")
	b.WriteString("### You may edit\n\n")
	b.WriteString("| Path | Purpose |\n")
	b.WriteString("|------|---------|\n")
	for _, e := range mayEdit {
		b.WriteString("| `" + e.Path + "` | " + e.Reason + " |\n")
	}
	b.WriteString("\n### Do not edit\n\n")
	b.WriteString("| Path | Why |\n")
	b.WriteString("|------|-----|\n")
	for _, e := range doNotEdit {
		b.WriteString("| `" + e.Path + "` | " + e.Reason + " |\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
