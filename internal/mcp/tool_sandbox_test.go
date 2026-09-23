package mcp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSandboxRootRejectsEscapePaths verifies every MCP tool that accepts a
// root/dir parameter rejects paths outside the workspace (cwd), including
// lexical and symlink escapes. This extends the Issue #97 coverage to every
// current MCP filesystem entrypoint.
func TestSandboxRootRejectsEscapePaths(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(workspace, "dangling")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "ncgo_new", args: map[string]any{"name": "demo", "module": "github.com/x/demo", "noGenerate": true}},
		{name: "ncgo_i18n_report", args: map[string]any{}},
		{name: "ncgo_i18n_check", args: map[string]any{}},
		{name: "ncgo_protolint", args: map[string]any{"files": []string{"app/demo.proto"}}},
		{name: "ncgo_doctor", args: map[string]any{}},
		{name: "ncgo_add_rule_center", args: map[string]any{"addr": "127.0.0.1:8888"}},
		{name: "ncgo_ai_sync", args: map[string]any{}},
		{name: "ncgo_ai_init_claude", args: map[string]any{}},
		{name: "ncgo_ai_context", args: map[string]any{}},
		{name: "ncgo_add_infra", args: map[string]any{"kind": "redis"}},
		{name: "ncgo_add_method", args: map[string]any{"spec": "device.Get"}},
		{name: "ncgo_add_rpc_method", args: map[string]any{"service": "demo", "rpc": "Ping"}},
		{name: "ncgo_check", args: map[string]any{}},
		{name: "ncgo_add_domain", args: map[string]any{"name": "device"}},
		{name: "ncgo_import", args: map[string]any{}},
		{name: "ncgo_upgrade", args: map[string]any{}},
		{name: "ncgo_extract_domain", args: map[string]any{"name": "device"}},
		{name: "ncgo_export_templates", args: map[string]any{}},
		{name: "ncgo_add_rpc", args: map[string]any{"name": "demo-rpc"}},
		{name: "ncgo_add_bff", args: map[string]any{"name": "demo-api"}},
	}
	escapes := map[string]string{
		"absolute": filepath.Join(workspace, "inside"),
		"relative": "../outside",
		"symlink":  "escape",
		"dangling": "dangling/child",
	}

	for _, tc := range cases {
		for escapeName, escapePath := range escapes {
			t.Run(tc.name+"/"+escapeName, func(t *testing.T) {
				args := map[string]any{}
				for k, v := range tc.args {
					args[k] = v
				}
				rootKey := "root"
				if tc.name == "ncgo_new" {
					rootKey = "dir"
				}
				args[rootKey] = escapePath

				input := EncodeMessage(map[string]any{
					"jsonrpc": "2.0", "id": 1, "method": "tools/call",
					"params": map[string]any{"name": tc.name, "arguments": args},
				})
				var out bytes.Buffer
				if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
					t.Fatalf("Serve: %v", err)
				}
				responses, err := DecodeResponses(out.Bytes())
				if err != nil {
					t.Fatalf("DecodeResponses: %v", err)
				}
				result := responses[0].Result.(map[string]any)
				if !result["isError"].(bool) {
					t.Fatalf("%s with %s escape %q unexpectedly succeeded: %+v", tc.name, escapeName, escapePath, result)
				}
				text := resultText(result)
				if !strings.Contains(text, "outside the workspace") {
					t.Fatalf("%s with %s escape %q: content = %q, want it to contain %q", tc.name, escapeName, escapePath, text, "outside the workspace")
				}
				errObject, ok := result["error"].(map[string]any)
				if !ok || errObject["code"] != sandboxErrorCode {
					t.Fatalf("%s with %s escape %q: error = %#v, want code %q", tc.name, escapeName, escapePath, result["error"], sandboxErrorCode)
				}
			})
		}
	}
}

func TestDefaultResolvePathHandlesSymlinksAndMissingTargets(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	inside := filepath.Join(workspace, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, filepath.Join(workspace, "safe-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(workspace, "dangling")); err != nil {
		t.Fatal(err)
	}
	resolvedWorkspace, err := resolveExistingComponents(workspace)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "workspace", target: "."},
		{name: "missing target", target: "new/child"},
		{name: "safe symlink", target: "safe-link/child"},
		{name: "absolute", target: filepath.Join(workspace, "inside"), wantErr: true},
		{name: "traversal", target: "inside/../../outside", wantErr: true},
		{name: "symlink escape", target: "escape", wantErr: true},
		{name: "missing below symlink escape", target: "escape/new", wantErr: true},
		{name: "dangling symlink", target: "dangling/new", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := defaultResolvePath(tt.target)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
					t.Fatalf("defaultResolvePath(%q) = %q, %v; want workspace error", tt.target, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("defaultResolvePath(%q): %v", tt.target, err)
			}
			if !pathWithin(resolvedWorkspace, got) {
				t.Fatalf("defaultResolvePath(%q) = %q, outside %q", tt.target, got, resolvedWorkspace)
			}
		})
	}
}

func TestMCPFilesystemInputsRejectEscapesBeforeMutation(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	t.Chdir(workspace)

	tests := []struct {
		name string
		args map[string]any
	}{
		{name: "ncgo_new", args: map[string]any{"name": "demo", "module": "github.com/x/demo", "dir": "../outside/new", "noGenerate": true}},
		{name: "ncgo_new", args: map[string]any{"name": "demo", "module": "github.com/x/demo", "mode": "micro", "dir": "new", "templateDir": "../outside/templates"}},
		{name: "ncgo_add_rpc", args: map[string]any{"name": "demo-rpc", "root": ".", "dir": "../outside/rpc"}},
		{name: "ncgo_add_rpc", args: map[string]any{"name": "demo-rpc", "root": ".", "templateDir": "../outside/templates"}},
		{name: "ncgo_add_bff", args: map[string]any{"name": "demo-api", "root": ".", "dir": "../outside/bff"}},
		{name: "ncgo_add_bff", args: map[string]any{"name": "demo-api", "root": ".", "templateDir": "../outside/templates"}},
		{name: "ncgo_extract_domain", args: map[string]any{"name": "device", "root": ".", "to": "../outside/service"}},
		{name: "ncgo_protolint", args: map[string]any{"root": ".", "files": []string{"../outside/api.proto"}}},
		{name: "ncgo_protolint", args: map[string]any{"root": ".", "ignoreFiles": []string{"../outside/api.proto"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := invokeSandboxTool(t, tt.name, tt.args)
			if isError, _ := result["isError"].(bool); !isError {
				t.Fatalf("result unexpectedly succeeded: %+v", result)
			}
			errObject, ok := result["error"].(map[string]any)
			if !ok || errObject["code"] != sandboxErrorCode {
				t.Fatalf("error = %#v, want code %q", result["error"], sandboxErrorCode)
			}
			entries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("outside directory was partially modified: %v", entries)
			}
			if _, err := os.Stat(filepath.Join(workspace, "new")); !os.IsNotExist(err) {
				t.Fatalf("workspace target was partially created before sandbox rejection: %v", err)
			}
		})
	}
}

func TestSandboxChildPathMatrix(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, filepath.Join(root, "safe-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := resolveExistingComponents(root)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "missing target", target: "new/child"},
		{name: "safe symlink", target: "safe-link/child"},
		{name: "absolute", target: filepath.Join(root, "inside"), wantErr: true},
		{name: "traversal", target: "inside/../child", wantErr: true},
		{name: "symlink escape", target: "escape/child", wantErr: true},
		{name: "dangling symlink", target: "dangling/child", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sandboxChild(root, tt.target)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "outside the workspace") {
					t.Fatalf("sandboxChild(%q) = %q, %v; want workspace error", tt.target, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("sandboxChild(%q): %v", tt.target, err)
			}
			if !pathWithin(resolvedRoot, got) {
				t.Fatalf("sandboxChild(%q) = %q, outside %q", tt.target, got, resolvedRoot)
			}
		})
	}
}

func TestSandboxRootRejectsEscapingDescendantSymlinks(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, ".claude")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	result := invokeSandboxTool(t, "ncgo_ai_init_claude", map[string]any{"root": "."})
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("result unexpectedly succeeded: %+v", result)
	}
	errObject, ok := result["error"].(map[string]any)
	if !ok || errObject["code"] != sandboxErrorCode {
		t.Fatalf("error = %#v, want code %q", result["error"], sandboxErrorCode)
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("outside directory was modified through descendant symlink: %v", entries)
	}
}

func TestSandboxRootAllowsInternalDescendantSymlinks(t *testing.T) {
	workspace := t.TempDir()
	inside := filepath.Join(workspace, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, filepath.Join(workspace, "safe-link")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)
	if _, err := sandboxRoot("."); err != nil {
		t.Fatalf("sandboxRoot rejected in-workspace descendant symlink: %v", err)
	}
}

func TestSandboxRootAllowsDescendantSymlinkElsewhereInWorkspace(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "project")
	shared := filepath.Join(workspace, "shared")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(project, "shared-link")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)
	if _, err := sandboxRoot("project"); err != nil {
		t.Fatalf("sandboxRoot rejected descendant symlink inside MCP workspace: %v", err)
	}
}

func TestSandboxRootRejectsNestedSymlinkChainEscape(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	project := filepath.Join(workspace, "project")
	shared := filepath.Join(workspace, "shared")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(project, ".claude")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(shared, "agents")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)
	if _, err := sandboxRoot("project"); err == nil || !strings.Contains(err.Error(), "outside the workspace") {
		t.Fatalf("sandboxRoot nested symlink chain error = %v, want workspace rejection", err)
	}
}

func TestSandboxRootRejectsWorkspaceServiceDirEscape(t *testing.T) {
	workspace := t.TempDir()
	workspaceBody := "ncgo:\n  version: test\nmode: micro\nname: demo\nmodule: github.com/x/demo\nservices:\n  - name: escaped\n    kind: hertz\n    dir: ../outside\n"
	if err := os.WriteFile(filepath.Join(workspace, "ncgo.workspace"), []byte(workspaceBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)

	result := invokeSandboxTool(t, "ncgo_check", map[string]any{"root": "."})
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("result unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), "services[0].dir") || !strings.Contains(resultText(result), "outside the workspace") {
		t.Fatalf("unexpected error: %q", resultText(result))
	}
}

func TestSandboxRootLeavesMalformedWorkspaceForOwningTool(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "ncgo.workspace"), []byte("services: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workspace)
	if _, err := sandboxRoot("."); err != nil {
		t.Fatalf("sandboxRoot classified non-path workspace error: %v", err)
	}
}

func invokeSandboxTool(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	return responses[0].Result.(map[string]any)
}
