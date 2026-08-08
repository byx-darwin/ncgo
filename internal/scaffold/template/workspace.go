package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// OverlayWorkspaceTemplates walks pkg.TemplateDir recursively and overlays
// files onto targetDir. Files with a .tpl suffix are rendered with Go
// text/template using data; the .tpl suffix is stripped from the target path.
// Files without .tpl are copied verbatim. Existing files at the target path
// are overwritten.
func OverlayWorkspaceTemplates(targetDir string, pkg *Package, data RenderData) error {
	root := pkg.TemplateDir
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return fmt.Errorf("workspace overlay: %s is not a directory", root)
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(targetDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(rel, ".tpl") {
			target = strings.TrimSuffix(target, ".tpl")
			body, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("workspace overlay: read %s: %w", path, err)
			}
			tmpl, err := template.New(rel).Parse(string(body))
			if err != nil {
				return fmt.Errorf("workspace overlay: parse %s: %w", path, err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("workspace overlay: render %s: %w", path, err)
			}
			return os.WriteFile(target, buf.Bytes(), 0o644)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("workspace overlay: read %s: %w", path, err)
		}
		return os.WriteFile(target, body, 0o644)
	})
}
