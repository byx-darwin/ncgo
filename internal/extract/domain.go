// Package extract plans future mono-to-micro extraction operations.
package extract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

var domainRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type DomainOptions struct {
	Root string
	Name string
	To   string
}

type DomainPlan struct {
	Root         string
	Name         string
	To           string
	TargetModule string
	Sources      []PlannedFile
	Written      []string
	Applied      bool
	NextSteps    []string
}

type PlannedFile struct {
	Role string
	From string
	To   string
}

func PlanDomain(opts DomainOptions) (*DomainPlan, error) {
	if opts.Root == "" {
		return nil, errors.New("extract: Root is required")
	}
	if !domainRE.MatchString(opts.Name) {
		return nil, fmt.Errorf("extract: domain %q must match %s", opts.Name, domainRE)
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("extract: resolve root: %w", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	if m.Mode != manifest.ModeMono {
		return nil, fmt.Errorf("extract: root mode %q is not mono", m.Mode)
	}
	if !contains(m.Domains, opts.Name) {
		return nil, fmt.Errorf("extract: domain %q is not listed in .ncgo/manifest.yaml", opts.Name)
	}
	to := opts.To
	if to == "" {
		to = filepath.Join("services", opts.Name+"-rpc")
	}
	if filepath.IsAbs(to) {
		return nil, errors.New("extract: --to must be relative to root")
	}
	to = filepath.Clean(to)
	if to == "." || to == ".." || strings.HasPrefix(to, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("extract: --to %q must stay under root", opts.To)
	}
	sources := plannedSources(root, filepath.ToSlash(to), opts.Name)
	missing := firstMissing(sources)
	if missing != "" {
		return nil, fmt.Errorf("extract: source file %s is missing", missing)
	}
	// --plan is preview-only: the target service may not exist yet, so prefer
	// its real manifest module when available and otherwise fall back to the
	// derived sourceModule/to value.
	targetModule := strings.TrimRight(m.Module, "/") + "/" + filepath.ToSlash(to)
	if tm, err := manifest.Load(filepath.Join(root, filepath.FromSlash(to))); err == nil && tm.Module != "" {
		targetModule = tm.Module
	}
	return &DomainPlan{
		Root:         root,
		Name:         opts.Name,
		To:           filepath.ToSlash(to),
		TargetModule: targetModule,
		Sources:      sources,
		NextSteps: []string{
			"create target RPC service with `ncgo add rpc " + opts.Name + "-rpc --root <workspace>`",
			"run `ncgo extract domain " + opts.Name + " --root <mono> --to " + filepath.ToSlash(to) + " --apply` to copy planned files",
			"wire clients and update imports manually; automatic migration is future work",
		},
	}, nil
}

func ApplyDomain(opts DomainOptions) (*DomainPlan, error) {
	plan, err := PlanDomain(opts)
	if err != nil {
		return nil, err
	}
	sourceManifest, err := manifest.Load(plan.Root)
	if err != nil {
		return nil, err
	}
	targetRoot := filepath.Join(plan.Root, filepath.FromSlash(plan.To))
	targetManifest, err := manifest.Load(targetRoot)
	if err != nil {
		return nil, fmt.Errorf("extract: target service manifest is required under %s; create the target RPC service first: %w", targetRoot, err)
	}
	if targetManifest.Service.Kind != manifest.KindKitex {
		return nil, fmt.Errorf("extract: target service kind %q is not kitex", targetManifest.Service.Kind)
	}
	if existing := firstExistingTarget(plan.Root, plan.Sources); existing != "" {
		return nil, fmt.Errorf("extract: target file %s already exists", existing)
	}
	plan.TargetModule = targetManifest.Module
	plan.Applied = true
	plan.NextSteps = []string{
		"review copied files and keep only the domain code that belongs in the RPC service",
		"wire RPC clients / handlers and update cross-service imports manually",
		"run `go mod tidy` and tests in both source and target services",
	}
	for _, f := range plan.Sources {
		dst := filepath.Join(plan.Root, filepath.FromSlash(f.To))
		if err := copyWithModuleRewrite(f.From, dst, sourceManifest.Module, targetManifest.Module); err != nil {
			return nil, err
		}
		plan.Written = append(plan.Written, filepath.ToSlash(f.To))
	}
	return plan, nil
}

func plannedSources(root, to, name string) []PlannedFile {
	return []PlannedFile{
		{
			Role: "usecase",
			From: filepath.Join(root, "internal", "usecase", name, name+".go"),
			To:   filepath.ToSlash(filepath.Join(to, "internal", "usecase", name, name+".go")),
		},
		{
			Role: "repository",
			From: filepath.Join(root, "internal", "repository", name, name+".go"),
			To:   filepath.ToSlash(filepath.Join(to, "internal", "repository", name, name+".go")),
		},
		{
			Role: "register",
			From: filepath.Join(root, "internal", "base", "data", name+"_register.go"),
			To:   filepath.ToSlash(filepath.Join(to, "internal", "base", "data", name+"_register.go")),
		},
	}
}

func firstMissing(files []PlannedFile) string {
	for _, f := range files {
		if _, err := os.Stat(f.From); err != nil {
			return f.From
		}
	}
	return ""
}

func firstExistingTarget(root string, files []PlannedFile) string {
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f.To))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func copyWithModuleRewrite(from, to, sourceModule, targetModule string) error {
	body, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("extract: read %s: %w", from, err)
	}
	if sourceModule != "" && targetModule != "" && sourceModule != targetModule {
		body = []byte(strings.ReplaceAll(string(body), strings.TrimRight(sourceModule, "/"), strings.TrimRight(targetModule, "/")))
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("extract: mkdir %s: %w", filepath.Dir(to), err)
	}
	if err := os.WriteFile(to, body, 0o644); err != nil {
		return fmt.Errorf("extract: write %s: %w", to, err)
	}
	return nil
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
