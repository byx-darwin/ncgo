package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const WorkspaceFileName = "ncgo.workspace"

// Workspace is the root-level metadata file for `--mode micro` repositories.
// Individual BFF/RPC services still keep their own .ncgo/manifest.yaml files.
type Workspace struct {
	Ncgo        Meta               `yaml:"ncgo"`
	Mode        string             `yaml:"mode"`
	Name        string             `yaml:"name"`
	Module      string             `yaml:"module"`
	Services    []WorkspaceService `yaml:"services"`
	GeneratedAt time.Time          `yaml:"generated_at"`
}

// WorkspaceService records a service directory managed by a micro workspace.
type WorkspaceService struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"`
	Dir  string `yaml:"dir"`
}

// WorkspacePath returns the absolute path to the root workspace metadata file.
func WorkspacePath(root string) string {
	return filepath.Join(root, WorkspaceFileName)
}

// LoadWorkspace reads and validates <root>/ncgo.workspace.
func LoadWorkspace(root string) (*Workspace, error) {
	p := WorkspacePath(root)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("workspace: read %s: %w", p, err)
	}
	var w Workspace
	if err := yaml.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("workspace: parse %s: %w", p, err)
	}
	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("workspace: %s: %w", p, err)
	}
	return &w, nil
}

// SaveWorkspace writes <root>/ncgo.workspace atomically.
func SaveWorkspace(root string, w *Workspace) error {
	if w == nil {
		return errors.New("workspace: SaveWorkspace called with nil workspace")
	}
	if w.GeneratedAt.IsZero() {
		w.GeneratedAt = time.Now().UTC()
	}
	if w.Services == nil {
		w.Services = []WorkspaceService{}
	}
	if err := w.Validate(); err != nil {
		return fmt.Errorf("workspace: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("workspace: mkdir %s: %w", root, err)
	}
	b, err := yaml.Marshal(w)
	if err != nil {
		return fmt.Errorf("workspace: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(root, "ncgo.workspace.*")
	if err != nil {
		return fmt.Errorf("workspace: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("workspace: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("workspace: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, WorkspacePath(root)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("workspace: rename %s: %w", tmpName, err)
	}
	return nil
}

// Validate enforces the root workspace metadata schema.
func (w *Workspace) Validate() error {
	if w.Mode != ModeMicro {
		if w.Mode == "" {
			return errors.New("mode is required (micro)")
		}
		return fmt.Errorf("mode %q is invalid (micro)", w.Mode)
	}
	if w.Name == "" {
		return errors.New("name is required")
	}
	if w.Module == "" {
		return errors.New("module is required")
	}
	if w.Ncgo.Version == "" {
		return errors.New("ncgo.version is required")
	}
	for i, s := range w.Services {
		if s.Name == "" || s.Kind == "" || s.Dir == "" {
			return fmt.Errorf("services[%d] requires name, kind and dir", i)
		}
		if s.Kind != KindHertz && s.Kind != KindKitex {
			return fmt.Errorf("services[%d].kind %q is invalid (hertz|kitex)", i, s.Kind)
		}
	}
	return nil
}
