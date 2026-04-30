// Package rpc implements `ncgo add rpc <name>` for micro workspaces.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/mono"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Options configures Add.
type Options struct {
	Root          string      // micro workspace root containing ncgo.workspace
	Name          string      // RPC service name
	Module        string      // Go module path; defaults to <workspace.module>/<service dir>
	Dir           string      // service dir relative to Root; defaults to services/<name>
	AssetsVersion string      // recorded into the service manifest
	NCGOVersion   string      // recorded into the service manifest
	NoGenerate    bool        // skip kitex invocation
	Runner        exec.Runner // injected exec; nil means mono default
	Now           time.Time   // injected clock for tests
}

// Result describes what Add produced.
type Result struct {
	ServiceDir  string
	ServiceRel  string
	Module      string
	NextSteps   []string
	Updated     bool
	RanGenerate bool
}

// Add creates a Kitex RPC service under a micro workspace and records it in
// ncgo.workspace. It delegates the service scaffold itself to mono.Generate.
func Add(ctx context.Context, opts Options) (*Result, error) {
	if err := validate(opts); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("rpc: resolve root: %w", err)
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
	monoRes, err := mono.Generate(ctx, mono.Options{
		Name:          opts.Name,
		Module:        module,
		Kind:          manifest.KindKitex,
		Dir:           serviceDir,
		AssetsVersion: opts.AssetsVersion,
		NCGOVersion:   opts.NCGOVersion,
		NoGenerate:    opts.NoGenerate,
		Runner:        opts.Runner,
		Now:           opts.Now,
	})
	if err != nil {
		return nil, err
	}
	updated := mergeService(w, manifest.WorkspaceService{Name: opts.Name, Kind: manifest.KindKitex, Dir: serviceRel})
	if updated {
		if err := manifest.SaveWorkspace(root, w); err != nil {
			return nil, err
		}
	}
	return &Result{
		ServiceDir:  serviceDir,
		ServiceRel:  serviceRel,
		Module:      module,
		NextSteps:   monoRes.NextSteps,
		Updated:     updated,
		RanGenerate: monoRes.RanGenerate,
	}, nil
}

func validate(opts Options) error {
	if opts.Root == "" {
		return errors.New("rpc: Root is required")
	}
	if !nameRE.MatchString(opts.Name) {
		return fmt.Errorf("rpc: name %q must match %s", opts.Name, nameRE)
	}
	if opts.Module != "" && !strings.Contains(opts.Module, "/") {
		return fmt.Errorf("rpc: module %q is not a valid Go module path", opts.Module)
	}
	if opts.NCGOVersion == "" {
		return errors.New("rpc: NCGOVersion is required")
	}
	return nil
}

func resolveServiceDir(root string, opts Options) (abs, rel string, err error) {
	rel = opts.Dir
	if rel == "" {
		rel = filepath.Join("services", opts.Name)
	}
	if filepath.IsAbs(rel) {
		return "", "", errors.New("rpc: Dir must be relative to Root")
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("rpc: Dir %q must stay under Root", opts.Dir)
	}
	return filepath.Join(root, rel), filepath.ToSlash(rel), nil
}

func defaultModule(workspaceModule, serviceRel string) string {
	return strings.TrimRight(workspaceModule, "/") + "/" + filepath.ToSlash(serviceRel)
}

func ensureNotListed(w *manifest.Workspace, name, dir string) error {
	for _, s := range w.Services {
		if s.Name == name {
			return fmt.Errorf("rpc: service %q is already listed in ncgo.workspace", name)
		}
		if filepath.ToSlash(s.Dir) == dir {
			return fmt.Errorf("rpc: service dir %q is already listed in ncgo.workspace", dir)
		}
	}
	return nil
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
