// Package domain implements `ncgo add domain <name>`.
//
// A domain in nc-skills terms is a triple of layers: a usecase, a repository
// port (with a stub implementation), and a samber/do registration. v0.1
// generates the three files verbatim from in-package templates and records
// the domain in .ncgo/manifest.yaml.
//
// When --wire is passed, ncgo also inserts the data.Register<Name>(injector)
// call into cmd/server/main.go after the injector is created, so the user
// does not need to wire by hand.
package domain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
)

// nameRE constrains the domain name to a Go-identifier-safe lowercase form.
// Underscores are allowed because the same string is used as the directory,
// the package name, and the file name; hyphens would force a split between
// directory and package which is a steady source of bugs.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Options configures Add.
type Options struct {
	Root   string // project root containing .ncgo/manifest.yaml
	Name   string // domain name; must satisfy nameRE
	Force  bool   // overwrite existing generated files
	DryRun bool   // report intended writes without modifying files
	Wire   bool   // opt-in: inject data.Register<Name>(injector) into cmd/server/main.go
}

// Result describes what Add produced.
type Result struct {
	WrittenPaths []string // absolute paths created/overwritten by this call
	NextSteps    []string // shell / code edits the caller should perform
	Plan         []planpkg.Item
	Updated      bool // true when manifest.Domains changed
	DryRun       bool // true when no files were modified
}

// Add validates opts, renders the three domain files, and updates the
// manifest. The function never invokes external tools; it is fully
// deterministic given the manifest and Options.
func Add(opts Options) (*Result, error) {
	if err := validateName(opts.Name); err != nil {
		return nil, err
	}
	if opts.Root == "" {
		return nil, errors.New("domain: Root is required")
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("domain: resolve root: %w", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	files := plan(root, m.Module, opts.Name)
	filePlans := make([]planpkg.Item, 0, len(files))
	paths := make([]string, 0, len(files))
	for _, f := range files {
		action, err := plannedFileAction(f.path, opts.Force)
		if err != nil {
			return nil, err
		}
		paths = append(paths, f.path)
		filePlans = append(filePlans, planpkg.Item{Kind: "file", Action: action, Path: f.path})
	}
	written := make([]string, 0, len(files))
	if !opts.DryRun {
		for _, f := range files {
			if err := writeFile(f.path, f.body); err != nil {
				return nil, err
			}
			written = append(written, f.path)
		}
	} else {
		written = paths
	}
	updated := mergeDomain(m, opts.Name)
	if updated && !opts.DryRun {
		if err := manifest.Save(root, m); err != nil {
			return nil, err
		}
	}
	// Wire the Register call into main.go when --wire is passed.
	wired := false
	if opts.Wire && !opts.DryRun {
		if err := wireDomain(root, m.Module, opts.Name); err != nil {
			return nil, fmt.Errorf("domain: wire %s: %w", opts.Name, err)
		}
		wired = true
	}
	next := nextSteps(opts.Name, wired)
	return &Result{
		WrittenPaths: written,
		NextSteps:    next,
		Plan:         buildPlan(filePlans, updated, next, wired, opts.Name),
		Updated:      updated,
		DryRun:       opts.DryRun,
	}, nil
}

func validateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("domain: name %q must match %s", name, nameRE)
	}
	return nil
}

// fileSpec pairs a destination path with its rendered body. Keeping the plan
// separate from the write loop lets us check existence up-front and fail
// before writing anything when --force is not set.
type fileSpec struct {
	path string
	body []byte
}

func plan(root, module, name string) []fileSpec {
	return []fileSpec{
		{
			path: filepath.Join(root, "internal", "usecase", name, name+".go"),
			body: renderUseCase(module, name),
		},
		{
			path: filepath.Join(root, "internal", "repository", name, name+".go"),
			body: renderRepository(name),
		},
		{
			path: filepath.Join(root, "internal", "base", "data", name+"_register.go"),
			body: renderRegister(module, name),
		},
	}
}

func plannedFileAction(path string, force bool) (string, error) {
	if _, err := os.Stat(path); err == nil {
		if !force {
			return "", fmt.Errorf("domain: %s already exists; rerun with --force to overwrite", path)
		}
		return "overwrite", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("domain: stat %s: %w", path, err)
	}
	return "create", nil
}

func writeFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("domain: mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("domain: write %s: %w", path, err)
	}
	return nil
}

// mergeDomain appends name to m.Domains if missing and keeps the slice sorted.
// Mirrors the behaviour of internal/scaffold/infra.mergeInfra so that golden
// outputs stay deterministic.
func mergeDomain(m *manifest.Manifest, name string) bool {
	for _, d := range m.Domains {
		if d == name {
			return false
		}
	}
	m.Domains = append(m.Domains, name)
	sort.Strings(m.Domains)
	return true
}

func nextSteps(name string, wired bool) []string {
	export := exportName(name)
	steps := []string{"go mod tidy"}
	if !wired {
		steps = append([]string{
			fmt.Sprintf("wire %s into cmd/server/main.go: data.Register%s(injector)", name, export),
		}, steps...)
	}
	return steps
}

func buildPlan(filePlans []planpkg.Item, manifestUpdated bool, next []string, wired bool, name string) []planpkg.Item {
	items := append([]planpkg.Item(nil), filePlans...)
	manifestAction := "already_present"
	if manifestUpdated {
		manifestAction = "add"
	}
	items = append(items, planpkg.Item{Kind: "manifest", Action: manifestAction, Path: filepath.Join(".ncgo", "manifest.yaml")})
	if wired {
		items = append(items, planpkg.Item{Kind: "wire", Action: "insert", Path: filepath.Join("cmd", "server", "main.go"), Detail: "data.Register" + exportName(name) + "(injector)"})
	}
	for _, step := range next {
		items = append(items, planpkg.Item{Kind: "next_step", Action: "run", Detail: step})
	}
	return items
}

// wireDomain inserts data.Register<Name>(injector) into cmd/server/main.go
// after the injector is created. It is called only when --wire is passed.
func wireDomain(root, module, name string) error {
	mainPath := filepath.Join(root, "cmd", "server", "main.go")
	body, err := os.ReadFile(mainPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("wire: %s not found; run the generator first (e.g. hz/kitex)", mainPath)
		}
		return fmt.Errorf("wire: read %s: %w", mainPath, err)
	}
	src := string(body)

	// Marker-based insertion: look for // ncgo:wire:domain marker first,
	// then fall back to injector := do.New() anchor.
	marker := "// ncgo:wire:domain"
	injectorAnchor := "injector := do.New()"
	registerStmt := fmt.Sprintf("\tdata.Register%s(injector)", exportName(name))

	// Check if already wired.
	if strings.Contains(src, registerStmt) {
		return nil // already wired; idempotent
	}

	var updated string
	if idx := strings.Index(src, marker); idx >= 0 {
		// Insert after the marker line.
		nl := strings.Index(src[idx:], "\n")
		if nl < 0 {
			nl = len(src[idx:])
		}
		updated = src[:idx+nl+1] + registerStmt + "\n" + src[idx+nl+1:]
	} else if idx := strings.Index(src, injectorAnchor); idx >= 0 {
		// Insert after the injector := do.New() line.
		nl := strings.Index(src[idx:], "\n")
		if nl < 0 {
			nl = len(src[idx:])
		}
		updated = src[:idx+nl+1] + registerStmt + "\n" + src[idx+nl+1:]
	} else {
		return fmt.Errorf("wire: could not find injection point in %s; add // ncgo:wire:domain marker or ensure injector := do.New() exists", mainPath)
	}

	// Ensure the data package import exists.
	dataPkg := fmt.Sprintf("data \"%s/internal/base/data\"", module)
	if !strings.Contains(updated, dataPkg) {
		updated = addGoImport(updated, dataPkg)
	}

	if err := os.WriteFile(mainPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("wire: write %s: %w", mainPath, err)
	}
	return nil
}

// addGoImport adds a Go import if not already present, inserting it after
// the last existing import or after the "package main" line.
func addGoImport(src, imp string) string {
	if strings.Contains(src, imp) {
		return src
	}
	// Find import block or just package line.
	importStart := strings.Index(src, "import (")
	if importStart >= 0 {
		// Insert before the closing ) of the import block.
		closeParen := strings.Index(src[importStart:], ")")
		if closeParen >= 0 {
			return src[:importStart+closeParen] + "\t" + imp + "\n" + src[importStart+closeParen:]
		}
	}
	// No import block found - insert after "package main\n"
	pkgLine := strings.Index(src, "\n")
	if pkgLine >= 0 {
		return src[:pkgLine+1] + fmt.Sprintf("\nimport %q\n", imp) + src[pkgLine+1:]
	}
	return src
}
