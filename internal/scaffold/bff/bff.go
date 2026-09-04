// Package bff implements `ncgo add bff <name>` for micro workspaces.
package bff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/mono"
	planpkg "github.com/byx-darwin/ncgo/internal/scaffold/plan"
	"github.com/byx-darwin/ncgo/internal/scaffold/shared"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Options configures Add.
type Options struct {
	Root          string      // micro workspace root containing ncgo.workspace
	Name          string      // BFF service name
	Module        string      // Go module path; defaults to <workspace.module>/<service dir>
	Dir           string      // service dir relative to Root; defaults to services/<name>
	AssetsVersion string      // recorded into the service manifest
	NCGOVersion   string      // recorded into the service manifest
	NoGenerate    bool        // skip hz invocation
	DryRun        bool        // report intended service writes without modifying files
	Preset        string      // preset template name (e.g., "rule-center")
	TemplateDir   string      // external template package dir; replaces embedded hertz-template and IDL placeholder
	Runner        exec.Runner // injected exec; nil means mono default
	Now           time.Time   // injected clock for tests
}

// Result describes what Add produced.
type Result struct {
	ServiceDir  string
	ServiceRel  string
	Module      string
	NextSteps   []string
	Plan        []planpkg.Item
	Updated     bool
	RanGenerate bool
	DryRun      bool
}

// Add creates a Hertz BFF service under a micro workspace and records it in
// ncgo.workspace. It delegates the service scaffold itself to mono.Generate.
func Add(ctx context.Context, opts Options) (*Result, error) {
	if err := validate(opts); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("bff: resolve root: %w", err)
	}
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		return nil, err
	}
	serviceDir, serviceRel, err := resolveServiceDir(root, opts)
	if err != nil {
		return nil, err
	}
	if err := ensureNotListed(w, opts.Name, serviceRel); err != nil {
		return nil, err
	}
	module := opts.Module
	if module == "" {
		module = defaultModule(w.Module, serviceRel)
	}
	if opts.DryRun {
		if err := ensureServiceDirAvailable(serviceDir); err != nil {
			return nil, err
		}
		next := dryRunNextSteps(serviceRel)
		return &Result{
			ServiceDir: serviceDir,
			ServiceRel: serviceRel,
			Module:     module,
			NextSteps:  next,
			Plan:       buildPlan(serviceDir, opts.Name, true, opts.NoGenerate, next),
			Updated:    true,
			DryRun:     true,
		}, nil
	}
	monoRes, err := mono.Generate(ctx, mono.Options{
		Name:          opts.Name,
		Module:        module,
		Kind:          manifest.KindHertz,
		Dir:           serviceDir,
		AssetsVersion: opts.AssetsVersion,
		NCGOVersion:   opts.NCGOVersion,
		NoGenerate:    opts.NoGenerate,
		Preset:        opts.Preset,
		TemplateDir:   opts.TemplateDir,
		Runner:        opts.Runner,
		Now:           opts.Now,
	})
	if err != nil {
		return nil, err
	}
	updated := mergeService(w, manifest.WorkspaceService{Name: opts.Name, Kind: manifest.KindHertz, Dir: serviceRel})
	if updated {
		if err := manifest.SaveWorkspace(root, w); err != nil {
			return nil, err
		}
		if err := rewriteDockerfileForLocalReplaces(root, serviceDir, serviceRel, w); err != nil {
			return nil, err
		}
		if err := shared.WriteWorkspaceCompose(root, w); err != nil {
			return nil, err
		}
	}
	return &Result{
		ServiceDir:  serviceDir,
		ServiceRel:  serviceRel,
		Module:      module,
		NextSteps:   monoRes.NextSteps,
		Plan:        buildPlan(serviceDir, opts.Name, updated, opts.NoGenerate, monoRes.NextSteps),
		Updated:     updated,
		RanGenerate: monoRes.RanGenerate,
	}, nil
}

func validate(opts Options) error {
	if opts.Root == "" {
		return errors.New("bff: Root is required")
	}
	if !nameRE.MatchString(opts.Name) {
		return fmt.Errorf("bff: name %q must match %s", opts.Name, nameRE)
	}
	if opts.Module != "" && !strings.Contains(opts.Module, "/") {
		return fmt.Errorf("bff: module %q is not a valid Go module path", opts.Module)
	}
	if opts.NCGOVersion == "" {
		return errors.New("bff: NCGOVersion is required")
	}
	return nil
}

func resolveServiceDir(root string, opts Options) (abs, rel string, err error) {
	rel = opts.Dir
	if rel == "" {
		rel = filepath.Join("services", opts.Name)
	}
	if filepath.IsAbs(rel) {
		return "", "", errors.New("bff: Dir must be relative to Root")
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("bff: Dir %q must stay under Root", opts.Dir)
	}
	return filepath.Join(root, rel), filepath.ToSlash(rel), nil
}

func defaultModule(workspaceModule, serviceRel string) string {
	return strings.TrimRight(workspaceModule, "/") + "/" + filepath.ToSlash(serviceRel)
}

func ensureNotListed(w *manifest.Workspace, name, dir string) error {
	for _, s := range w.Services {
		if s.Name == name {
			return fmt.Errorf("bff: service %q is already listed in ncgo.workspace", name)
		}
		if filepath.ToSlash(s.Dir) == dir {
			return fmt.Errorf("bff: service dir %q is already listed in ncgo.workspace", dir)
		}
	}
	return nil
}

// rewriteDockerfileForLocalReplaces rewrites serviceDir/Dockerfile when the
// service's go.mod has a local replace directive pointing at a sibling
// workspace service, so the Dockerfile COPYs both directories.
func rewriteDockerfileForLocalReplaces(root, serviceDir, serviceRel string, w *manifest.Workspace) error {
	replaces, err := shared.ParseLocalReplaces(serviceDir)
	if err != nil {
		return fmt.Errorf("bff: parse go.mod replace: %w", err)
	}
	siblings := shared.SiblingDirs(root, serviceRel, replaces, w.Services)
	if len(siblings) == 0 {
		return nil
	}
	return shared.RewriteServiceDockerfileForSiblings(serviceDir, manifest.KindHertz, serviceRel, siblings)
}

func mergeService(w *manifest.Workspace, service manifest.WorkspaceService) bool {
	for _, s := range w.Services {
		if s.Name == service.Name || filepath.ToSlash(s.Dir) == service.Dir {
			return false
		}
	}
	w.Services = append(w.Services, service)
	sort.Slice(w.Services, func(i, j int) bool { return w.Services[i].Dir < w.Services[j].Dir })
	return true
}

func ensureServiceDirAvailable(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bff: stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("bff: %s exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("bff: read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("bff: %s is not empty", dir)
	}
	return nil
}

func dryRunNextSteps(serviceRel string) []string {
	return []string{fmt.Sprintf("rerun without --plan to create %s", serviceRel)}
}

func buildPlan(serviceDir, name string, workspaceUpdated, noGenerate bool, next []string) []planpkg.Item {
	items := []planpkg.Item{
		{Kind: "directory", Action: "create", Path: serviceDir, Detail: "hertz service scaffold"},
	}
	workspaceAction := "already_present"
	if workspaceUpdated {
		workspaceAction = "add"
	}
	items = append(items, planpkg.Item{Kind: "workspace", Action: workspaceAction, Path: "ncgo.workspace", Detail: name})
	generatorAction, generatorDetail := "run", "hz"
	if noGenerate {
		generatorAction, generatorDetail = "skip", "--no-generate"
	}
	items = append(items, planpkg.Item{Kind: "generator", Action: generatorAction, Detail: generatorDetail})
	items = append(items, planpkg.Item{Kind: "file", Action: "write", Path: "compose.yaml", Detail: "workspace orchestration"})
	for _, step := range next {
		items = append(items, planpkg.Item{Kind: "next_step", Action: "run", Detail: step})
	}
	return items
}
