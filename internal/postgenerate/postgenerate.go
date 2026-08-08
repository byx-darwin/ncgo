package postgenerate

import (
	"context"
	"io"

	"github.com/byx-darwin/ncgo/internal/exec"
)

// Options configures post-generation auto-step execution.
type Options struct {
	Dir         string      // absolute project root
	AITarget    string      // "claude" (default) | "all" | "agents" | "cursor" | "none"
	NoAutoSteps bool        // skip all auto steps
	RanGenerate bool        // whether generator (hz/kitex) ran successfully
	Runner      exec.Runner // injected exec; nil = exec.NewDefault()
	Stdout      io.Writer   // progress and warning output
}

// Result reports the outcome of each auto step.
type Result struct {
	Steps []StepResult
}

// StepResult describes one auto step's outcome.
type StepResult struct {
	Name   string // "go mod tidy" | "ai sync"
	Status string // "skipped" | "succeeded" | "failed"
	Detail string // human-readable reason, timing, or error message
}

// Run executes post-generation auto steps. It never returns an error for
// step failures; failures are recorded in Result.Steps and written as
// warnings to opts.Stdout.
func Run(opts Options) *Result {
	res := &Result{}

	// Skip all steps if NoAutoSteps or !RanGenerate
	if opts.NoAutoSteps || !opts.RanGenerate {
		res.Steps = []StepResult{
			{Name: "go mod tidy", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
			{Name: "ai sync", Status: "skipped", Detail: "auto steps disabled or generator did not run"},
		}
		return res
	}

	ctx := context.Background()

	// Step 1: go mod tidy
	goModTidyResult := goModTidy(ctx, opts)
	res.Steps = append(res.Steps, goModTidyResult)

	// Step 2: ai sync (TODO in next task)
	aiSyncResult := StepResult{Name: "ai sync", Status: "skipped", Detail: "not yet implemented"}
	res.Steps = append(res.Steps, aiSyncResult)

	return res
}
