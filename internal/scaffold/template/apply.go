package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ApplyOptions describes a template apply operation.
type ApplyOptions struct {
	Root         string        // project root (after hz new)
	Module       string        // Go module path
	ServiceName  string        // service name
	WithDatabase bool          // database enabled flag
	Infra        []string      // infra add-ons
	Services     []ServiceInfo // parsed from proto (all services)
}

// ApplyResult describes what was applied.
type ApplyResult struct {
	Written []string
	Skipped []string
}

// Apply reads templates from template/hertz-template/ and applies them
// to the project tree.
func Apply(opts ApplyOptions) (*ApplyResult, error) {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	tplDir := filepath.Join(absRoot, "template", "hertz-template")
	entries, err := os.ReadDir(tplDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &ApplyResult{}, nil
		}
		return nil, fmt.Errorf("read template dir: %w", err)
	}

	var result ApplyResult

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		tplPath := filepath.Join(tplDir, e.Name())
		tpl, err := readTemplateYAML(tplPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", e.Name(), err)
		}
		if err := applySingle(absRoot, tpl, opts, &result); err != nil {
			return nil, fmt.Errorf("apply template %s: %w", tpl.Path, err)
		}
	}

	return &result, nil
}

func readTemplateYAML(path string) (*TemplateFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tpl TemplateFile
	if err := yaml.Unmarshal(content, &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	return &tpl, nil
}

// conditionMet evaluates a template condition string against opts.
// An empty condition always returns true.
func conditionMet(cond string, opts ApplyOptions) bool {
	switch cond {
	case "":
		return true
	case "WithDatabase":
		return opts.WithDatabase
	default:
		return true // unknown conditions are permissive
	}
}

func applySingle(root string, tpl *TemplateFile, opts ApplyOptions, result *ApplyResult) error {
	// Skip the entire template when its condition is not met.
	if !conditionMet(tpl.Condition, opts) {
		result.Skipped = append(result.Skipped, tpl.Path)
		return nil
	}

	// If loop_service is true, generate one file per proto service.
	// When no services were parsed, skip entirely — falling through to
	// the non-loop path would render with an empty ServiceName, producing
	// broken Go files (e.g. "package " with no name).
	if tpl.LoopService {
		for _, si := range opts.Services {
			if err := applyForService(root, tpl, opts, si, result); err != nil {
				return err
			}
		}
		return nil
	}

	// Non-loop templates: render once with the first service (or empty ServiceInfo).
	var si ServiceInfo
	if len(opts.Services) > 0 {
		si = opts.Services[0]
	}
	return applyForService(root, tpl, opts, si, result)
}

func applyForService(root string, tpl *TemplateFile, opts ApplyOptions, si ServiceInfo, result *ApplyResult) error {
	// Resolve the actual target path — replace template variables in path.
	// Use snake_case naming consistent with hz generator (kebab-case → snake_case).
	// opts.ServiceName is the kebab-case name from CLI (e.g., "test-dup-verify").
	// hz converts this to snake_case (e.g., "test_dup_verify") for file naming.
	targetPath := tpl.Path
	snakeName := toSnakeCase(opts.ServiceName)
	targetPath = strings.ReplaceAll(targetPath, "{{ToLower .ServiceName}}", snakeName)

	rendered, err := Render(tpl.Body, RenderData{
		Module:       opts.Module,
		ServiceName:  si.ServiceName,
		ServiceInfo:  si,
		WithDatabase: opts.WithDatabase,
		Infra:        opts.Infra,
	})
	if err != nil {
		return err
	}

	targetPath = filepath.Join(root, targetPath)
	return writeOrSkip(root, targetPath, rendered, tpl.UpdateBehavior.Type, result)
}

// toSnakeCase converts kebab-case or PascalCase to snake_case.
// Examples: "test-dup-verify" → "test_dup_verify", "TestDupVerify" → "test_dup_verify"
func toSnakeCase(s string) string {
	// First replace hyphens with underscores
	s = strings.ReplaceAll(s, "-", "_")
	// Then handle PascalCase by inserting underscores before uppercase letters
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Check if previous char is lowercase or next char is lowercase
			prev := rune(s[i-1])
			if (prev >= 'a' && prev <= 'z') || (i+1 < len(s) && rune(s[i+1]) >= 'a' && rune(s[i+1]) <= 'z') {
				result.WriteRune('_')
			}
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func writeOrSkip(_ string, targetPath, content, behavior string, result *ApplyResult) error {
	if behavior == "skip" {
		if _, err := os.Stat(targetPath); err == nil {
			result.Skipped = append(result.Skipped, targetPath)
			return nil
		}
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", targetPath, err)
	}
	result.Written = append(result.Written, targetPath)
	return nil
}
