package ratelimit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// SeedOptions configures the seed command.
type SeedOptions struct {
	Root        string
	DSN         string // If set, connects directly via psql on host. If empty, uses docker compose exec.
	MaxRequests int
	WindowSecs  int
}

// Seed inserts test rate-limit rules into PostgreSQL.
func Seed(ctx context.Context, opts SeedOptions) error {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.MaxRequests == 0 {
		opts.MaxRequests = 10
	}
	if opts.WindowSecs == 0 {
		opts.WindowSecs = 60
	}

	m, err := manifest.Load(opts.Root)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	service := m.Service.Name
	if service == "" {
		service = filepath.Base(opts.Root)
	}

	sql := buildSeedSQL(service, opts.MaxRequests, opts.WindowSecs)
	return execSQLViaDockerCompose(ctx, opts.Root, opts.DSN, sql)
}

// buildSeedSQL generates DELETE + INSERT SQL for test rules.
func buildSeedSQL(service string, maxRequests, windowSecs int) string {
	var b strings.Builder
	svc := sanitizeSQLString(service)
	b.WriteString(fmt.Sprintf("DELETE FROM rate_limit_rules WHERE service = '%s';\n", svc))
	b.WriteString(fmt.Sprintf(`INSERT INTO rate_limit_rules (service, phase, method, match_kind, path, path_pattern, enabled, key_by, strategy, window_seconds, max_requests) VALUES
('%s', 'pre_auth', '*', 'prefix', NULL, '/', true, ARRAY['ip']::text[], 'fixed_window', %d, %d),
('%s', 'pre_auth', 'GET', 'exact', '/healthz', NULL, true, ARRAY['ip']::text[], 'fixed_window', %d, %d),
('%s', 'grpc', '*', 'prefix', NULL, '/', true, ARRAY['ip']::text[], 'fixed_window', %d, %d);
`, svc, windowSecs, maxRequests, svc, windowSecs, maxRequests, svc, windowSecs, maxRequests))
	return b.String()
}

// execSQLViaDockerCompose pipes SQL to postgres container via stdin.
func execSQLViaDockerCompose(ctx context.Context, root, dsn, sql string) error {
	if dsn != "" {
		// Direct psql connection on host
		psqlPath, err := exec.LookPath("psql")
		if err != nil {
			return fmt.Errorf("psql not found (install postgres client or use docker compose instead)")
		}
		cmd := exec.CommandContext(ctx, psqlPath, dsn)
		cmd.Stdin = strings.NewReader(sql)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("execute SQL via psql: %w", err)
		}
		fmt.Println("seeded rate-limit rules via psql")
		return nil
	}

	// Use docker compose exec
	composeDir := root
	if !pathExists(filepath.Join(root, "compose.yaml")) && !pathExists(filepath.Join(root, "docker-compose.yaml")) {
		return fmt.Errorf("no compose.yaml or docker-compose.yaml found in %s (run `docker compose up -d` first)", root)
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found (install docker compose or use --dsn to connect directly)")
	}

	cmd := exec.CommandContext(ctx, dockerPath, "compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", "app")
	cmd.Dir = composeDir
	cmd.Stdin = strings.NewReader(sql)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute SQL via docker compose: %w\n\nHint: make sure postgres is running: docker compose up -d", err)
	}
	fmt.Println("seeded rate-limit rules via docker compose")
	return nil
}

func sanitizeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
