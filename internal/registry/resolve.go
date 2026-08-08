package registry

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveTemplateDir converts --template/--template-dir to an absolute
// directory path. Returns ("", nil) when neither flag is set (default
// embedded templates). Uses the default registry client for --template
// resolution.
func ResolveTemplateDir(templateName, templateDir string) (string, error) {
	if templateName != "" && templateDir != "" {
		return "", fmt.Errorf("--template and --template-dir are mutually exclusive")
	}
	if templateName != "" {
		client := NewClient(ResolveURL(""), nil)
		return resolveFromClient(client, templateName)
	}
	if templateDir != "" {
		abs, err := filepath.Abs(templateDir)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return "", nil
}

// resolveFromClient resolves a template name using the provided client.
func resolveFromClient(client *Client, templateName string) (string, error) {
	dir := client.LocalPath(templateName)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("template %q not in cache (%s); run: ncgo template pull %s", templateName, dir, templateName)
	}
	return dir, nil
}
