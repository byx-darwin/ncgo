package orchestrator

import (
	"context"

	"github.com/byx-darwin/ncgo/internal/ai"
)

// AIInitClaudeOptions configures an ai init claude run.
type AIInitClaudeOptions struct {
	Root   string
	Preset string
	Force  bool
	DryRun bool
}

// AIInitClaudeResult is the structured result of an ai init claude run.
type AIInitClaudeResult struct {
	Written   []string  `json:"written"`
	Skipped   []ai.Skip `json:"skipped"`
	Notes     []string  `json:"notes,omitempty"`
	NextSteps []string  `json:"nextSteps,omitempty"`
}

// AISyncOptions configures an ai sync run.
type AISyncOptions struct {
	Root   string
	Lang   string
	Force  bool
	DryRun bool
}

// AISyncResult is the structured result of an ai sync run.
type AISyncResult struct {
	Written   []string            `json:"written"`
	Skipped   []ai.Skip           `json:"skipped"`
	Notes     []string            `json:"notes,omitempty"`
	NextSteps []string            `json:"nextSteps,omitempty"`
	Scope     string              `json:"scope,omitempty"`
	SourceRef string              `json:"sourceRef,omitempty"`
	Workspace *ai.ResultWorkspace `json:"workspace,omitempty"`
}

// RunAIInitClaude bootstraps the .claude starter set.
func RunAIInitClaude(ctx context.Context, opts AIInitClaudeOptions) (*AIInitClaudeResult, error) {
	res, err := ai.InitClaude(ai.InitOptions{
		Root:   opts.Root,
		Preset: opts.Preset,
		Force:  opts.Force,
		DryRun: opts.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &AIInitClaudeResult{
		Written:   res.Written,
		Skipped:   res.Skipped,
		Notes:     res.Notes,
		NextSteps: res.NextSteps,
	}, nil
}

// RunAISync renders AI context files for an ncgo service or micro workspace.
func RunAISync(ctx context.Context, opts AISyncOptions) (*AISyncResult, error) {
	res, err := ai.Sync(ai.Options{
		Root:   opts.Root,
		Lang:   opts.Lang,
		Force:  opts.Force,
		DryRun: opts.DryRun,
	})
	if err != nil {
		return nil, err
	}
	return &AISyncResult{
		Written:   res.Written,
		Skipped:   res.Skipped,
		Notes:     res.Notes,
		NextSteps: res.NextSteps,
		Scope:     res.Scope,
		SourceRef: res.SourceRef,
		Workspace: res.Workspace,
	}, nil
}
