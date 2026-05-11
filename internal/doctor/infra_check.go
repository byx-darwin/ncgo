package doctor

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"gopkg.in/yaml.v3"
)

// infraFiles maps each infra kind to the expected generated file path.
var infraFiles = map[string]string{
	"redis":      filepath.Join("internal", "base", "data", "redis.go"),
	"kafka":      filepath.Join("internal", "base", "data", "kafka.go"),
	"es":         filepath.Join("internal", "base", "data", "es.go"),
	"clickhouse": filepath.Join("internal", "base", "data", "clickhouse.go"),
	"nacos":      filepath.Join("internal", "base", "data", "nacos.go"),
	"polaris":    filepath.Join("internal", "base", "data", "polaris.go"),
}

// checkInfraFiles verifies that every infra backend referenced in the config
// and every kind listed in the manifest has a corresponding generated file on
// disk.
func checkInfraFiles(root string, m *manifest.Manifest) []Check {
	var out []Check

	// 1. Manifest infra list → file existence
	for _, kind := range m.Infra {
		out = append(out, infraFileCheck(root, kind, "listed in manifest"))
	}

	// 2. Config YAML backend references → file existence
	cfgPath := filepath.Join(root, "conf", "dev", "conf.yaml")
	needed := scanConfigInfraNeeds(cfgPath)
	for kind, reason := range needed {
		if manifestHasKind(m, kind) {
			continue // already reported by manifest check
		}
		out = append(out, infraFileCheck(root, kind, "referenced by "+reason))
	}

	return out
}

func infraFileCheck(root, kind, reason string) Check {
	rel, ok := infraFiles[kind]
	if !ok {
		return Check{ID: "infra.file." + kind, Severity: SeverityWarn, OK: true, Message: kind + " is not a known infra kind (skipped)"}
	}
	path := filepath.Join(root, rel)
	c := Check{ID: "infra.file." + kind, Severity: SeverityError, File: path}
	if _, err := fileStatForInfra(path); err == nil {
		c.OK = true
		c.Message = kind + " infra file exists at " + rel + " (" + reason + ")"
	} else {
		c.OK = false
		c.Message = kind + " infra file " + rel + " is missing (" + reason + ")"
		c.Hint = "run `ncgo add infra " + kind + "` then `go mod tidy`"
	}
	return c
}

func scanConfigInfraNeeds(path string) map[string]string {
	needs := map[string]string{}
	body, err := os.ReadFile(path)
	if err != nil {
		return needs
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return needs
	}

	// rate_limit.backend == "redis"
	if rl, ok := cfg["rate_limit"].(map[string]any); ok {
		if backend, _ := rl["backend"].(string); backend == "redis" {
			needs["redis"] = "rate_limit.backend"
		}
	}

	// auth.signature.nonce.backend == "redis"
	if auth, ok := cfg["auth"].(map[string]any); ok {
		if sig, ok := auth["signature"].(map[string]any); ok {
			if nonce, ok := sig["nonce"].(map[string]any); ok {
				if backend, _ := nonce["backend"].(string); backend == "redis" {
					needs["redis"] = "auth.signature.nonce.backend"
				}
			}
		}
	}

	// idempotency.backend == "redis"
	if idem, ok := cfg["idempotency"].(map[string]any); ok {
		if backend, _ := idem["backend"].(string); backend == "redis" {
			needs["redis"] = "idempotency.backend"
		}
	}

	// config_center.enabled + provider → nacos/polaris
	if cc, ok := cfg["config_center"].(map[string]any); ok {
		if enabled, _ := cc["enabled"].(bool); enabled {
			if provider, _ := cc["provider"].(string); provider == "nacos" || provider == "polaris" {
				needs[provider] = "config_center.provider"
			}
		}
	}

	return needs
}

func manifestHasKind(m *manifest.Manifest, kind string) bool {
	return slices.Contains(m.Infra, kind)
}

// fileStatForInfra is the file stat function used by infra checks. Split out so
// tests can swap it.
var fileStatForInfra = func(p string) (fs.FileInfo, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return info, nil
}
