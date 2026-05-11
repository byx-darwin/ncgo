package ratelimit

import (
	"context"
	"fmt"
)

// RunOptions configures the run command.
type RunOptions struct {
	Root     string
	Host     string
	Port     int
	Rate     int
	Duration string
	Paths    []string
}

// Run executes vegeta attacks against the running service.
// TODO(Task 5): implement vegeta attack execution.
func Run(ctx context.Context, opts RunOptions) error {
	return fmt.Errorf("ncgo test rate-limit run is not yet implemented")
}
