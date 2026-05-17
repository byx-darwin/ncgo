// Package rulecenter implements `ncgo add rule-center` for adding
// rule-center gRPC client integration to existing Hertz services.
package rulecenter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// Options describes a `ncgo add rule-center` invocation.
type Options struct {
	Root   string // project root
	Addr   string // rule-center gRPC address
	Force  bool   // overwrite existing files
	DryRun bool   // preview mode
}

// Result describes what Add produced.
type Result struct {
	DryRun       bool     `json:"dryRun"`
	WrittenPaths []string `json:"writtenPaths"`
	NextSteps    []string `json:"nextSteps"`
}

// Add generates the rule-center gRPC client and updates configuration.
func Add(opts Options) (*Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Addr == "" {
		return nil, fmt.Errorf("rule-center: addr is required")
	}

	m, err := manifest.Load(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("rule-center: load manifest: %w", err)
	}
	if m.Service.Kind != "hertz" {
		return nil, fmt.Errorf("rule-center: only supported for Hertz services (got %s)", m.Service.Kind)
	}

	result := &Result{DryRun: opts.DryRun}

	// 1. Generate rule_center_client.go
	clientPath, err := writeRuleCenterClient(opts, m)
	if err != nil {
		return result, err
	}
	if !opts.DryRun {
		result.WrittenPaths = append(result.WrittenPaths, clientPath)
	}

	// 2. Update conf/dev/conf.yaml to set source.type: rule_center
	confPath := filepath.Join(opts.Root, "conf", "dev", "conf.yaml")
	if !opts.DryRun {
		if err := updateConfForRuleCenter(confPath, opts.Addr); err != nil {
			return result, fmt.Errorf("rule-center: update config: %w", err)
		}
		result.WrittenPaths = append(result.WrittenPaths, confPath)
	}

	// 3. Wire RuleCenter client into server.go if it exists
	serverPath := filepath.Join(opts.Root, "internal", "base", "server", "server.go")
	if !opts.DryRun {
		if _, err := os.Stat(serverPath); err == nil {
			if err := wireRuleCenterInServer(serverPath); err != nil {
				return result, fmt.Errorf("rule-center: wire server.go: %w", err)
			}
			result.WrittenPaths = append(result.WrittenPaths, serverPath)
		}
	}

	result.NextSteps = []string{
		"go get google.golang.org/grpc",
		"go mod tidy",
		"make dev",
	}

	return result, nil
}

func writeRuleCenterClient(opts Options, m *manifest.Manifest) (string, error) {
	srcFS := assets.FS()
	b, err := fs.ReadFile(srcFS, "hertz/optional/rule_center_client.go")
	if err != nil {
		return "", fmt.Errorf("read embedded template: %w", err)
	}

	rendered := strings.ReplaceAll(string(b), "{{.GoModule}}", m.Module)
	targetDir := filepath.Join(opts.Root, "internal", "pkg", "middleware")
	target := filepath.Join(targetDir, "rule_center_client.go")

	if !opts.Force {
		if _, err := os.Stat(target); err == nil {
			return "", fmt.Errorf("rule_center_client.go already exists (use --force to overwrite)")
		}
	}

	if !opts.DryRun {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", targetDir, err)
		}
		if err := os.WriteFile(target, []byte(rendered), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", target, err)
		}
	}

	return target, nil
}

func updateConfForRuleCenter(path, addr string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(b)

	// Update source type scoped to rate_limit block
	if strings.Contains(content, "rate_limit:") {
		// Extract rate_limit block, replace type within it, reassemble
		idx := strings.Index(content, "rate_limit:")
		rest := content[idx:]
		if strings.Contains(rest, "type: config") {
			newRest := strings.Replace(rest, "type: config", "type: rule_center", 1)
			content = content[:idx] + newRest
		}
		if strings.Contains(rest, "type: database") {
			newRest := strings.Replace(rest, "type: database", "type: rule_center", 1)
			content = content[:idx] + newRest
		}
	}

	// Add rule_center block if not present
	if !strings.Contains(content, "rule_center:") {
		content = strings.Replace(content, "source:", fmt.Sprintf("rule_center:\n    address: %q\n    query_timeout_milliseconds: 200\n  source:", addr), 1)
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

// wireRuleCenterInServer injects RuleCenter client wiring into the generated
// server.go after the `var rlOpts ratelimit.Options` declaration.
func wireRuleCenterInServer(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read server.go: %w", err)
	}
	content := string(b)

	// Skip if already wired.
	if strings.Contains(content, "middleware.NewRuleCenterClient") {
		return nil
	}

	// Find the rlOpts declaration and inject wiring after it.
	marker := "var rlOpts ratelimit.Options"
	idx := strings.Index(content, marker)
	if idx == -1 {
		// server.go doesn't have the expected rlOpts block — skip gracefully
		return nil
	}

	wiring := `
	if strings.EqualFold(strings.TrimSpace(cfg.RateLimit.Source.Type), "rule_center") {
		rc, err := middleware.NewRuleCenterClient(cfg.RateLimit.RuleCenter.Address)
		if err != nil {
			panic(fmt.Sprintf("rule_center: init client: %v", err))
		}
		defer func() { _ = rc.Close() }()
		rlOpts.RuleCenter = rc
	}
`
	insertPos := idx + len(marker)
	content = content[:insertPos] + wiring + content[insertPos:]

	return os.WriteFile(path, []byte(content), 0o644)
}
