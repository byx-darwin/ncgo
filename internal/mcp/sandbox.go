package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePath resolves and validates a target path against the workspace boundary.
// Default implementation uses the current working directory as the workspace.
// The variable enables test overrides without weakening production behavior.
var resolvePath = defaultResolvePath

func defaultResolvePath(target string) (string, error) {
	if target == "" {
		target = "."
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("mcp: cannot resolve path %q: %w", target, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("mcp: cannot get working directory: %w", err)
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("mcp: cannot resolve working directory: %w", err)
	}
	cleanAbs := filepath.Clean(abs)
	cleanCwd := filepath.Clean(absCwd)
	if cleanAbs == cleanCwd {
		return abs, nil
	}
	if !strings.HasPrefix(cleanAbs, cleanCwd+string(filepath.Separator)) {
		return "", fmt.Errorf("mcp: path %q is outside the workspace %q", abs, absCwd)
	}
	return abs, nil
}

// sandboxRoot validates that target resolves within the current workspace
// (cwd), returning an error if it is an absolute path or a relative path
// that escapes it (e.g. via "../"). Every MCP tool handler that accepts a
// root/dir parameter MUST call this before using that value — see
// TestSandboxRootRejectsEscapePaths in tool_sandbox_test.go for the list of
// tools currently covered; a new tool handler that accepts root/dir needs
// a call here and a new case in that table.
func sandboxRoot(target string) (string, error) {
	if target == "" {
		target = "."
	}
	return resolvePath(target)
}
