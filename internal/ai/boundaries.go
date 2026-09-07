package ai

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// boundaryEntry describes one file path in the edit-boundaries table.
type boundaryEntry struct {
	Path   string
	Reason string
}

// EditBoundaries returns the may-edit and do-not-edit file tables for a
// sync source. The may-edit list is derived from the manifest domains; for
// service scope it is unioned with domain directories that still exist on
// disk under internal/repository/ and internal/usecase/ even though they
// were removed from the manifest, so the table doesn't silently drop a row
// for a directory a hand-written caller still depends on. The do-not-edit
// list is static plus generated method-stub markers per rendered domain.
//
// root is the filesystem root to check for service scope (pass opts.Root).
// It is ignored for workspace scope, which renders one row per workspace
// service name rather than per domain and has no single root against which
// to check domain directories.
func EditBoundaries(source syncSource, root string) (mayEdit, doNotEdit []boundaryEntry) {
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
	var extra []string
	switch source.Scope {
	case syncScopeService:
		domains = source.Service.Domains
		extra = domainsOnDiskNotInManifest(root, domains)
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
	for _, d := range extra {
		mayEdit = append(mayEdit,
			boundaryEntry{Path: "internal/usecase/" + d + "/", Reason: "Business logic (not in manifest; verify manual usage)"},
			boundaryEntry{Path: "internal/repository/" + d + "/", Reason: "Data access implementation (not in manifest; verify manual usage)"},
		)
	}
	return mayEdit, doNotEdit
}

// domainsOnDiskNotInManifest returns domain names, sorted, that have an
// existing internal/repository/<name>/ or internal/usecase/<name>/
// directory under root but are absent from manifestDomains. Returns nil
// when root is empty (e.g. tests constructing a syncSource without a real
// filesystem root) or when neither directory exists/can be read.
func domainsOnDiskNotInManifest(root string, manifestDomains []string) []string {
	if root == "" {
		return nil
	}
	known := make(map[string]bool, len(manifestDomains))
	for _, d := range manifestDomains {
		known[d] = true
	}
	found := make(map[string]bool)
	var extra []string
	for _, sub := range []string{"internal/repository", "internal/usecase"} {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(sub)))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || known[e.Name()] || found[e.Name()] {
				continue
			}
			found[e.Name()] = true
			extra = append(extra, e.Name())
		}
	}
	sort.Strings(extra)
	return extra
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
