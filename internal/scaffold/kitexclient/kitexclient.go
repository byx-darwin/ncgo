// Package kitexclient implements `ncgo add kitex-client <name>` for generating
// Kitex client wrappers that BFF services use to call RPC services.
package kitexclient

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Options describes a `ncgo add kitex-client` invocation.
type Options struct {
	Root    string // project root
	Name    string // client name (e.g. rbac, rulecenter)
	Service string // RPC service name
	IDL     string // path to proto file
	Force   bool   // overwrite existing files
	DryRun  bool   // preview mode
}

// Result describes what Add produced.
type Result struct {
	DryRun       bool     `json:"dryRun"`
	WrittenPaths []string `json:"writtenPaths"`
	NextSteps    []string `json:"nextSteps"`
}

// Add generates the Kitex client wrapper and config for calling an RPC service.
func Add(opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("kitex-client: name is required")
	}
	if opts.Service == "" {
		return nil, fmt.Errorf("kitex-client: --service is required")
	}
	if opts.IDL == "" {
		return nil, fmt.Errorf("kitex-client: --idl is required")
	}

	result := &Result{DryRun: opts.DryRun}

	// Create pkg/client/<name>/ directory
	clientDir := filepath.Join(opts.Root, "pkg", "client", opts.Name)
	if !opts.DryRun {
		if err := os.MkdirAll(clientDir, 0o755); err != nil {
			return nil, fmt.Errorf("kitex-client: mkdir %s: %w", clientDir, err)
		}
	}

	// Generate client.go
	clientPath := filepath.Join(clientDir, "client.go")
	if err := generateClient(clientPath, opts, result); err != nil {
		return nil, err
	}

	// Generate config.go
	configPath := filepath.Join(clientDir, "config.go")
	if err := generateConfig(configPath, opts, result); err != nil {
		return nil, err
	}

	result.NextSteps = []string{
		"go get github.com/cloudwego/kitex",
		"go mod tidy",
		fmt.Sprintf("kitex -module %s %s", opts.Service, opts.IDL),
	}

	return result, nil
}

func generateClient(path string, opts Options, result *Result) error {
	if !opts.Force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("kitex-client: %s already exists (use --force to overwrite)", path)
		}
	}

	tmpl := `package {{.Name}}

import (
	"context"
)

// Client wraps the Kitex generated client for {{.Service}}.
type Client struct {
	// TODO: Add Kitex client field after running kitex code generation
}

// New creates a new Kitex client for {{.Service}}.
func New(addr string) (*Client, error) {
	// TODO: Initialize Kitex client after running kitex code generation
	// Example:
	// c, err := {{.Service}}.NewClient(addr, client.WithHostPorts(addr))
	// if err != nil {
	//     return nil, err
	// }
	// return &Client{client: c}, nil
	return &Client{}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	// TODO: Implement cleanup
	return nil
}
`
	t, err := template.New("client").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("kitex-client: parse client template: %w", err)
	}

	if !opts.DryRun {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("kitex-client: create %s: %w", path, err)
		}
		defer f.Close()
		if err := t.Execute(f, opts); err != nil {
			return fmt.Errorf("kitex-client: execute client template: %w", err)
		}
	}
	result.WrittenPaths = append(result.WrittenPaths, path)
	return nil
}

func generateConfig(path string, opts Options, result *Result) error {
	if !opts.Force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("kitex-client: %s already exists (use --force to overwrite)", path)
		}
	}

	content := fmt.Sprintf(`package %s

// Config holds configuration for the %s Kitex client.
type Config struct {
	Address string `+"`yaml:\"address\"`"+`
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		Address: "localhost:8888",
	}
}
`, opts.Name, opts.Name)

	if !opts.DryRun {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("kitex-client: write %s: %w", path, err)
		}
	}
	result.WrittenPaths = append(result.WrittenPaths, path)
	return nil
}
