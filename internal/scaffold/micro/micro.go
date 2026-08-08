// Package micro implements the root workspace half of `ncgo new --mode micro`.
// Service generators (`ncgo add rpc` / `ncgo add bff`) are layered on top later.
package micro

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/shared"
	scaffoldtemplate "github.com/byx-darwin/ncgo/internal/scaffold/template"
)

type Options struct {
	Name          string
	Module        string
	Dir           string
	AssetsVersion string
	NCGOVersion   string
	Now           time.Time
	TemplateDir   string // external template package dir; overlays workspace/ templates onto built-in skeleton
}

type Result struct {
	Dir       string
	NextSteps []string
}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func Generate(opts Options) (*Result, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("micro: resolve dir: %w", err)
	}
	if err := ensureEmptyDir(dir); err != nil {
		return nil, err
	}
	if err := writeWorkspace(dir, opts); err != nil {
		return nil, err
	}
	if err := writeReadme(dir, opts); err != nil {
		return nil, err
	}
	if err := shared.WriteWorkspaceCompose(dir, &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: opts.NCGOVersion, AssetsVersion: opts.AssetsVersion},
		Mode:        manifest.ModeMicro,
		Name:        opts.Name,
		Module:      opts.Module,
		Services:    []manifest.WorkspaceService{},
		GeneratedAt: opts.Now,
	}); err != nil {
		return nil, err
	}
	if err := shared.WriteRepositoryHooks(dir); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "services", ".gitkeep"), nil, 0o644); err != nil {
		return nil, fmt.Errorf("micro: write services/.gitkeep: %w", err)
	}
	// Overlay workspace templates if TemplateDir is set.
	if opts.TemplateDir != "" {
		pkg, err := scaffoldtemplate.LoadPackage(opts.TemplateDir, "micro")
		if err != nil {
			return nil, err
		}
		data := scaffoldtemplate.RenderData{
			Module:      opts.Module,
			ServiceName: opts.Name,
		}
		if err := scaffoldtemplate.OverlayWorkspaceTemplates(dir, pkg, data); err != nil {
			return nil, fmt.Errorf("micro: workspace overlay: %w", err)
		}
	}
	return &Result{Dir: dir, NextSteps: nextSteps(opts)}, nil
}

func (o Options) validate() error {
	if !nameRE.MatchString(o.Name) {
		return fmt.Errorf("micro: name %q must match %s", o.Name, nameRE)
	}
	if o.Module == "" || !strings.Contains(o.Module, "/") {
		return fmt.Errorf("micro: module %q is not a valid Go module path", o.Module)
	}
	if o.Dir == "" {
		return errors.New("micro: Dir is required")
	}
	if o.NCGOVersion == "" {
		return errors.New("micro: NCGOVersion is required")
	}
	return nil
}

func ensureEmptyDir(dir string) error {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.MkdirAll(filepath.Join(dir, "services"), 0o755)
	case err != nil:
		return fmt.Errorf("micro: stat %s: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("micro: %s exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("micro: read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("micro: %s is not empty", dir)
	}
	return os.MkdirAll(filepath.Join(dir, "services"), 0o755)
}

func writeWorkspace(dir string, opts Options) error {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return manifest.SaveWorkspace(dir, &manifest.Workspace{
		Ncgo: manifest.Meta{Version: opts.NCGOVersion, AssetsVersion: opts.AssetsVersion},
		Mode: manifest.ModeMicro, Name: opts.Name, Module: opts.Module,
		Services: []manifest.WorkspaceService{}, GeneratedAt: now,
	})
}

func writeReadme(dir string, opts Options) error {
	body := fmt.Sprintf("# %s\n\n"+
		"ncgo micro workspace for module `%s`.\n\n"+
		"- Workspace metadata: `ncgo.workspace`\n"+
		"- Workspace orchestration: `compose.yaml`\n"+
		"- Local hooks config: `.pre-commit-config.yaml`\n"+
		"- Services live under `services/` and keep their own `.ncgo/manifest.yaml`.\n\n"+
		"Use `ncgo add rpc <name>` to add Kitex RPC services and `ncgo add bff <name>` to add Hertz BFF services.\n",
		opts.Name, opts.Module)
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(body), 0o644)
}

func nextSteps(opts Options) []string {
	rel, _ := filepath.Rel(mustCwd(), opts.Dir)
	if rel == "" {
		rel = filepath.Base(opts.Dir)
	}
	return []string{
		fmt.Sprintf("cd %s", rel),
		"add a Kitex RPC service with `ncgo add rpc <name>`",
		"add a Hertz BFF service with `ncgo add bff <name>`",
	}
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
