package ai

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/byx-darwin/ncgo/internal/assets"
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
			starterFile{RelPath: ".claude/agents/implementer.md", AssetPath: "claude/agents/implementer.md"},
			starterFile{RelPath: ".claude/agents/reviewer.md", AssetPath: "claude/agents/reviewer.md"},
			starterFile{RelPath: ".claude/commands/plan.md", AssetPath: "claude/commands/plan.md"},
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
	res := &Result{}
	for _, f := range files {
		if err := writeStarterFile(opts, f, res); err != nil {
			return res, err
		}
	}
	return res, nil
}

func writeStarterFile(opts InitOptions, f starterFile, res *Result) error {
	content, err := fs.ReadFile(assets.FS(), f.AssetPath)
	if err != nil {
		return fmt.Errorf("ai init claude: read embedded %s: %w", f.AssetPath, err)
	}
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
