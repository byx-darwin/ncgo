// Package manifest reads and writes the per-project `.ncgo/manifest.yaml`
// file. The manifest is the single source of truth that ncgo consults when
// running `add`, `doctor`, `ai sync`, or `extract` against an existing
// project. It is also the primary structured context surface that AI agents
// load to reason about the project.
package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Dir is the project-relative directory that holds ncgo metadata.
	Dir = ".ncgo"
	// FileName is the manifest file name inside Dir.
	FileName = "manifest.yaml"

	ModeMono  = "mono"
	ModeMicro = "micro"

	KindHertz = "hertz"
	KindKitex = "kitex"
)

// Manifest mirrors the schema documented in docs/prd.md §5.
type Manifest struct {
	Ncgo        Meta      `yaml:"ncgo"`
	Mode        string    `yaml:"mode"`
	Module      string    `yaml:"module"`
	Service     Service   `yaml:"service"`
	Infra       []string  `yaml:"infra,omitempty"`
	Domains     []string  `yaml:"domains,omitempty"`
	GeneratedAt time.Time `yaml:"generated_at"`
}

// Meta describes the ncgo build that produced the manifest.
type Meta struct {
	Version       string `yaml:"version"`
	AssetsVersion string `yaml:"assets_version"`
}

// Service describes the deployable unit the manifest belongs to.
type Service struct {
	Name         string `yaml:"name"`
	Kind         string `yaml:"kind"`
	WithDatabase bool   `yaml:"with_database"`
	IDL          string `yaml:"idl,omitempty"`
}

// Path returns the absolute path of the manifest file for the given project
// root: <root>/.ncgo/manifest.yaml.
func Path(root string) string {
	return filepath.Join(root, Dir, FileName)
}

// Load reads, parses, and validates the manifest under root.
func Load(root string) (*Manifest, error) {
	p := Path(root)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("manifest: read %s: %w", p, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("manifest: parse %s: %w", p, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest: %s: %w", p, err)
	}
	return &m, nil
}

// Save writes the manifest under root, creating <root>/.ncgo/ if needed.
// The write is atomic per file: tmp file + rename. GeneratedAt is stamped
// when zero so callers do not have to remember it.
func Save(root string, m *Manifest) error {
	if m == nil {
		return errors.New("manifest: Save called with nil manifest")
	}
	if m.GeneratedAt.IsZero() {
		m.GeneratedAt = time.Now().UTC()
	}
	if err := m.Validate(); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("manifest: mkdir %s: %w", dir, err)
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("manifest: marshal: %w", err)
	}
	final := Path(root)
	tmp, err := os.CreateTemp(dir, "manifest.*.yaml")
	if err != nil {
		return fmt.Errorf("manifest: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("manifest: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("manifest: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("manifest: rename %s -> %s: %w", tmpName, final, err)
	}
	return nil
}

// Validate enforces required fields and enum values.
func (m *Manifest) Validate() error {
	switch m.Mode {
	case ModeMono, ModeMicro:
	case "":
		return errors.New("mode is required (mono|micro)")
	default:
		return fmt.Errorf("mode %q is invalid (mono|micro)", m.Mode)
	}
	if m.Module == "" {
		return errors.New("module is required")
	}
	if m.Service.Name == "" {
		return errors.New("service.name is required")
	}
	switch m.Service.Kind {
	case KindHertz, KindKitex:
	case "":
		return errors.New("service.kind is required (hertz|kitex)")
	default:
		return fmt.Errorf("service.kind %q is invalid (hertz|kitex)", m.Service.Kind)
	}
	if m.Ncgo.Version == "" {
		return errors.New("ncgo.version is required")
	}
	return nil
}
