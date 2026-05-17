package ai

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// InitOptions controls `ncgo ai init claude`.
type InitOptions struct {
	Root   string // repository root where .claude/ should be bootstrapped
	Preset string // minimal (default) or team
	Force  bool   // overwrite existing starter files
	DryRun bool   // do not write; only report intended actions
}

const (
	InitPresetMinimal = "minimal"
	InitPresetTeam    = "team"
)

type starterFile struct {
	RelPath   string
	AssetPath string
}

type projectShape string

const (
	projectShapeUnknown        projectShape = "unknown"
	projectShapeMonoService    projectShape = "mono_service"
	projectShapeMicroWorkspace projectShape = "micro_workspace"
)

// initContext carries the detection result from manifests, sufficient for
// rendering all starter files.
type initContext struct {
	Shape           projectShape
	Kind            string            // hertz | kitex | "" (empty for micro_workspace or unknown)
	WorkspaceServices []initServiceDesc
}

// initServiceDesc describes one service in a micro workspace.
type initServiceDesc struct {
	Name string
	Kind string // hertz | kitex
	Dir  string // relative to workspace root
}

func claudeStarterFiles(preset string) ([]starterFile, error) {
	files := []starterFile{
		{RelPath: ".claude/README.md", AssetPath: "claude/README.md"},
		{RelPath: ".claude/rules/agent-engineering.md", AssetPath: "claude/rules/agent-engineering.md"},
		{RelPath: ".claude/rules/go.md", AssetPath: "claude/rules/go.md"},
		{RelPath: ".claude/local/.gitignore", AssetPath: "claude/local/.gitignore"},
	}
	switch preset {
	case "", InitPresetMinimal:
		return files, nil
	case InitPresetTeam:
		files = append(files,
			starterFile{RelPath: ".claude/skills/plan-change.md", AssetPath: "claude/skills/plan-change.md"},
			starterFile{RelPath: ".claude/skills/run-validation.md", AssetPath: "claude/skills/run-validation.md"},
			starterFile{RelPath: ".claude/skills/doc-sync.md", AssetPath: "claude/skills/doc-sync.md"},
			starterFile{RelPath: ".claude/skills/write-tests.md", AssetPath: "claude/skills/write-tests.md"},
			starterFile{RelPath: ".claude/agents/planner.md", AssetPath: "claude/agents/planner.md"},
			starterFile{RelPath: ".claude/agents/implementer.md", AssetPath: "claude/agents/implementer.md"},
			starterFile{RelPath: ".claude/agents/reviewer.md", AssetPath: "claude/agents/reviewer.md"},
			starterFile{RelPath: ".claude/agents/debugger.md", AssetPath: "claude/agents/debugger.md"},
			starterFile{RelPath: ".claude/agents/doc-writer.md", AssetPath: "claude/agents/doc-writer.md"},
			starterFile{RelPath: ".claude/commands/plan.md", AssetPath: "claude/commands/plan.md"},
			starterFile{RelPath: ".claude/commands/implement-change.md", AssetPath: "claude/commands/implement-change.md"},
			starterFile{RelPath: ".claude/commands/fix-failing-test.md", AssetPath: "claude/commands/fix-failing-test.md"},
			starterFile{RelPath: ".claude/commands/update-docs.md", AssetPath: "claude/commands/update-docs.md"},
			starterFile{RelPath: ".claude/commands/review-diff.md", AssetPath: "claude/commands/review-diff.md"},
			starterFile{RelPath: ".claude/hooks/README.md", AssetPath: "claude/hooks/README.md"},
		)
		return files, nil
	default:
		return nil, fmt.Errorf("ai init claude: unsupported preset %q (minimal|team)", preset)
	}
}

// InitClaude bootstraps the hand-authored `.claude` starter set.
func InitClaude(opts InitOptions) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	files, err := claudeStarterFiles(opts.Preset)
	if err != nil {
		return nil, err
	}
	ctx := detectInitContext(opts.Root)
	res := &Result{
		Written: []string{},
		Skipped: []Skip{},
		Notes:   []string{fmt.Sprintf("detected project shape: %s", projectShapeLabel(ctx.Shape))},
	}
	if !opts.DryRun {
		res.NextSteps = []string{fmt.Sprintf("run ncgo ai sync --root %s --lang en", opts.Root)}
	}
	for _, f := range files {
		if err := writeStarterFile(opts, ctx, f, res); err != nil {
			return res, err
		}
	}
	if ctx.Shape == projectShapeMicroWorkspace {
		if err := writeServiceDirs(opts, ctx, res); err != nil {
			return res, err
		}
	}
	return res, nil
}

// detectInitContext returns an initContext populated from manifests at root.
func detectInitContext(root string) initContext {
	if ws, err := manifest.LoadWorkspace(root); err == nil {
		services := make([]initServiceDesc, 0, len(ws.Services))
		for _, s := range ws.Services {
			services = append(services, initServiceDesc{
				Name: s.Name,
				Kind: s.Kind,
				Dir:  s.Dir,
			})
		}
		return initContext{
			Shape:           projectShapeMicroWorkspace,
			Kind:            "",
			WorkspaceServices: services,
		}
	}
	if m, err := manifest.Load(root); err == nil {
		return initContext{
			Shape: projectShapeMonoService,
			Kind:  m.Service.Kind,
		}
	}
	return initContext{Shape: projectShapeUnknown}
}

func projectShapeLabel(shape projectShape) string {
	switch shape {
	case projectShapeMicroWorkspace:
		return "micro workspace root"
	case projectShapeMonoService:
		return "service root"
	default:
		return "unknown"
	}
}

func renderStarterContent(relPath string, ctx initContext, content []byte) []byte {
	body := string(content)

	if relPath == ".claude/README.md" {
		body = strings.ReplaceAll(body, "{{PROJECT_SHAPE_GUIDANCE}}", projectShapeGuidance(ctx))
	}

	if relPath == ".claude/rules/go.md" {
		body = strings.ReplaceAll(body, "{{ARCHITECTURE_RULES}}", architectureRulesSnippet(ctx))
	}

	// Agents: inject arch hint for implementer, reviewer, debugger
	if isArchHintAgent(relPath) {
		body = strings.ReplaceAll(body, "{{ARCH_HINT}}", archHintSnippet(ctx))
	}

	return []byte(body)
}

func isArchHintAgent(relPath string) bool {
	switch relPath {
	case ".claude/agents/implementer.md",
		".claude/agents/reviewer.md",
		".claude/agents/debugger.md":
		return true
	default:
		return false
	}
}

func projectShapeGuidance(ctx initContext) string {
	switch ctx.Shape {
	case projectShapeMicroWorkspace:
		return microWorkspaceGuidance(ctx)
	case projectShapeMonoService:
		return monoServiceGuidance(ctx)
	default:
		return unknownGuidance()
	}
}

func microWorkspaceGuidance(ctx initContext) string {
	var b strings.Builder
	b.WriteString("Detected repository shape: **micro workspace root**.\n\n")
	b.WriteString("- Root metadata lives in `ncgo.workspace`.\n")
	b.WriteString("- Individual services under `services/*` keep their own `.ncgo/manifest.yaml`.\n")
	if len(ctx.WorkspaceServices) > 0 {
		b.WriteString("\nRegistered services:\n\n")
		for _, svc := range ctx.WorkspaceServices {
			kindLabel := kindHumanLabel(svc.Kind)
			fmt.Fprintf(&b, "- `%s` (%s) -- `%s`\n", svc.Name, kindLabel, svc.Dir)
		}
		b.WriteString("\n")
	}
	b.WriteString("For service-specific tasks, inspect the target service manifest in addition to the workspace root.\n\n")
	b.WriteString("Before making non-trivial changes, read `.claude/generated/project-context.md` when present. If it is missing or stale, fall back to `ncgo.workspace` and the affected service manifests.")
	return strings.TrimSpace(b.String())
}

func monoServiceGuidance(ctx initContext) string {
	kindLabel := kindHumanLabel(ctx.Kind)
	var b strings.Builder
	b.WriteString("Detected repository shape: **service root**")
	if ctx.Kind != "" {
		fmt.Fprintf(&b, " (%s)", kindLabel)
	}
	b.WriteString(".\n\n")
	b.WriteString("- Primary metadata lives in `.ncgo/manifest.yaml`.\n")
	b.WriteString("- Most tasks can be planned and validated at the service or package level.\n\n")
	b.WriteString("Before making non-trivial changes, read `.claude/generated/project-context.md` when present. If it is missing or stale, fall back to `.ncgo/manifest.yaml`.")
	return strings.TrimSpace(b.String())
}

func unknownGuidance() string {
	return strings.TrimSpace(`Repository shape could not be detected yet.

- If this is an ncgo mono service, expect ` + "`.ncgo/manifest.yaml`" + `.
- If this is an ncgo micro workspace, expect ` + "`ncgo.workspace`" + ` at the root and per-service manifests under ` + "`services/*`" + `.

After project metadata exists, run ` + "`ncgo ai sync --root . --lang en`" + ` so agents can rely on ` + "`.claude/generated/project-context.md`" + `.`)
}

func kindHumanLabel(kind string) string {
	switch kind {
	case manifest.KindHertz:
		return "HTTP (Hertz)"
	case manifest.KindKitex:
		return "RPC (Kitex)"
	default:
		return kind
	}
}

// architectureRulesSnippet returns the arch-specific rules for mono services,
// or an empty string for micro workspaces (where rules live per-service).
func architectureRulesSnippet(ctx initContext) string {
	if ctx.Shape != projectShapeMonoService {
		return ""
	}
	switch ctx.Kind {
	case manifest.KindHertz:
		return hertzRulesSnippet()
	case manifest.KindKitex:
		return kitexRulesSnippet()
	default:
		return ""
	}
}

func hertzRulesSnippet() string {
	return `## Hertz HTTP Service Rules

This service uses Hertz as the HTTP transport layer.

### Middleware vs Handler

- middleware (` + "`internal/pkg/middleware/`" + `) handles cross-cutting concerns: auth, rate-limit, CORS, idempotency
- handlers (` + "`internal/handler/`" + `) delegate to usecase; do NOT embed middleware logic in handlers
- the middleware chain order is defined in ` + "`internal/base/server/server.go`" + `

### Request Context

- Hertz uses ` + "`*app.RequestContext`" + ` as the request context carrier
- propagate ` + "`ctx context.Context`" + ` explicitly (derived from ` + "`c.Request.Context()`" + `)
- do not store ` + "`*app.RequestContext`" + ` in struct state or pass it past the handler boundary

### Response Patterns

- use ` + "`response.OK(c, resp)`" + ` / ` + "`response.Err(c, err)`" + ` from ` + "`internal/pkg/response`" + `
- error codes are 5-digit: ` + "`1xxxx`" + ` for request/auth/rate-limit errors
- business codes are registered at startup via ` + "`response.MustRegister`" + `

### Route Registration

- routes are generated by ` + "`hz`" + ` from IDL and registered via ` + "`router.GeneratedRegister`" + `
- do not manually edit generated route registration; use the ` + "`hz`" + ` template or ` + "`ncgo add`" + ` commands`
}

func kitexRulesSnippet() string {
	return `## Kitex RPC Service Rules

This service uses Kitex as the RPC transport layer.

### Interceptors vs Handler

- interceptors (` + "`internal/pkg/interceptor/`" + `) handle cross-cutting concerns: RequestID, AccessLog, Recovery, RequestTimeout, CallerAllowlist
- handlers (` + "`internal/handler/`" + `) delegate to usecase; do NOT embed interceptor logic in handlers
- interceptors are wired in ` + "`internal/base/server/server.go`" + ` via kitex server options

### RPC Error Handling

- all errors crossing the RPC boundary go through ` + "`rpcerror.ToBizError(err)`" + `
- callers receive a ` + "`kitex.BizStatusError`" + ` carrying a 5-digit business code
- do NOT return plain Go errors from handlers; always use the rpcerror mapping

### Request Context

- Kitex uses ` + "`context.Context`" + ` carried through the RPC invocation chain
- propagate the caller's context; do not create ` + "`context.Background()`" + ` in the middle of a request path

### Client-Side Usage

- generated clients live in ` + "`pkg/client/<service>/`" + `
- consumed by adapters, never directly from handlers or usecases
- retry and circuit-breaker config is in the client package, not per-call-site`
}

// archHintSnippet returns the service-specific guidance paragraph for agents.
// For micro workspaces it references the per-service directory; for mono
// services it is empty (arch rules are already in go.md).
func archHintSnippet(ctx initContext) string {
	if ctx.Shape != projectShapeMicroWorkspace || len(ctx.WorkspaceServices) == 0 {
		return ""
	}
	return "\n## Service-Specific Guidance\n\n" +
		"When working on a service that has a `.claude/services/<service-name>/` directory,\n" +
		"read the relevant files there before making changes:\n\n" +
		"- `rules.md` — architecture-specific Go rules\n" +
		"- `reviewer-checklist.md` — service-specific review checklist\n" +
		"- `implementer-guide.md` — implementation patterns for this architecture\n" +
		"- `debugger-playbook.md` — framework-specific troubleshooting"
}

func writeStarterFile(opts InitOptions, ctx initContext, f starterFile, res *Result) error {
	content, err := fs.ReadFile(assets.FS(), f.AssetPath)
	if err != nil {
		return fmt.Errorf("ai init claude: read embedded %s: %w", f.AssetPath, err)
	}
	content = renderStarterContent(f.RelPath, ctx, content)
	full := filepath.Join(opts.Root, f.RelPath)
	if _, err := os.Stat(full); err == nil {
		if !opts.Force {
			res.Skipped = append(res.Skipped, Skip{Path: f.RelPath, Reason: "exists; pass --force to overwrite"})
			return nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("ai init claude: stat %s: %w", full, err)
	}
	if opts.DryRun {
		res.Skipped = append(res.Skipped, Skip{Path: f.RelPath, Reason: "dry-run"})
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("ai init claude: mkdir %s: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		return fmt.Errorf("ai init claude: write %s: %w", full, err)
	}
	res.Written = append(res.Written, f.RelPath)
	return nil
}

// writeServiceDirs creates .claude/services/<name>/ subdirectories for each
// service in a micro workspace, populating them with architecture-specific
// files from embedded templates.
func writeServiceDirs(opts InitOptions, ctx initContext, res *Result) error {
	srcFS := assets.FS()
	for _, svc := range ctx.WorkspaceServices {
		archDir := "claude/arch/" + svc.Kind
		entries, err := fs.ReadDir(srcFS, archDir)
		if err != nil {
			// Unknown arch kind — skip silently.
			continue
		}
		svcClaudeDir := filepath.Join(opts.Root, ".claude", "services", svc.Name)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			assetPath := archDir + "/" + e.Name()
			b, err := fs.ReadFile(srcFS, assetPath)
			if err != nil {
				return fmt.Errorf("ai init claude: read embedded %s: %w", assetPath, err)
			}
			target := filepath.Join(svcClaudeDir, e.Name())
			if _, err := os.Stat(target); err == nil {
				if !opts.Force {
					res.Skipped = append(res.Skipped, Skip{Path: ".claude/services/" + svc.Name + "/" + e.Name(), Reason: "exists; pass --force to overwrite"})
					continue
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("ai init claude: stat %s: %w", target, err)
			}
			if opts.DryRun {
				res.Skipped = append(res.Skipped, Skip{Path: ".claude/services/" + svc.Name + "/" + e.Name(), Reason: "dry-run"})
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("ai init claude: mkdir %s: %w", filepath.Dir(target), err)
			}
			if err := os.WriteFile(target, b, 0o644); err != nil {
				return fmt.Errorf("ai init claude: write %s: %w", target, err)
			}
			res.Written = append(res.Written, ".claude/services/"+svc.Name+"/"+e.Name())
		}
	}
	return nil
}

// WriteServiceClaudeDirs is the public API used by `ncgo add` to incrementally
// update .claude/ for a newly added service. It creates the per-service
// subdirectory with architecture-specific files without touching existing files.
func WriteServiceClaudeDirs(root, serviceName, kind string) error {
	srcFS := assets.FS()
	archDir := "claude/arch/" + kind
	entries, err := fs.ReadDir(srcFS, archDir)
	if err != nil {
		return nil // unknown arch — nothing to write
	}
	svcClaudeDir := filepath.Join(root, ".claude", "services", serviceName)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		assetPath := archDir + "/" + e.Name()
		b, err := fs.ReadFile(srcFS, assetPath)
		if err != nil {
			return fmt.Errorf("ai init claude: read embedded %s: %w", assetPath, err)
		}
		target := filepath.Join(svcClaudeDir, e.Name())
		if _, err := os.Stat(target); err == nil {
			continue // skip existing
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("ai init claude: stat %s: %w", target, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("ai init claude: mkdir %s: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return fmt.Errorf("ai init claude: write %s: %w", target, err)
		}
	}
	// Also update .claude/README.md to list the new service
	if err := UpdateClaudeReadmeForService(root, serviceName, kind); err != nil {
		return fmt.Errorf("ai init claude: update README: %w", err)
	}
	return nil
}

// UpdateClaudeReadmeForService appends a service entry to the "Registered
// services:" section of .claude/README.md. If the section does not exist, it
// is inserted before the "## Agent Files" heading.
func UpdateClaudeReadmeForService(root, serviceName, kind string) error {
	readmePath := filepath.Join(root, ".claude", "README.md")
	existing, err := os.ReadFile(readmePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // no README to update
		}
		return fmt.Errorf("read README: %w", err)
	}
	body := string(existing)
	kindLabel := kindHumanLabel(kind)
	serviceLine := fmt.Sprintf("- `%s` (%s) -- `services/%s`", serviceName, kindLabel, serviceName)

	// If service is already listed, skip.
	if strings.Contains(body, "`"+serviceName+"`") {
		return nil
	}

	servicesHeader := "Registered services:\n\n"
	agentHeading := "\n## Agent Files"

	if idx := strings.Index(body, servicesHeader); idx != -1 {
		// Insert the service line after the header, before the next blank line or heading.
		insertPos := idx + len(servicesHeader)
		body = body[:insertPos] + serviceLine + "\n" + body[insertPos:]
	} else {
		// Insert the services section before the Agent Files heading.
		if idx := strings.Index(body, agentHeading); idx != -1 {
			servicesSection := "\nRegistered services:\n\n" + serviceLine + "\n"
			body = body[:idx] + servicesSection + body[idx:]
		} else {
			// Fallback: append before the final heading or at the end.
			body = body + "\n\nRegistered services:\n\n" + serviceLine + "\n"
		}
	}

	return os.WriteFile(readmePath, []byte(body), 0o644)
}
