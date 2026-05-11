package cli

import (
	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/scaffold/test/ratelimit"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run generated tests for an ncgo project",
	}
	cmd.AddCommand(newRateLimitTestCmd())
	return cmd
}

func newRateLimitTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rate-limit",
		Short: "Seed and run rate-limit tests",
	}
	cmd.AddCommand(newRateLimitSeedCmd())
	cmd.AddCommand(newRateLimitRunCmd())
	return cmd
}

func newRateLimitSeedCmd() *cobra.Command {
	var dsn, root string
	var maxRequests, windowSecs int
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Insert test rate-limit rules into PostgreSQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ratelimit.Seed(cmd.Context(), ratelimit.SeedOptions{
				Root:        root,
				DSN:         dsn,
				MaxRequests: maxRequests,
				WindowSecs:  windowSecs,
			})
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "Project root")
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN (default: use docker compose exec)")
	cmd.Flags().IntVar(&maxRequests, "max-requests", 10, "Max requests per window")
	cmd.Flags().IntVar(&windowSecs, "window", 60, "Window size in seconds")
	return cmd
}

func newRateLimitRunCmd() *cobra.Command {
	var root, host, duration string
	var port, rate int
	var paths []string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute vegeta attack against the running service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ratelimit.Run(cmd.Context(), ratelimit.RunOptions{
				Root:     root,
				Host:     host,
				Port:     port,
				Rate:     rate,
				Duration: duration,
				Paths:    paths,
			})
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "Project root")
	cmd.Flags().StringVar(&host, "host", "localhost", "Service host")
	cmd.Flags().IntVar(&port, "port", 8080, "Service port")
	cmd.Flags().IntVar(&rate, "rate", 200, "Requests per second")
	cmd.Flags().StringVar(&duration, "duration", "10s", "Attack duration")
	cmd.Flags().StringSliceVar(&paths, "paths", nil, "Target paths (default: /healthz, /)")
	return cmd
}
