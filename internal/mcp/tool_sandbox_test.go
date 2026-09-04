package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestSandboxRootRejectsEscapePaths verifies every MCP tool that accepts a
// root/dir parameter rejects paths outside the workspace (cwd), whether via
// an absolute path or a "../" relative escape. This locks in the fix for
// Issue #97 (sandbox boundary validation was missing on 8 tool entrypoints).
func TestSandboxRootRejectsEscapePaths(t *testing.T) {
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
	}
	escapes := map[string]string{
		"absolute": "/etc",
		"relative": "../../../etc",
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
			})
		}
	}
}
