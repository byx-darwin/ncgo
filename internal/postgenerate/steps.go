package postgenerate

import (
	"context"
	"fmt"
	"time"

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
