package ai

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// target describes one rendered artifact.
type target struct {
	Group   string // target group: agents | claude | cursor
	RelPath string
	Render  func(inputs renderInputs) string
}

type renderInputs struct {
	SourceRef          string
	LongBody           string
	ProjectContextBody string
	WorkflowBody       string
	RulesBody          string
	MethodsByDomain    map[string][]string
	ErrorCodes         string
	EditBoundaries     string
	LocalNotes         string
}

// targets returns the static set of artifacts produced by `ncgo ai sync`.
// docs/ai-context/architecture.md (PRD §6 row 4) is intentionally
// deferred until ncgo can scan domain port signatures via go/parser.
func targets() []target {
	return []target{
		{Group: "agents", RelPath: "AGENTS.md", Render: renderAgents},
		{Group: "claude", RelPath: "CLAUDE.md", Render: renderClaude},
		{Group: "claude", RelPath: ".claude/skills/ncgo-dev/SKILL.md", Render: renderNcgoDevSkill},
		{Group: "claude", RelPath: ".claude/generated/project-context.md", Render: renderProjectContext},
		{Group: "cursor", RelPath: ".cursor/rules/ncgo.mdc", Render: renderCursorMDC},
	}
}

// buildInputs assembles the pre-rendered markdown bodies consumed by the
// various ai sync targets. lang selects the embedded workflow/rules asset
// language ("en" or "zh-CN").
func buildInputs(source syncSource, local, lang string) renderInputs {
	inputs := renderInputs{SourceRef: source.SourceRef}
	switch source.Scope {
	case syncScopeWorkspace:
		inputs.LongBody = buildWorkspaceLongBody(source.Workspace, source.WorkspaceServices, source.DesignDoc, local)
		inputs.ProjectContextBody = buildWorkspaceProjectContextBody(source.Workspace, source.WorkspaceServices, source.DesignDoc)
	default:
		inputs.LongBody = buildServiceLongBody(source.Service, source.ServiceWorkspace, source.DesignDoc, local)
		inputs.ProjectContextBody = buildServiceProjectContextBody(source.Service, source.ServiceWorkspace, source.DesignDoc)
	}
	inputs.WorkflowBody = readAIDoc("ncgo-dev-workflow." + lang + ".md")
	inputs.RulesBody = readAIDoc("ncgo-dev-rules." + lang + ".md")
	return inputs
}

// readAIDoc reads one docs/ai/ asset; a missing asset falls back to "" so
// older embedded assets versions still render without a hard failure.
func readAIDoc(name string) string {
	rel := filepath.ToSlash(filepath.Join("docs", "ai", name))
	b, err := fs.ReadFile(assets.FS(), rel)
	if err != nil {
		return ""
	}
	return string(b)
}

func buildServiceLongBody(m *manifest.Manifest, membership *serviceWorkspaceMembership, doc, local string) string {
	var b strings.Builder
	writeProjectFacts(&b, "## Quick Facts", m, membership)
	return b.String()
}

func buildServiceProjectContextBody(m *manifest.Manifest, membership *serviceWorkspaceMembership, doc string) string {
	var b strings.Builder
	writeProjectFacts(&b, "## Project Facts", m, membership)
	writeArchitectureSummary(&b, doc)
	writeRepositoryRules(&b)
	if membership != nil {
		writeGeneratedNotes(&b,
			fmt.Sprintf("This service is registered in micro workspace `%s`; run `ncgo ai sync --root %s` for workspace-level context.", membership.Name, membership.RootRel),
		)
		return b.String()
	}
	writeGeneratedNotes(&b)
	return b.String()
}

func buildWorkspaceLongBody(w *manifest.Workspace, services []workspaceServiceFacts, doc, local string) string {
	var b strings.Builder
	writeWorkspaceFacts(&b, "## Quick Facts", w, services)
	return b.String()
}

func buildWorkspaceProjectContextBody(w *manifest.Workspace, services []workspaceServiceFacts, doc string) string {
	var b strings.Builder
	writeWorkspaceFacts(&b, "## Project Facts", w, services)
	writeArchitectureSummary(&b, doc)
	writeRepositoryRules(&b)
	writeGeneratedNotes(&b,
		"This root is a micro workspace; run `ncgo ai sync --root services/<name>` when you need service-level context.",
	)
	return b.String()
}

// writeProjectFacts emits the manifest-derived facts consumed by AI agents.
func writeProjectFacts(b *strings.Builder, heading string, m *manifest.Manifest, membership *serviceWorkspaceMembership) {
	fmt.Fprintf(b, "%s\n\n", heading)
	fmt.Fprintf(b, "- module: `%s`\n", m.Module)
	fmt.Fprintf(b, "- service.name: `%s`\n", m.Service.Name)
	fmt.Fprintf(b, "- service.kind: `%s`\n", m.Service.Kind)
	fmt.Fprintf(b, "- service.with_database: `%t`\n", m.Service.WithDatabase)
	if m.Service.IDL != "" {
		fmt.Fprintf(b, "- service.idl: `%s`\n", m.Service.IDL)
	}
	fmt.Fprintf(b, "- mode: `%s`\n", m.Mode)
	if len(m.Infra) > 0 {
		fmt.Fprintf(b, "- infra: `[%s]`\n", strings.Join(m.Infra, ", "))
	}
	if len(m.Domains) > 0 {
		fmt.Fprintf(b, "- domains: `[%s]`\n", strings.Join(m.Domains, ", "))
	}
	if membership != nil {
		fmt.Fprintf(b, "- workspace.member: `true`\n")
		fmt.Fprintf(b, "- workspace.name: `%s`\n", membership.Name)
		fmt.Fprintf(b, "- workspace.module: `%s`\n", membership.Module)
		fmt.Fprintf(b, "- workspace.root: `%s`\n", membership.RootRel)
		fmt.Fprintf(b, "- workspace.service_dir: `%s`\n", membership.ServiceDir)
	}
	fmt.Fprintf(b, "- ncgo: `%s` (assets `%s`)\n", m.Ncgo.Version, m.Ncgo.AssetsVersion)
}

func writeWorkspaceFacts(b *strings.Builder, heading string, w *manifest.Workspace, services []workspaceServiceFacts) {
	fmt.Fprintf(b, "%s\n\n", heading)
	fmt.Fprintf(b, "- workspace.name: `%s`\n", w.Name)
	fmt.Fprintf(b, "- workspace.module: `%s`\n", w.Module)
	fmt.Fprintf(b, "- mode: `%s`\n", w.Mode)
	fmt.Fprintf(b, "- services.count: `%d`\n", len(services))
	fmt.Fprintf(b, "- ncgo: `%s` (assets `%s`)\n", w.Ncgo.Version, w.Ncgo.AssetsVersion)
	if len(services) == 0 {
		return
	}
	b.WriteString("- services:\n")
	for _, svc := range services {
		fmt.Fprintf(b, "  - `%s` (`%s`) — dir `%s`, module `%s`", svc.Name, svc.Kind, svc.Dir, svc.Module)
		if svc.IDL != "" {
			fmt.Fprintf(b, ", idl `%s`", svc.IDL)
		}
		fmt.Fprintf(b, ", with_database `%t`\n", svc.WithDatabase)
		if len(svc.Infra) > 0 {
			fmt.Fprintf(b, "    - infra: `[%s]`\n", strings.Join(svc.Infra, ", "))
		}
		if len(svc.Domains) > 0 {
			fmt.Fprintf(b, "    - domains: `[%s]`\n", strings.Join(svc.Domains, ", "))
		}
	}
}

func writeArchitectureSummary(b *strings.Builder, doc string) {
	b.WriteString("\n## Architecture & Built-in Features\n\n")
	summary := firstSectionParagraphs(doc, 2)
	if summary == "" {
		summary = stripFirstHeading(doc)
	}
	b.WriteString(strings.TrimRight(summary, "\n"))
}

func writeRepositoryRules(b *strings.Builder) {
	b.WriteString("\n\n## Repository Rules\n\n")
	b.WriteString("- `.claude/rules/go.md`\n")
	b.WriteString("- `.claude/rules/agent-engineering.md`\n")
}

func writeGeneratedNotes(b *strings.Builder, extra ...string) {
	b.WriteString("\n## Notes\n\n")
	b.WriteString("- This file is generated by `ncgo ai sync`; do not edit by hand.\n")
	b.WriteString("- Put personal notes and machine-specific preferences under `.claude/local/*`.\n")
	for _, line := range extra {
		b.WriteString("- " + line + "\n")
	}
}

func firstSectionParagraphs(doc string, maxParagraphs int) string {
	doc = stripFirstHeading(doc)
	if doc == "" || maxParagraphs <= 0 {
		return ""
	}
	var (
		paragraphs []string
		current    []string
		inSection  bool
	)
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, strings.Join(current, " "))
		current = nil
	}
	for line := range strings.SplitSeq(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "## "):
			if !inSection {
				inSection = true
				continue
			}
			flush()
			return strings.Join(paragraphs[:min(len(paragraphs), maxParagraphs)], "\n\n")
		case !inSection:
			continue
		case strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "```"):
			flush()
			return strings.Join(paragraphs[:min(len(paragraphs), maxParagraphs)], "\n\n")
		case trimmed == "":
			flush()
			if len(paragraphs) >= maxParagraphs {
				return strings.Join(paragraphs[:maxParagraphs], "\n\n")
			}
		default:
			current = append(current, trimmed)
		}
	}
	flush()
	if len(paragraphs) == 0 {
		return ""
	}
	return strings.Join(paragraphs[:min(len(paragraphs), maxParagraphs)], "\n\n")
}

// stripFirstHeading drops the first H1 line so the design doc body can
// be embedded under our own H1 without duplication.
func stripFirstHeading(doc string) string {
	doc = strings.TrimLeft(doc, "\n")
	if !strings.HasPrefix(doc, "# ") {
		return doc
	}
	if _, rest, ok := strings.Cut(doc, "\n"); ok {
		return strings.TrimLeft(rest, "\n")
	}
	return ""
}

// renderAgents produces AGENTS.md (universal agent context).
func renderAgents(inputs renderInputs) string {
	var b strings.Builder
	b.WriteString(ManagedMarker + "\n\n")
	b.WriteString("# Project Agent Context\n\n")
	b.WriteString("> Auto-generated by `ncgo ai sync` from `")
	b.WriteString(inputs.SourceRef)
	b.WriteString("`.\n")
	b.WriteString("> This file is a task-oriented summary; the full design\n")
	b.WriteString("> reference lives at `docs/ncgo/<profile>/design-doc.en.md`.\n\n")

	b.WriteString(inputs.LongBody)
	b.WriteString("\n")
	b.WriteString(renderMethods(inputs.MethodsByDomain))
	b.WriteString("\n")
	b.WriteString(inputs.EditBoundaries)
	b.WriteString("\n")
	b.WriteString(layerRulesMarkdown)
	b.WriteString("\n## Error Codes\n\n")
	b.WriteString(errorCodesNote)
	b.WriteString(inputs.ErrorCodes)
	b.WriteString("\n## Workflow\n\n")
	b.WriteString(inputs.WorkflowBody)
	b.WriteString("\n")
	b.WriteString(verifyMarkdown)

	if strings.TrimSpace(inputs.LocalNotes) != "" {
		b.WriteString("\n## Local Notes\n\n")
		b.WriteString(strings.TrimRight(inputs.LocalNotes, "\n"))
		b.WriteString("\n")
	}

	return ensureTrailingNewline(b.String())
}

// renderClaude wraps the same body with a Claude Code preface.
func renderClaude(inputs renderInputs) string {
	var b strings.Builder
	b.WriteString(ManagedMarker + "\n\n")
	b.WriteString("# Project Context for Claude Code\n\n")
	b.WriteString("> Auto-generated by `ncgo ai sync` from `")
	b.WriteString(inputs.SourceRef)
	b.WriteString("`.\n")
	b.WriteString("> This file is a task-oriented summary; the full design\n")
	b.WriteString("> reference lives at `docs/ncgo/<profile>/design-doc.en.md`.\n\n")

	b.WriteString(inputs.LongBody)
	b.WriteString("\n")
	b.WriteString(renderMethods(inputs.MethodsByDomain))
	b.WriteString("\n")
	b.WriteString(inputs.EditBoundaries)
	b.WriteString("\n")
	b.WriteString(layerRulesMarkdown)
	b.WriteString("\n## Error Codes\n\n")
	b.WriteString(errorCodesNote)
	b.WriteString(inputs.ErrorCodes)
	b.WriteString("\n## Workflow\n\n")
	b.WriteString(inputs.WorkflowBody)
	b.WriteString("\n")
	b.WriteString(verifyMarkdown)

	if strings.TrimSpace(inputs.LocalNotes) != "" {
		b.WriteString("\n## Local Notes\n\n")
		b.WriteString(strings.TrimRight(inputs.LocalNotes, "\n"))
		b.WriteString("\n")
	}

	return ensureTrailingNewline(b.String())
}

// renderMethods renders the per-domain method list.
func renderMethods(methodsByDomain map[string][]string) string {
	if len(methodsByDomain) == 0 {
		return ""
	}
	domains := make([]string, 0, len(methodsByDomain))
	for d := range methodsByDomain {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	var b strings.Builder
	b.WriteString("### Methods\n\n")
	for _, d := range domains {
		methods := methodsByDomain[d]
		b.WriteString("- **" + d + "**: ")
		b.WriteString(strings.Join(methods, ", "))
		b.WriteString("\n")
	}
	return b.String()
}

// renderCursorMDC wraps the body with Cursor's MDC frontmatter so the
// rules engine picks it up for every Go file in the project.
func renderCursorMDC(inputs renderInputs) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: ncgo project rules and feature workflow (auto-generated)\n")
	b.WriteString("globs: [\"**/*.go\"]\n")
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n")
	b.WriteString(ManagedMarker + "\n\n")
	b.WriteString("# Project Rules (ncgo)\n\n")
	b.WriteString("> Auto-generated by `ncgo ai sync`. Cursor consumes these\n")
	b.WriteString("> rules whenever it edits a Go file in this project.\n\n")
	b.WriteString(inputs.RulesBody)
	return ensureTrailingNewline(b.String())
}

// renderNcgoDevSkill produces the ncgo-dev Claude Code skill file. The
// frontmatter precedes the managed marker so the skill loads correctly and
// isManaged() still finds the marker within the first 6 lines.
func renderNcgoDevSkill(inputs renderInputs) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ncgo-dev\n")
	b.WriteString("description: Implement a feature in this ncgo project (add domain → add method → sqlc → verify → ai sync)\n")
	b.WriteString("---\n")
	b.WriteString(ManagedMarker + "\n\n")
	b.WriteString(inputs.WorkflowBody)
	return ensureTrailingNewline(b.String())
}

func renderProjectContext(inputs renderInputs) string {
	var b strings.Builder
	b.WriteString(ManagedMarker + "\n\n")
	b.WriteString("# Claude Project Context\n\n")
	b.WriteString("> Auto-generated by `ncgo ai sync` from `")
	b.WriteString(inputs.SourceRef)
	b.WriteString("`.\n")
	b.WriteString("> Facts only; policy lives under `.claude/rules/*`.\n")
	b.WriteString("> Personal notes belong in `.claude/local/*`.\n\n")

	b.WriteString(inputs.ProjectContextBody)

	methods := renderMethods(inputs.MethodsByDomain)
	if methods != "" {
		b.WriteString("\n")
		b.WriteString(methods)
	}

	return ensureTrailingNewline(b.String())
}

// isManaged reports whether the given file body carries the managed
// marker on (or near) its first content line. We search the first 6
// non-blank lines so MDC and skill frontmatter does not hide the marker.
func isManaged(content []byte) bool {
	scanned := 0
	for line := range bytes.SplitSeq(content, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if bytes.Contains(trimmed, []byte(ManagedMarker)) {
			return true
		}
		scanned++
		if scanned >= 6 {
			return false
		}
	}
	return false
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

const layerRulesMarkdown = `## Layer Rules

` + "```" + `
handler/* → usecase/*, pkg/response, pb        (no repo / data import)
usecase/* → adapter, repository (ports), pb    (no framework import)
repository/* → base/data, db/gen               (no usecase import)
` + "```" + `

- All errors are ` + "`goerror`" + ` chains carrying a numeric ` + "`Code`" + ` + ` + "`Public`" + ` msg.
- Handlers use ` + "`response.OK(c, resp)`" + ` / ` + "`response.Err(c, err)`" + `.
- Do not store request context in struct state.

`

const errorCodesNote = `Business codes (` + "`>= 40100`" + `) fallback to HTTP 200. Register fine-grained
statuses with ` + "`goerror.RegisterHTTPStatuses`" + ` when non-200 is required.

`

const verifyMarkdown = `## Verify

` + "```bash" + `
go build ./...            # compiles
go vet ./...              # static analysis
ncgo check --root .       # ncgo consistency (anchors, manifest, context)
ncgo ai sync --root .     # re-render after changes
ncgo check --root .       # confirm stale-context check passes
` + "```" + `

`
