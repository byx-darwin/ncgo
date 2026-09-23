package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"gopkg.in/yaml.v3"
)

const sandboxErrorCode = "mcp_path_outside_workspace"

// resolvePath resolves and validates a target path against the workspace boundary.
// Default implementation uses the current working directory as the workspace.
// The variable enables test overrides without weakening production behavior.
var resolvePath = defaultResolvePath

func defaultResolvePath(target string) (string, error) {
	if target == "" {
		target = "."
	}
	if err := validateRelativePath(target); err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("mcp: cannot get working directory: %w", err)
	}
	workspace, err := resolveExistingComponents(cwd)
	if err != nil {
		return "", fmt.Errorf("mcp: cannot resolve workspace %q: %w", cwd, err)
	}
	candidate, err := resolveExistingComponents(filepath.Join(cwd, target))
	if err != nil {
		return "", sandboxPathError(target, workspace, err.Error())
	}
	if !pathWithin(workspace, candidate) {
		return "", sandboxPathError(target, workspace, "resolved path is outside the workspace")
	}
	return candidate, nil
}

func validateRelativePath(target string) error {
	if filepath.IsAbs(target) {
		return sandboxPathError(target, "", "absolute paths are outside the workspace boundary; use a relative path")
	}
	for _, part := range strings.FieldsFunc(target, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return sandboxPathError(target, "", "relative traversal is outside the workspace boundary")
		}
	}
	return nil
}

// resolveExistingComponents canonicalizes every existing path component. For a
// target that does not exist yet, it resolves the nearest existing parent and
// appends only the missing suffix. A dangling symlink is an existing component,
// so EvalSymlinks fails instead of allowing validation to climb past it.
func resolveExistingComponents(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	current := filepath.Clean(abs)
	missing := make([]string, 0, 4)
	for {
		_, lstatErr := os.Lstat(current)
		switch {
		case lstatErr == nil:
			resolved, evalErr := filepath.EvalSymlinks(current)
			if evalErr != nil {
				return "", fmt.Errorf("cannot resolve symlink component %q: %w", current, evalErr)
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		case errors.Is(lstatErr, os.ErrNotExist):
			parent := filepath.Dir(current)
			if parent == current {
				return "", lstatErr
			}
			missing = append(missing, filepath.Base(current))
			current = parent
		default:
			return "", lstatErr
		}
	}
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sandboxPathError(target, workspace, reason string) error {
	if workspace == "" {
		return fmt.Errorf("mcp: path %q is outside the workspace: %s", target, reason)
	}
	return fmt.Errorf("mcp: path %q is outside the workspace %q: %s", target, workspace, reason)
}

// sandboxRoot validates a caller-provided root/dir against the MCP workspace.
// User paths must be relative. Existing components are resolved through
// symlinks, and missing targets are checked through their nearest existing
// parent.
func sandboxRoot(target string) (string, error) {
	if target == "" {
		target = "."
	}
	resolved, err := resolvePath(target)
	if err != nil {
		return "", err
	}
	boundary := resolved
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if resolvedCWD, resolveErr := resolveExistingComponents(cwd); resolveErr == nil && pathWithin(resolvedCWD, resolved) {
			boundary = resolvedCWD
		}
	}
	if err := validateDescendantSymlinks(resolved, boundary); err != nil {
		return "", err
	}
	if err := validateWorkspaceServiceDirs(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// validateDescendantSymlinks prevents fixed tool-owned paths below a valid
// root (for example .ncgo/, .claude/, internal/, and template/) from escaping
// the boundary. WalkDir does not follow directory symlinks, so each link is
// evaluated explicitly and safe in-workspace links remain supported.
func validateDescendantSymlinks(root, boundary string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return sandboxPathError(root, root, err.Error())
	}
	if !info.IsDir() {
		return nil
	}
	visited := map[string]bool{}
	var walkResolvedDir func(string) error
	walkResolvedDir = func(dir string) error {
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return sandboxPathError(dir, boundary, "cannot resolve directory: "+err.Error())
		}
		resolvedDir, err = filepath.Abs(resolvedDir)
		if err != nil {
			return sandboxPathError(dir, boundary, err.Error())
		}
		if visited[resolvedDir] {
			return nil
		}
		visited[resolvedDir] = true
		return filepath.WalkDir(resolvedDir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return sandboxPathError(path, boundary, walkErr.Error())
			}
			if entry.Type()&os.ModeSymlink == 0 {
				return nil
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return sandboxPathError(path, boundary, "cannot resolve symlink: "+err.Error())
			}
			resolved, err = filepath.Abs(resolved)
			if err != nil {
				return sandboxPathError(path, boundary, err.Error())
			}
			if !pathWithin(boundary, resolved) {
				return sandboxPathError(path, boundary, "descendant symlink resolves outside the workspace")
			}
			targetInfo, err := os.Stat(resolved)
			if err != nil {
				return sandboxPathError(path, boundary, err.Error())
			}
			if targetInfo.IsDir() {
				return walkResolvedDir(resolved)
			}
			return nil
		})
	}
	return walkResolvedDir(root)
}

// validateWorkspaceServiceDirs treats paths stored in ncgo.workspace as
// indirect filesystem inputs. Tools must not trust metadata to escape the MCP
// root lexically or through a symlink.
func validateWorkspaceServiceDirs(root string) error {
	workspacePath := manifest.WorkspacePath(root)
	if _, err := os.Stat(workspacePath); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return sandboxPathError(workspacePath, root, err.Error())
	}
	body, err := os.ReadFile(workspacePath)
	if err != nil {
		// The owning tool retains responsibility for reporting ordinary read
		// and schema errors. The sandbox only classifies path violations.
		return nil
	}
	var workspacePaths struct {
		Services []struct {
			Dir string `yaml:"dir"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(body, &workspacePaths); err != nil {
		return nil
	}
	for i, service := range workspacePaths.Services {
		if service.Dir == "" {
			continue
		}
		if _, err := sandboxChild(root, service.Dir); err != nil {
			return fmt.Errorf("mcp: ncgo.workspace services[%d].dir: %w", i, err)
		}
	}
	return nil
}

// sandboxChild applies the same policy to a filesystem input interpreted
// relative to an already validated project root.
func sandboxChild(root, target string) (string, error) {
	if target == "" {
		return "", nil
	}
	if err := validateRelativePath(target); err != nil {
		return "", err
	}
	resolvedRoot, err := resolveExistingComponents(root)
	if err != nil {
		return "", fmt.Errorf("mcp: cannot resolve project root %q: %w", root, err)
	}
	candidate, err := resolveExistingComponents(filepath.Join(root, target))
	if err != nil {
		return "", sandboxPathError(target, resolvedRoot, err.Error())
	}
	if !pathWithin(resolvedRoot, candidate) {
		return "", sandboxPathError(target, resolvedRoot, "resolved path is outside the project root")
	}
	return candidate, nil
}

func sandboxErrorResult(err error) map[string]any {
	result := textResult(err.Error(), true)
	result["error"] = map[string]any{
		"code":    sandboxErrorCode,
		"message": err.Error(),
	}
	return result
}
