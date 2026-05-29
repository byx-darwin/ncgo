package orchestrator

import (
	"context"

	"github.com/byx-darwin/ncgo/internal/protolint"
)

// ProtolintOptions configures a protolint run.
type ProtolintOptions struct {
	Root        string
	Files       []string
	Rules       []string
	IgnoreRules []string
	IgnoreFiles []string
}

// ProtolintResult is the structured result of a protolint run.
type ProtolintResult struct {
	Root         string                 `json:"root"`
	Files        []string               `json:"files"`
	RulesRun     []string               `json:"rulesRun"`
	IgnoredRules []string               `json:"ignoredRules,omitempty"`
	IgnoredFiles []string               `json:"ignoredFiles,omitempty"`
	OK           bool                   `json:"ok"`
	Summary      protolint.Summary      `json:"summary"`
	Diagnostics  []protolint.Diagnostic `json:"diagnostics"`
	// Result holds the raw protolint.Result for formatting by CLI/MCP layers.
	Result *protolint.Result `json:"-"`
}

// RunProtolint executes a protolint run and returns a structured result.
// It wraps protolint.Run without changing its public API.
func RunProtolint(ctx context.Context, opts ProtolintOptions) (*ProtolintResult, error) {
	res, err := protolint.Run(ctx, protolint.RunOptions{
		Root:          opts.Root,
		Files:         opts.Files,
		RuleIDs:       opts.Rules,
		IgnoreRuleIDs: opts.IgnoreRules,
		IgnoreFiles:   opts.IgnoreFiles,
	})
	if err != nil {
		return nil, err
	}
	return &ProtolintResult{
		Root:         res.Root,
		Files:        res.Files,
		RulesRun:     res.RulesRun,
		IgnoredRules: res.IgnoredRules,
		IgnoredFiles: res.IgnoredFiles,
		OK:           res.OK,
		Summary:      res.Summary,
		Diagnostics:  res.Diagnostics,
		Result:       res,
	}, nil
}
