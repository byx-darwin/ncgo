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
	shape := detectProjectShape(opts.Root)
	res := &Result{
		Written: []string{},
		Skipped: []Skip{},
		Notes:   []string{fmt.Sprintf("detected project shape: %s", projectShapeLabel(shape))},
	}
	if !opts.DryRun {
		res.NextSteps = []string{fmt.Sprintf("run ncgo ai sync --root %s --lang en", opts.Root)}
	}
	for _, f := range files {
		if err := writeStarterFile(opts, shape, f, res); err != nil {
			return res, err
		}
	}
	return res, nil
}

func detectProjectShape(root string) projectShape {
	if _, err := manifest.LoadWorkspace(root); err == nil {
		return projectShapeMicroWorkspace
	}
	if _, err := manifest.Load(root); err == nil {
		return projectShapeMonoService
	}
	return projectShapeUnknown
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

func renderStarterContent(relPath string, shape projectShape, content []byte) []byte {
	if relPath != ".claude/README.md" {
		return content
	}
	body := strings.ReplaceAll(string(content), "{{PROJECT_SHAPE_GUIDANCE}}", projectShapeGuidance(shape))
	return []byte(body)
}

func projectShapeGuidance(shape projectShape) string {
	switch shape {
	case projectShapeMicroWorkspace:
		return strings.TrimSpace(`Detected repository shape: **micro workspace root**.

- Root metadata lives in ` + "`ncgo.workspace`" + `.
- Individual services under ` + "`services/*`" + ` keep their own ` + "`.ncgo/manifest.yaml`" + `.
- For service-specific tasks, inspect the target service manifest in addition to the workspace root.

Before making non-trivial changes, read ` + "`.claude/generated/project-context.md`" + ` when present. If it is missing or stale, fall back to ` + "`ncgo.workspace`" + ` and the affected service manifests.`)
	case projectShapeMonoService:
		return strings.TrimSpace(`Detected repository shape: **service root**.

- Primary metadata lives in ` + "`.ncgo/manifest.yaml`" + `.
- Most tasks can be planned and validated at the service or package level.

Before making non-trivial changes, read ` + "`.claude/generated/project-context.md`" + ` when present. If it is missing or stale, fall back to ` + "`.ncgo/manifest.yaml`" + `.`)
	default:
		return strings.TrimSpace(`Repository shape could not be detected yet.

- If this is an ncgo mono service, expect ` + "`.ncgo/manifest.yaml`" + `.
- If this is an ncgo micro workspace, expect ` + "`ncgo.workspace`" + ` at the root and per-service manifests under ` + "`services/*`" + `.

After project metadata exists, run ` + "`ncgo ai sync --root . --lang en`" + ` so agents can rely on ` + "`.claude/generated/project-context.md`" + `.`)
	}
}

func writeStarterFile(opts InitOptions, shape projectShape, f starterFile, res *Result) error {
	content, err := fs.ReadFile(assets.FS(), f.AssetPath)
	if err != nil {
		return fmt.Errorf("ai init claude: read embedded %s: %w", f.AssetPath, err)
	}
	content = renderStarterContent(f.RelPath, shape, content)
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
