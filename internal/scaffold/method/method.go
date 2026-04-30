// Package method implements marker-based method insertion commands.
package method

import (
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

const (
	LayerUsecase = "usecase"
	startMarker  = "// ncgo:methods:start"
	endMarker    = "// ncgo:methods:end"
)

var (
	domainRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
	methodRE = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]{0,62}$`)
)

type Options struct {
	Root  string // project root containing .ncgo/manifest.yaml
	Spec  string // <domain>.<Method>
	Layer string // currently only "usecase"
}

type Result struct {
	Path   string
	Domain string
	Method string
}

func Add(opts Options) (*Result, error) {
	if opts.Root == "" {
		return nil, errors.New("method: Root is required")
	}
	if opts.Layer == "" {
		opts.Layer = LayerUsecase
	}
	if opts.Layer != LayerUsecase {
		return nil, fmt.Errorf("method: --in %q is invalid (only usecase is supported)", opts.Layer)
	}
	domain, method, err := parseSpec(opts.Spec)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("method: resolve root: %w", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	if !domainListed(m, domain) {
		return nil, fmt.Errorf("method: domain %q is not listed in .ncgo/manifest.yaml", domain)
	}
	path := filepath.Join(root, "internal", "usecase", domain, domain+".go")
	if err := insertUsecaseMethod(path, method); err != nil {
		return nil, err
	}
	return &Result{Path: path, Domain: domain, Method: method}, nil
}

func parseSpec(spec string) (domain, method string, err error) {
	parts := strings.Split(spec, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("method: spec %q must be <domain>.<Method>", spec)
	}
	domain, method = parts[0], parts[1]
	if !domainRE.MatchString(domain) {
		return "", "", fmt.Errorf("method: domain %q must match %s", domain, domainRE)
	}
	if !methodRE.MatchString(method) {
		return "", "", fmt.Errorf("method: method %q must match %s", method, methodRE)
	}
	return domain, method, nil
}

func domainListed(m *manifest.Manifest, domain string) bool {
	for _, d := range m.Domains {
		if d == domain {
			return true
		}
	}
	return false
}

func insertUsecaseMethod(path, method string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("method: read %s: %w", path, err)
	}
	src := string(body)
	if strings.Contains(src, "func (u *UseCase) "+method+"(") {
		return fmt.Errorf("method: %s already exists in %s", method, path)
	}
	start := strings.Index(src, startMarker)
	end := strings.Index(src, endMarker)
	if start < 0 || end < 0 || end < start {
		return fmt.Errorf("method: %s is missing %q/%q markers", path, startMarker, endMarker)
	}
	insertAt := end
	stub := renderUsecaseMethod(method)
	updated := src[:insertAt] + stub + src[insertAt:]
	formatted, err := format.Source([]byte(updated))
	if err != nil {
		return fmt.Errorf("method: format %s: %w", path, err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return fmt.Errorf("method: write %s: %w", path, err)
	}
	return nil
}

func renderUsecaseMethod(method string) string {
	return fmt.Sprintf(`
// %s is a usecase method scaffolded by ncgo.
func (u *UseCase) %s() error {
	return nil
}

`, method, method)
}
