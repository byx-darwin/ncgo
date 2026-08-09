package postgenerate

import (
	"context"
	"fmt"
	"time"

	"github.com/byx-darwin/ncgo/internal/ai"
	"github.com/byx-darwin/ncgo/internal/exec"
)

// goModTidy runs `go mod tidy` in opts.Dir.
func goModTidy(ctx context.Context, opts Options) StepResult {
	start := time.Now()
	r := opts.Runner
	if r == nil {
		r = exec.NewDefault()
	}
	_, err := exec.GoModTidy(ctx, r, opts.Dir)
	elapsed := time.Since(start)

	if err != nil {
		result := StepResult{
			Name:   "go mod tidy",
			Status: "failed",
			Detail: fmt.Sprintf("%v (non-blocking)", err),
		}
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "✗ go mod tidy failed: %v (non-blocking)\n", err)
		}
		return result
	}
	result := StepResult{
		Name:   "go mod tidy",
		Status: "succeeded",
		Detail: fmt.Sprintf("(%.1fs)", elapsed.Seconds()),
	}
	if opts.Stdout != nil {
		fmt.Fprintf(opts.Stdout, "✓ go mod tidy (%.1fs)\n", elapsed.Seconds())
	}
	return result
}

// aiSync calls ai.Sync to render AI context files.
func aiSync(ctx context.Context, opts Options) StepResult {
	if opts.AITarget == "none" {
		result := StepResult{
			Name:   "ai sync",
			Status: "skipped",
			Detail: "target=none",
		}
		if opts.Stdout != nil {
			fmt.Fprintln(opts.Stdout, "- ai sync skipped (target=none)")
		}
		return result
	}

	target := opts.AITarget
	if target == "" {
		target = ai.TargetClaude
	}

	start := time.Now()
	_, err := ai.Sync(ai.Options{
		Root:   opts.Dir,
		Target: target,
	})
	elapsed := time.Since(start)

	if err != nil {
		result := StepResult{
			Name:   "ai sync",
			Status: "failed",
			Detail: fmt.Sprintf("%v (non-blocking)", err),
		}
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "✗ ai sync failed: %v (non-blocking)\n", err)
		}
		return result
	}
	result := StepResult{
		Name:   "ai sync",
		Status: "succeeded",
		Detail: fmt.Sprintf("--target %s (%.1fs)", target, elapsed.Seconds()),
	}
	if opts.Stdout != nil {
		fmt.Fprintf(opts.Stdout, "✓ ai sync --target %s (%.1fs)\n", target, elapsed.Seconds())
	}
	return result
}
