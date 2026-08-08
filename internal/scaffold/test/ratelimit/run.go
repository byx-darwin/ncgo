package ratelimit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunOptions configures the run command.
type RunOptions struct {
	Root     string
	Host     string
	Port     int
	Rate     int
	Duration string
	Paths    []string
	GRPC     bool // when true, use grpcurl instead of vegeta
}

// Run executes vegeta attacks against the running service.
func Run(ctx context.Context, opts RunOptions) error {
	if opts.GRPC {
		return runGRPC(ctx, opts)
	}
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Host == "" {
		opts.Host = "localhost"
	}
	if opts.Port == 0 {
		opts.Port = 8080
	}
	if opts.Rate == 0 {
		opts.Rate = 200
	}
	if opts.Duration == "" {
		opts.Duration = "10s"
	}
	if len(opts.Paths) == 0 {
		opts.Paths = []string{"/healthz", "/"}
	}

	targets := buildTargets(opts.Host, opts.Port, opts.Paths)
	targetBody := strings.Join(targets, "\n") + "\n"

	if _, err := exec.LookPath("vegeta"); err == nil {
		return runVegetaLocal(ctx, targetBody, opts)
	}
	return runVegetaDocker(ctx, opts.Root, targetBody, opts)
}

// buildTargets creates vegeta target lines like "GET http://host:port/path".
func buildTargets(host string, port int, paths []string) []string {
	var out []string
	for _, p := range paths {
		out = append(out, fmt.Sprintf("GET http://%s:%d%s", host, port, p))
	}
	return out
}

func runVegetaLocal(ctx context.Context, targets string, opts RunOptions) error {
	tmpFile := filepath.Join(os.TempDir(), "vegeta-targets.txt")
	if err := os.WriteFile(tmpFile, []byte(targets), 0o644); err != nil {
		return fmt.Errorf("write targets file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	cmd := exec.CommandContext(ctx, "vegeta", "attack",
		"-targets="+tmpFile,
		"-rate="+fmt.Sprintf("%d", opts.Rate),
		"-duration="+opts.Duration,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runVegetaDocker(ctx context.Context, root, targets string, opts RunOptions) error {
	tmpFile := filepath.Join(os.TempDir(), "vegeta-targets.txt")
	if err := os.WriteFile(tmpFile, []byte(targets), 0o644); err != nil {
		return fmt.Errorf("write targets file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	cmd := exec.CommandContext(ctx, "docker", "compose", "run", "--rm",
		"-v", tmpFile+":/targets.txt",
		"vegeta", "attack", "-targets=/targets.txt",
		"-rate="+fmt.Sprintf("%d", opts.Rate),
		"-duration="+opts.Duration,
	)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGRPC(ctx context.Context, opts RunOptions) error {
	if _, err := exec.LookPath("grpcurl"); err != nil {
		return fmt.Errorf("grpcurl not found: install it via `go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest`")
	}
	target := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	cmd := exec.CommandContext(ctx, "grpcurl", "-plaintext", "-d",
		`{"service":"test","phase":"grpc","path":"/test"}`,
		target, "ratelimit.v1.RuleService.GetRule")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
