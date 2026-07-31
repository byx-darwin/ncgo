package cli

import (
	"encoding/json"
	"fmt"
	"time"

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
	cmd.AddCommand(newRateLimitE2ECmd())
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
	var grpc bool
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
				GRPC:     grpc,
			})
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "Project root")
	cmd.Flags().StringVar(&host, "host", "localhost", "Service host")
	cmd.Flags().IntVar(&port, "port", 8080, "Service port")
	cmd.Flags().IntVar(&rate, "rate", 200, "Requests per second")
	cmd.Flags().StringVar(&duration, "duration", "10s", "Attack duration")
	cmd.Flags().StringSliceVar(&paths, "paths", nil, "Target paths (default: /healthz, /)")
	cmd.Flags().BoolVar(&grpc, "grpc", false, "Use grpcurl to verify gRPC rule-center")
	return cmd
}

func newRateLimitE2ECmd() *cobra.Command {
	var root, host, duration, dsn, report, output, readinessPath, rpcMethod, rpcPayload string
	var port, rate int
	var paths []string
	var cleanup, plan bool
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "End-to-end rate-limit test with auto-detection and report",
		Long: `Detect project type (mono/micro), read rate-limit config,
start dependencies, seed rules, run vegeta attack, analyze results,
and optionally generate a report.

  ncgo test rate-limit e2e
  ncgo test rate-limit e2e --report report.md
  ncgo test rate-limit e2e --plan
  ncgo test rate-limit e2e --readiness-path /healthz --paths /ping
  ncgo test rate-limit e2e --rpc-method MyService.Ping --rpc-payload '{"user":"alice"}'
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if plan {
				output = "json"
			}
			result, err := ratelimit.E2E(cmd.Context(), ratelimit.E2EOptions{
				Root:          root,
				Host:          host,
				Port:          port,
				Rate:          rate,
				Duration:      duration,
				Paths:         paths,
				ReadinessPath: readinessPath,
				DSN:           dsn,
				Cleanup:       cleanup,
				DryRun:        plan,
				Report:        report,
				RPCMethod:     rpcMethod,
				RPCPayload:    rpcPayload,
			})
			if err != nil {
				return err
			}
			if plan || output == "json" {
				b, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(b))
				return nil
			}
			printE2EResult(result)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "Project root")
	cmd.Flags().StringVar(&host, "host", "localhost", "Service host")
	cmd.Flags().IntVar(&port, "port", 8080, "Service port")
	cmd.Flags().IntVar(&rate, "rate", 200, "Requests per second")
	cmd.Flags().StringVar(&duration, "duration", "10s", "Attack duration")
	cmd.Flags().StringSliceVar(&paths, "paths", nil, "Target paths (default: /healthz, /)")
	cmd.Flags().StringVar(&readinessPath, "readiness-path", "", "Health check path (defaults to first path)")
	cmd.Flags().StringVar(&dsn, "dsn", "", "PostgreSQL DSN (default: use docker compose exec)")
	cmd.Flags().BoolVar(&cleanup, "cleanup", true, "Stop dependencies after test")
	cmd.Flags().StringVar(&report, "report", "", "Output report file (.md or .json)")
	cmd.Flags().StringVar(&output, "output", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&plan, "plan", false, "Dry-run: show plan without executing")
	cmd.Flags().StringVar(&rpcMethod, "rpc-method", "", "Kitex RPC method to invoke (defaults to <serviceName>.HealthCheck)")
	cmd.Flags().StringVar(&rpcPayload, "rpc-payload", "", "Kitex RPC JSON payload (defaults to \"{}\")")
	return cmd
}

func printE2EResult(r *ratelimit.E2EResult) {
	fmt.Printf("Mode: %s (%s) | Source: %s | Backend: %s\n", r.Mode, r.Kind, r.Source, r.Backend)
	fmt.Printf("Service: %s\n\n", r.ServiceName)
	fmt.Printf("Status: %s\n", r.Status)
	fmt.Printf("Requests: %d total | %d OK | %d rate-limited | %d other\n",
		r.TotalReqs, r.Status200, r.Status429, r.StatusOther)
	fmt.Printf("Latency: avg=%s p99=%s\n", r.AvgLatency.Round(time.Millisecond), r.P99Latency.Round(time.Millisecond))
	if r.ReportPath != "" {
		fmt.Printf("Report: %s\n", r.ReportPath)
	}
}
