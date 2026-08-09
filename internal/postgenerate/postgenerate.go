package postgenerate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/byx-darwin/ncgo/internal/exec"
)

// Options configures post-generation auto-step execution.
type Options struct {
	Dir          string      // absolute project root
	AITarget     string      // "claude" (default) | "all" | "agents" | "cursor" | "none"
	NoAutoSteps  bool        // skip all auto steps
	RanGenerate  bool        // whether generator (hz/kitex) ran successfully
	Runner       exec.Runner // injected exec; nil = exec.NewDefault()
	Stdout       io.Writer   // progress and warning output
	Kind         string      // service kind: "hertz" | "kitex" (for sqlc ordering)
	WithDatabase bool        // whether database scaffolding is enabled (for sqlc ordering)
}

// Result reports the outcome of each auto step.
type Result struct {
	Steps []StepResult
}

// StepResult describes one auto step's outcome.
type StepResult struct {
	Name   string `json:"name"`   // "go mod tidy" | "ai sync"
	Status string `json:"status"` // "skipped" | "succeeded" | "failed"
	Detail string `json:"detail"` // human-readable reason, timing, or error message
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

	// Print header
	if opts.Stdout != nil {
		fmt.Fprintln(opts.Stdout, "auto steps:")
	}

	ctx := context.Background()

	// Step 0 (conditional): sqlc - must run before go mod tidy for Kitex or Hertz-with-db
	if requiresSQLCBeforeTidy(opts) {
		sqlcResult := sqlc(ctx, opts)
		res.Steps = append(res.Steps, sqlcResult)
	}

	// Step 1: go mod tidy
	goModTidyResult := goModTidy(ctx, opts)
	res.Steps = append(res.Steps, goModTidyResult)

	// Step 2: ai sync
	aiSyncResult := aiSync(ctx, opts)
	res.Steps = append(res.Steps, aiSyncResult)

	return res
}

// requiresSQLCBeforeTidy reports whether the scaffolded code references
// internal/db/gen before the user has had a chance to run `go mod tidy`.
// Kitex always wires base/data + repository placeholders, while Hertz only
// needs sqlc first when the database scaffold is enabled.
func requiresSQLCBeforeTidy(opts Options) bool {
	return opts.Kind == "kitex" || opts.WithDatabase
}

// FilterNextSteps removes NextSteps that were auto-executed successfully.
// It returns the input unchanged when r is nil or a step did not succeed,
// so callers keep showing steps that still need manual execution. This is
// the single source of truth shared by the CLI and MCP output paths.
func (r *Result) FilterNextSteps(steps []string) []string {
	if r == nil {
		return steps
	}
	filtered := make([]string, 0, len(steps))
	for _, step := range steps {
		// Skip "go mod tidy" if it succeeded
		if strings.Contains(step, "go mod tidy") {
			if r.stepSucceeded("go mod tidy") {
				continue
			}
		}
		// Skip "ncgo ai sync" if it succeeded
		if strings.Contains(step, "ncgo ai sync") {
			if r.stepSucceeded("ai sync") {
				continue
			}
		}
		filtered = append(filtered, step)
	}
	return filtered
}

// stepSucceeded reports whether the named auto step ended with status
// "succeeded".
func (r *Result) stepSucceeded(name string) bool {
	for _, s := range r.Steps {
		if s.Name == name && s.Status == "succeeded" {
			return true
		}
	}
	return false
}
