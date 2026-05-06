package protolint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

type workspaceTarget struct {
	Root   string
	File   string
	Path   string
	Prefix string
}

func discoverServiceFiles(root string) ([]string, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return nil, err
	}
	idl := strings.TrimSpace(m.Service.IDL)
	if idl == "" {
		return nil, fmt.Errorf("protolint: %s has empty manifest.service.idl", manifest.Path(root))
	}
	return []string{filepath.ToSlash(filepath.Clean(idl))}, nil
}

func discoverWorkspaceTargets(root string) ([]workspaceTarget, error) {
	w, err := manifest.LoadWorkspace(root)
	if err != nil {
		return nil, err
	}
	if len(w.Services) == 0 {
		return nil, fmt.Errorf("protolint: %s declares no services", manifest.WorkspacePath(root))
	}
	targets := make([]workspaceTarget, 0, len(w.Services))
	for _, svc := range w.Services {
		serviceRoot := filepath.Join(root, filepath.FromSlash(svc.Dir))
		m, err := manifest.Load(serviceRoot)
		if err != nil {
			return nil, fmt.Errorf("protolint: load workspace service %s: %w", svc.Name, err)
		}
		idl := strings.TrimSpace(m.Service.IDL)
		if idl == "" {
			return nil, fmt.Errorf("protolint: workspace service %s has empty manifest.service.idl", svc.Name)
		}
		targets = append(targets, workspaceTarget{
			Root:   serviceRoot,
			File:   filepath.ToSlash(filepath.Clean(idl)),
			Path:   filepath.ToSlash(filepath.Join(svc.Dir, idl)),
			Prefix: filepath.ToSlash(filepath.Clean(svc.Dir)),
		})
	}
	return targets, nil
}

func prefixWorkspacePath(prefix, path string) string {
	if prefix == "" {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return filepath.ToSlash(filepath.Join(prefix, path))
}

func hasManifestMetadata(root string) bool {
	_, err := os.Stat(manifest.Path(root))
	return err == nil
}

func hasWorkspaceMetadata(root string) bool {
	_, err := os.Stat(manifest.WorkspacePath(root))
	return err == nil
}

func missingFilesDiscoveryError() error {
	return fmt.Errorf("protolint: at least one --file is required unless --root points to an ncgo service or micro workspace")
}
