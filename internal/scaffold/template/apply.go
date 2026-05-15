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
	Root         string         // project root (after hz new)
	Module       string         // Go module path
	ServiceName  string         // service name
	WithDatabase bool           // database enabled flag
	Infra        []string       // infra add-ons
	Services     []ServiceInfo  // parsed from proto (all services)
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

func applySingle(root string, tpl *TemplateFile, opts ApplyOptions, result *ApplyResult) error {
	// If loop_service is true, generate one file per proto service.
	if tpl.LoopService && len(opts.Services) > 0 {
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
	targetPath := tpl.Path
	targetPath = strings.ReplaceAll(targetPath, "{{ToLower .ServiceName}}", strings.ToLower(si.ServiceName))

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
