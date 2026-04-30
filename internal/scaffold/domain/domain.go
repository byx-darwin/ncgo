// Package domain implements `ncgo add domain <name>`.
//
// A domain in nc-skills terms is a triple of layers: a usecase, a repository
// port (with a stub implementation), and a samber/do registration. v0.1
// generates the three files verbatim from in-package templates and records
// the domain in .ncgo/manifest.yaml. AST-level wiring of the domain into
// cmd/server/main.go is deferred to v0.3 (PRD §7); the user is expected to
// call Register<Name>() from their bootstrap once.
package domain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// nameRE constrains the domain name to a Go-identifier-safe lowercase form.
// Underscores are allowed because the same string is used as the directory,
// the package name, and the file name; hyphens would force a split between
// directory and package which is a steady source of bugs.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Options configures Add.
type Options struct {
	Root  string // project root containing .ncgo/manifest.yaml
	Name  string // domain name; must satisfy nameRE
	Force bool   // overwrite existing generated files
}

// Result describes what Add produced.
type Result struct {
	WrittenPaths []string // absolute paths created/overwritten by this call
	NextSteps    []string // shell / code edits the caller should perform
	Updated      bool     // true when manifest.Domains changed
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
	if !opts.Force {
		if existing := firstExisting(files); existing != "" {
			return nil, fmt.Errorf("domain: %s already exists; rerun with --force to overwrite", existing)
		}
	}
	written := make([]string, 0, len(files))
	for _, f := range files {
		if err := writeFile(f.path, f.body); err != nil {
			return nil, err
		}
		written = append(written, f.path)
	}
	updated := mergeDomain(m, opts.Name)
	if updated {
		if err := manifest.Save(root, m); err != nil {
			return nil, err
		}
	}
	return &Result{
		WrittenPaths: written,
		NextSteps:    nextSteps(opts.Name),
		Updated:      updated,
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

func firstExisting(files []fileSpec) string {
	for _, f := range files {
		if _, err := os.Stat(f.path); err == nil {
			return f.path
		}
	}
	return ""
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

func nextSteps(name string) []string {
	export := exportName(name)
	return []string{
		fmt.Sprintf("wire %s into cmd/server/main.go: data.Register%s(injector)", name, export),
		"go mod tidy",
	}
}
