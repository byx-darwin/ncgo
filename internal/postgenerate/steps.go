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
		return StepResult{
			Name:   "go mod tidy",
			Status: "failed",
			Detail: fmt.Sprintf("%v (non-blocking)", err),
		}
	}
	return StepResult{
		Name:   "go mod tidy",
		Status: "succeeded",
		Detail: fmt.Sprintf("(%.1fs)", elapsed.Seconds()),
	}
}

// aiSync calls ai.Sync to render AI context files.
func aiSync(ctx context.Context, opts Options) StepResult {
	if opts.AITarget == "none" {
		return StepResult{
			Name:   "ai sync",
			Status: "skipped",
			Detail: "target=none",
		}
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
		return StepResult{
			Name:   "ai sync",
			Status: "failed",
			Detail: fmt.Sprintf("%v (non-blocking)", err),
		}
	}
	return StepResult{
		Name:   "ai sync",
		Status: "succeeded",
		Detail: fmt.Sprintf("--target %s (%.1fs)", target, elapsed.Seconds()),
	}
}
