// Package upgrade implements metadata-only project upgrades.
package upgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

type Options struct {
	Root          string
	NCGOVersion   string
	AssetsVersion string
	DryRun        bool
	Plan          bool
}

type Result struct {
	Root           string
	Mode           string
	Path           string
	Changed        bool
	DryRun         bool
	Plan           bool
	OldVersion     string
	NewVersion     string
	OldAssets      string
	NewAssets      string
	ServiceCount   int
	ServiceUpdates []ServiceUpdate
}

type ServiceUpdate struct {
	Name       string
	Path       string
	Changed    bool
	OldVersion string
	NewVersion string
	OldAssets  string
	NewAssets  string
}

func Run(opts Options) (*Result, error) {
	if opts.Root == "" {
		return nil, errors.New("upgrade: Root is required")
	}
	if opts.NCGOVersion == "" {
		return nil, errors.New("upgrade: NCGOVersion is required")
	}
	if opts.AssetsVersion == "" {
		return nil, errors.New("upgrade: AssetsVersion is required")
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("upgrade: resolve root: %w", err)
	}
	if fileExists(manifest.WorkspacePath(root)) {
		return runWorkspace(root, opts)
	}
	if fileExists(manifest.Path(root)) {
		return runManifest(root, opts)
	}
	return nil, fmt.Errorf("upgrade: %s contains neither %s nor %s", root, manifest.WorkspaceFileName, filepath.Join(manifest.Dir, manifest.FileName))
}

func runManifest(root string, opts Options) (*Result, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	res := &Result{
		Root: root, Mode: manifest.ModeMono, Path: manifest.Path(root), DryRun: opts.DryRun, Plan: opts.Plan,
		OldVersion: m.Ncgo.Version, NewVersion: opts.NCGOVersion,
		OldAssets: m.Ncgo.AssetsVersion, NewAssets: opts.AssetsVersion,
	}
	res.Changed = applyMeta(&m.Ncgo, opts)
	if res.Changed && !readOnly(opts) {
		if err := manifest.Save(root, m); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func runWorkspace(root string, opts Options) (*Result, error) {
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		return nil, err
	}
	res := &Result{
		Root: root, Mode: manifest.ModeMicro, Path: manifest.WorkspacePath(root), DryRun: opts.DryRun, Plan: opts.Plan,
		OldVersion: w.Ncgo.Version, NewVersion: opts.NCGOVersion,
		OldAssets: w.Ncgo.AssetsVersion, NewAssets: opts.AssetsVersion,
		ServiceCount: len(w.Services),
	}
	res.Changed = applyMeta(&w.Ncgo, opts)
	for _, svc := range w.Services {
		serviceRoot := filepath.Join(root, filepath.FromSlash(svc.Dir))
		update, err := inspectService(serviceRoot, svc.Name, opts)
		if err != nil {
			return nil, err
		}
		res.ServiceUpdates = append(res.ServiceUpdates, update)
		res.Changed = res.Changed || update.Changed
	}
	if res.Changed && !readOnly(opts) {
		if err := manifest.SaveWorkspace(root, w); err != nil {
			return nil, err
		}
		for _, svc := range w.Services {
			serviceRoot := filepath.Join(root, filepath.FromSlash(svc.Dir))
			m, err := manifest.Load(serviceRoot)
			if err != nil {
				return nil, err
			}
			if applyMeta(&m.Ncgo, opts) {
				if err := manifest.Save(serviceRoot, m); err != nil {
					return nil, err
				}
			}
		}
	}
	return res, nil
}

func inspectService(root, name string, opts Options) (ServiceUpdate, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return ServiceUpdate{}, err
	}
	return ServiceUpdate{
		Name: name, Path: manifest.Path(root), OldVersion: m.Ncgo.Version, OldAssets: m.Ncgo.AssetsVersion,
		NewVersion: opts.NCGOVersion, NewAssets: opts.AssetsVersion,
		Changed: m.Ncgo.Version != opts.NCGOVersion || m.Ncgo.AssetsVersion != opts.AssetsVersion,
	}, nil
}

func readOnly(opts Options) bool {
	return opts.DryRun || opts.Plan
}

func applyMeta(meta *manifest.Meta, opts Options) bool {
	changed := meta.Version != opts.NCGOVersion || meta.AssetsVersion != opts.AssetsVersion
	meta.Version = opts.NCGOVersion
	meta.AssetsVersion = opts.AssetsVersion
	return changed
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
