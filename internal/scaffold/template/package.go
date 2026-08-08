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
	Name                 string   `yaml:"name"`
	Kind                 string   `yaml:"kind"`
	Description          string   `yaml:"description"`
	Version              string   `yaml:"version"`
	SkipDefaultTemplates []string `yaml:"skip_default_templates"`
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
	SchemaDir   string   // absolute schema directory (may not exist)
	Schemas     []string // absolute .sql paths under SchemaDir
	LayoutFile  string   // absolute layout.yaml path (may not exist)
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
// template.yaml is optional (HasMeta=false); when present its kind must be
// compatible with the expected kind. The expected kind determines which
// subdirectories are loaded:
//
//   - (pkgKind="", expected=kitex|hertz) → <kind>-template/ + idl/
//   - (pkgKind=expected) → <kind>-template/ + idl/ (or workspace/ when kind=micro)
//   - (pkgKind=micro, expected=kitex|hertz) → <kind>-template/ + idl/<kind>/
//   - mismatch → error
//
// When a kind-specific IDL subdirectory (e.g. idl/kitex/) contains no .proto
// files, LoadPackage falls back to the flat idl/ directory.
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
	case errors.Is(err, fs.ErrNotExist):
		// optional metadata
	default:
		return nil, err
	}
	tplSubDir, idlSubDir, err := resolveTemplateSubDirs(pkg.Meta.Kind, kind)
	if err != nil {
		return nil, fmt.Errorf("template package %q: %w", dir, err)
	}
	pkg.TemplateDir = filepath.Join(abs, tplSubDir)
	entries, err := os.ReadDir(pkg.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("template package %q has no %s/ directory", dir, tplSubDir)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		pkg.Templates = append(pkg.Templates, filepath.Join(pkg.TemplateDir, e.Name()))
	}
	// For service template directories, require at least one .yaml template.
	// For workspace directories (micro expected kind), any file format is valid
	// (the overlay walks the directory directly and handles .tpl substitution).
	if len(pkg.Templates) == 0 && kind != "micro" {
		return nil, fmt.Errorf("template package %q has no .yaml templates in %s/", dir, tplSubDir)
	}
	pkg.IDLDir = filepath.Join(abs, idlSubDir)
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
	// IDL fallback: when a kind-specific subdirectory (e.g. idl/kitex/) has
	// no .proto files, try the flat idl/ directory for backward compatibility.
	if idlSubDir != "idl" && len(pkg.IDLs) == 0 {
		flatIDL := filepath.Join(abs, "idl")
		if fi, err := os.Stat(flatIDL); err == nil && fi.IsDir() {
			_ = filepath.Walk(flatIDL, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && strings.HasSuffix(path, ".proto") {
					pkg.IDLs = append(pkg.IDLs, path)
				}
				return nil
			})
			if len(pkg.IDLs) > 0 {
				pkg.IDLDir = flatIDL
			}
		}
	}
	pkg.SchemaDir = filepath.Join(abs, "schema")
	if fi, err := os.Stat(pkg.SchemaDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(pkg.SchemaDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".sql") {
				pkg.Schemas = append(pkg.Schemas, path)
			}
			return nil
		})
	}

	layoutPath := filepath.Join(abs, "layout.yaml")
	if _, err := os.Stat(layoutPath); err == nil {
		pkg.LayoutFile = layoutPath
	}
	return pkg, nil
}

// resolveTemplateSubDirs returns the template and IDL subdirectory names
// based on the package's declared kind and the consumer's expected kind.
func resolveTemplateSubDirs(pkgKind, expectedKind string) (tplDir, idlDir string, err error) {
	switch {
	case pkgKind == "" || pkgKind == expectedKind:
		// No metadata or matching kind.
		// For micro expected kind, use workspace/; otherwise <kind>-template/.
		if expectedKind == "micro" {
			return "workspace", "", nil
		}
		return expectedKind + "-template", "idl", nil
	case pkgKind == "micro" && expectedKind == "kitex":
		return "kitex-template", "idl/kitex", nil
	case pkgKind == "micro" && expectedKind == "hertz":
		return "hertz-template", "idl/hertz", nil
	default:
		return "", "", fmt.Errorf("template package kind %q does not match expected kind %q", pkgKind, expectedKind)
	}
}
