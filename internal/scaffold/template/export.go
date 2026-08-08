package template

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileRule describes how a file path pattern maps to a template.
type FileRule struct {
	Pattern        string // glob pattern
	UpdateBehavior string // "skip" or "cover"
	LoopService    bool   // per-service generation
}

// HertzRules returns the file mapping rules for Hertz projects.
func HertzRules() []FileRule {
	return []FileRule{
		{Pattern: "main.go", UpdateBehavior: "cover"},
		{Pattern: "conf/dev/conf.yaml", UpdateBehavior: "skip"},
		{Pattern: "internal/base/conf/conf.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/data/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/server/server.go", UpdateBehavior: "cover"},
		{Pattern: "internal/handler/**/*.go", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/usecase/**/*.go", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/repository/**/*.go", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/router/**/*.go", UpdateBehavior: "cover", LoopService: true},
		{Pattern: "internal/pkg/**/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/logging/*.go", UpdateBehavior: "cover"},
	}
}

// KitexRules returns the file mapping rules for Kitex projects.
func KitexRules() []FileRule {
	return []FileRule{
		{Pattern: "main.go", UpdateBehavior: "cover"},
		{Pattern: "conf/dev/conf.yaml", UpdateBehavior: "skip"},
		{Pattern: "internal/base/conf/conf.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/data/data.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/server/server.go", UpdateBehavior: "cover"},
		{Pattern: "internal/handler/**/*.go", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/usecase/**/*.go", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "internal/repository/**/*.go", UpdateBehavior: "skip", LoopService: true},
		{Pattern: "pkg/client/**/*.go", UpdateBehavior: "cover", LoopService: true},
		{Pattern: "internal/pkg/**/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/middleware/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/release/*.go", UpdateBehavior: "cover"},
		{Pattern: "internal/base/logging/*.go", UpdateBehavior: "cover"},
	}
}

// ExcludedPaths returns paths that should never be exported as templates.
var ExcludedPaths = []string{
	"internal/pb/", // hz-generated protobuf code
	"kitex_gen/",   // kitex-generated RPC stubs
}

// ExportOptions describes an export operation.
type ExportOptions struct {
	Root        string // project root
	Kind        string // "hertz" or "kitex"
	Module      string // Go module path
	ServiceName string // service name from manifest
}

// ExportResult describes what was exported.
type ExportResult struct {
	OutputDir string
	Templates []string
	IDLs      []string // relative idl/... paths with service name parameterized
}

// Export extracts code templates from an existing ncgo project.
func Export(opts ExportOptions) (*ExportResult, error) {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	rules := HertzRules()
	tplDir := "hertz-template"
	if opts.Kind == "kitex" {
		rules = KitexRules()
		tplDir = "kitex-template"
	}

	outDir := filepath.Join(absRoot, "template", tplDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	svcNameLower := serviceNameLower(opts.ServiceName)

	var templates []string

	for _, rule := range rules {
		files, err := matchFiles(absRoot, rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("match %s: %w", rule.Pattern, err)
		}
		for _, f := range files {
			rel := relPath(absRoot, f)
			if isExcluded(rel) {
				continue
			}
			tpl, err := fileToTemplate("", f, rel, opts, rule, svcNameLower)
			if err != nil {
				return nil, fmt.Errorf("process %s: %w", rel, err)
			}
			if tpl == nil {
				continue
			}
			if err := writeTemplateYAML(outDir, tpl); err != nil {
				return nil, fmt.Errorf("write template %s: %w", tpl.Path, err)
			}
			templates = append(templates, tpl.Path)
		}
	}

	// Export Makefile if it exists
	if mk, err := makefileTemplate(absRoot, opts); err == nil && mk != nil {
		if err := writeTemplateYAML(outDir, mk); err != nil {
			return nil, fmt.Errorf("write makefile template: %w", err)
		}
		templates = append(templates, mk.Path)
	}

	idls, err := exportIDLs(absRoot, opts)
	if err != nil {
		return nil, fmt.Errorf("export idl: %w", err)
	}

	return &ExportResult{
		OutputDir: relPath(absRoot, outDir),
		Templates: templates,
		IDLs:      idls,
	}, nil
}

// exportIDLs variabilizes the project's service IDL into template/idl/.
// hz standard support files (openapi/, validate/) stay embedded and are
// excluded. A missing idl/ dir is not an error.
func exportIDLs(root string, opts ExportOptions) ([]string, error) {
	idlRoot := filepath.Join(root, "idl")
	if fi, err := os.Stat(idlRoot); err != nil || !fi.IsDir() {
		return nil, nil
	}
	var exported []string
	err := filepath.Walk(idlRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel := relPath(idlRoot, path)
		if strings.HasPrefix(rel, "openapi/") || strings.HasPrefix(rel, "validate/") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		body := regexp.MustCompile(regexp.QuoteMeta(opts.Module)).ReplaceAllString(string(content), "{{.Module}}")
		body = replaceServiceName(body, opts.ServiceName)
		tplRel := idlTemplatePath(rel, opts)
		out := filepath.Join(root, "template", "idl", filepath.FromSlash(tplRel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
			return err
		}
		exported = append(exported, "idl/"+tplRel)
		return nil
	})
	return exported, err
}

// idlTemplatePath parameterizes the service name inside IDL file names so
// consumers render them onto their own default IDL paths (hertz
// idl/app/<name>.proto, kitex idl/<name>.proto). Substitution is scoped to the
// file base only: parent directories (e.g. the fixed "app" dir for hertz) are
// left literal even when they happen to equal the lowercase service name.
func idlTemplatePath(rel string, opts ExportOptions) string {
	dashed := strings.ToLower(opts.ServiceName) // "user-rpc"
	lower := serviceNameLower(opts.ServiceName) // "userrpc"
	dir, base := path.Split(rel)
	if dashed != "" {
		base = strings.ReplaceAll(base, dashed, "{{ToLower .ServiceName}}")
	}
	if lower != "" && lower != dashed {
		base = strings.ReplaceAll(base, lower, "{{ToLower .ServiceName}}")
	}
	return dir + base
}

func matchFiles(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel := relPath(root, path)
		if globMatch(pattern, rel) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

// globMatch handles ** globstar patterns that filepath.Match does not support.
func globMatch(pattern, path string) bool {
	if strings.Contains(pattern, "**") {
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")
		if prefix != "" && !strings.HasPrefix(path, prefix+"/") && path != prefix {
			return false
		}
		if suffix != "" {
			matched, _ := filepath.Match(suffix, filepath.Base(path))
			return matched
		}
		return true
	}
	matched, _ := filepath.Match(pattern, path)
	return matched
}

func isExcluded(rel string) bool {
	for _, prefix := range ExcludedPaths {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func fileToTemplate(_ string, absPath, relPath string, opts ExportOptions, rule FileRule, svcNameLower string) (*TemplateFile, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	body := string(content)

	// Replace module path with {{.Module}}
	re := regexp.MustCompile(regexp.QuoteMeta(opts.Module))
	body = re.ReplaceAllString(body, "{{.Module}}")

	// Replace service name identifiers
	body = replaceServiceName(body, opts.ServiceName)

	// Compute template path
	tplPath := templatePath(relPath, rule, svcNameLower)

	return &TemplateFile{
		Path:           tplPath,
		UpdateBehavior: UpdateBehavior{Type: rule.UpdateBehavior},
		LoopService:    rule.LoopService,
		Body:           body,
	}, nil
}

// replaceServiceName replaces service name identifiers with template variables.
func replaceServiceName(body, serviceName string) string {
	export := exportName(serviceName)

	// Replace PascalCase type identifiers: UserApi → {{.ServiceName}}
	// Handle cases like UserApiImpl → {{.ServiceName}}Impl
	if export != "" {
		// Match export name followed by: end of string, non-letter, or uppercase letter (start of next word)
		pattern := regexp.QuoteMeta(export) + `($|[^a-zA-Z]|[A-Z])`
		typeRE := regexp.MustCompile(pattern)
		body = typeRE.ReplaceAllStringFunc(body, func(s string) string {
			suffix := s[len(export):]
			return "{{.ServiceName}}" + suffix
		})
	}

	// Replace lowercase service name occurrences that appear as bounded
	// tokens: path segments, package declarations, package qualifiers,
	// and quoted import paths. Non-boundary occurrences (userrpc2,
	// myuserrpc) are left untouched.
	lower := serviceNameLower(serviceName)
	if lower != "" {
		segRE := regexp.MustCompile(`(^|[\s/."'])` + regexp.QuoteMeta(lower) + `($|[\s/."'])`)
		body = segRE.ReplaceAllString(body, "${1}{{ToLower .ServiceName}}${2}")
	}

	return body
}

func templatePath(rel string, rule FileRule, svcNameLower string) string {
	if rule.LoopService && svcNameLower != "" {
		return strings.Replace(rel, "/"+svcNameLower+"/", "/{{ToLower .ServiceName}}/", 1)
	}
	return rel
}

func makefileTemplate(root string, opts ExportOptions) (*TemplateFile, error) {
	mkPath := filepath.Join(root, "Makefile")
	content, err := os.ReadFile(mkPath)
	if err != nil {
		return nil, err
	}

	body := string(content)
	re := regexp.MustCompile(regexp.QuoteMeta(opts.Module))
	body = re.ReplaceAllString(body, "{{.Module}}")
	body = replaceServiceName(body, opts.ServiceName)

	return &TemplateFile{
		Path:           "Makefile",
		UpdateBehavior: UpdateBehavior{Type: "cover"},
		Body:           body,
	}, nil
}

func writeTemplateYAML(outDir string, tpl *TemplateFile) error {
	yamlName := yamlFileName(tpl.Path)
	data, err := yaml.Marshal(tpl)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	comment := "# ncgo exported template — " + tpl.Path + "\n"
	data = append([]byte(comment), data...)

	return os.WriteFile(filepath.Join(outDir, yamlName), data, 0644)
}

func yamlFileName(path string) string {
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "")
	if !strings.HasSuffix(name, ".yaml") {
		name += ".yaml"
	}
	return name
}

func relPath(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}

// serviceNameLower converts a service name to its lowercase form used in paths.
func serviceNameLower(name string) string {
	out := strings.ToLower(name)
	out = strings.ReplaceAll(out, "-", "")
	return out
}
