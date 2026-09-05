package framework

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(kitexAdapter{}) }

type kitexAdapter struct{}

func (kitexAdapter) Kind() string { return manifest.KindKitex }

func (kitexAdapter) OptionalAssetPath(infraKind string) (string, bool) {
	return "kitex/optional/" + infraKind + ".go", true
}

func (kitexAdapter) HertzConfigAssetPath(infraKind string) (string, bool) {
	return "", false
}

func (kitexAdapter) MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil // Hertz-only in current behavior.
}

func (kitexAdapter) MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error) {
	if current == nil {
		return &ConfigWrite{Body: []byte(defaultRateLimitConfBlock()), Action: "create"}, nil
	}
	merged, changed := mergeKitexRateLimitConfig(string(current))
	if !changed {
		return nil, nil
	}
	return &ConfigWrite{Body: []byte(merged), Action: "update"}, nil
}

// defaultRateLimitConfBlock/mergeKitexRateLimitConfig are moved verbatim from
// internal/scaffold/infra/infra.go:724-816.
func defaultRateLimitConfBlock() string {
	return `rate_limit:
  enabled: true
  mode: shadow
  backend: memory
  fail_open: true
  source:
    type: config
    cache_ttl_seconds: 60s
    fallback_on_error: true
  static:
    max_qps: 0
    max_connections: 0
`
}

func mergeKitexRateLimitConfig(src string) (string, bool) {
	if !hasTopLevelConfigKey(src, "rate_limit") {
		trimmed := strings.TrimRight(src, "\n")
		if trimmed == "" {
			return defaultRateLimitConfBlock(), true
		}
		return trimmed + "\n\n" + defaultRateLimitConfBlock(), true
	}
	lines := strings.Split(src, "\n")
	startIdx := -1
	endIdx := len(lines)
	childIndent := -1
	for i, line := range lines {
		if startIdx == -1 {
			if strings.HasPrefix(line, "rate_limit:") {
				startIdx = i
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			endIdx = i
			break
		}
		if childIndent == -1 {
			childIndent = len(line) - len(strings.TrimLeft(line, " \t"))
		}
	}
	if startIdx == -1 {
		return src, false
	}
	changed := false
	for i := startIdx + 1; i < endIdx; i++ {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if childIndent >= 0 && indent != childIndent {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "enabled:") {
			if !strings.Contains(trimmed, "true") {
				indentStr := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indentStr + "enabled: true"
				changed = true
			}
		} else if strings.HasPrefix(trimmed, "mode:") {
			if !strings.Contains(trimmed, "shadow") {
				indentStr := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indentStr + "mode: shadow"
				changed = true
			}
		}
	}
	if !changed {
		return src, false
	}
	return strings.Join(lines, "\n"), true
}

func (kitexAdapter) DockerConfigBlocks(m *manifest.Manifest) []string {
	if !m.Service.WithDatabase {
		return nil
	}
	return []string{fmt.Sprintf("database:\n  enabled: true\n  dsn: %q", postgresDSN(m.Service.Name))}
}

func (kitexAdapter) ContainerPort() int { return 8888 }

func (kitexAdapter) ComposeFeatures(withDatabase bool) ComposeFeatureFlags {
	return ComposeFeatureFlags{}
}

func (kitexAdapter) IDLPath(opts GeneratorOptions) string {
	return filepath.ToSlash(filepath.Join("idl", kitexIDLBase(opts)+".proto"))
}

// kitexIDLBase matches the generated Makefile's
// `{{.ServiceInfo.ServiceName | ToLower}}` convention, and avoids invalid
// proto/Go identifiers for CLI names like "user-api".
func kitexIDLBase(opts GeneratorOptions) string {
	return strings.ToLower(exportName(opts.Name))
}

// exportName converts "user-api" to "UserApi" for use as a proto service
// name. Shared by both adapters' generated-name logic.
func exportName(s string) string {
	out := make([]byte, 0, len(s))
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '_' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
		upper = false
	}
	return string(out)
}

func (kitexAdapter) IDLNameToken(opts GeneratorOptions) string {
	return kitexIDLBase(opts)
}

func (kitexAdapter) GeneratorCommand(opts GeneratorOptions, idl string) string {
	return fmt.Sprintf("kitex -module %s -template-dir template/kitex-template -type protobuf %s", opts.Module, idl)
}

func (kitexAdapter) RenderIDLPlaceholder(opts GeneratorOptions) string {
	base := kitexIDLBase(opts)
	service := exportName(opts.Name)
	return fmt.Sprintf(`syntax = "proto3";

package %s;

option go_package = "%s/kitex_gen/%s;%s";

service %s {
}
`, base, opts.Module, base, base, service)
}

func (kitexAdapter) WriteIDLSupportFiles(dir string) error { return nil }

func (kitexAdapter) RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error) {
	return exec.Kitex(ctx, r, dir, kitexArgs(opts, idl)...)
}

// kitexArgs mirrors the invocation documented in
// internal/assets/_data/kitex/kitex-template/makefile.yaml so `make update`
// later produces the same files.
func kitexArgs(opts GeneratorOptions, idl string) []string {
	return []string{
		"-module", opts.Module,
		"-template-dir", "template/kitex-template",
		"-type", "protobuf",
		idl,
	}
}

func (kitexAdapter) RequiresSQLCBeforeTidy(withDatabase bool) bool { return true }

func (kitexAdapter) ServerFilePath() string {
	return "internal/base/server/server.go"
}
