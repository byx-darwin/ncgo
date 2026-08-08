package template

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PackageMeta is the template.yaml metadata describing a template package.
type PackageMeta struct {
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// Package is a template package loaded from disk.
type Package struct {
	Dir         string // absolute package root
	Meta        PackageMeta
	HasMeta     bool     // false when template.yaml is absent
	TemplateDir string   // absolute <kind>-template directory
	Templates   []string // absolute .yaml paths under TemplateDir
	IDLDir      string   // absolute idl directory (may not exist)
	IDLs        []string // absolute .proto paths under IDLDir
}

// ReadPackageMeta reads <dir>/template.yaml. A missing file returns an error
// satisfying fs.ErrNotExist; callers listing registries skip such dirs.
func ReadPackageMeta(dir string) (PackageMeta, error) {
	path := filepath.Join(dir, "template.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return PackageMeta{}, err
	}
	var m PackageMeta
	if err := yaml.Unmarshal(b, &m); err != nil {
		return PackageMeta{}, fmt.Errorf("template package: parse %s: %w", path, err)
	}
	return m, nil
}

// LoadPackage loads a template package rooted at dir for the given kind.
// template.yaml is optional (HasMeta=false); when present its kind must
// match. The package must contain at least one .yaml in <kind>-template/.
func LoadPackage(dir, kind string) (*Package, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve template dir: %w", err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("template package %q does not exist", dir)
	}
	pkg := &Package{Dir: abs}
	meta, err := ReadPackageMeta(abs)
	switch {
	case err == nil:
		pkg.Meta, pkg.HasMeta = meta, true
		if meta.Kind != "" && meta.Kind != kind {
			return nil, fmt.Errorf("template package %q has kind %q, want %q", dir, meta.Kind, kind)
		}
	case errors.Is(err, fs.ErrNotExist):
		// optional metadata
	default:
		return nil, err
	}
	pkg.TemplateDir = filepath.Join(abs, kind+"-template")
	entries, err := os.ReadDir(pkg.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("template package %q has no %s-template/ directory", dir, kind)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		pkg.Templates = append(pkg.Templates, filepath.Join(pkg.TemplateDir, e.Name()))
	}
	if len(pkg.Templates) == 0 {
		return nil, fmt.Errorf("template package %q has no .yaml templates in %s-template/", dir, kind)
	}
	pkg.IDLDir = filepath.Join(abs, "idl")
	if fi, err := os.Stat(pkg.IDLDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(pkg.IDLDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".proto") {
				pkg.IDLs = append(pkg.IDLs, path)
			}
			return nil
		})
	}
	return pkg, nil
}
