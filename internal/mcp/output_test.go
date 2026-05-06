package mcp

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestResolveMCPOutput(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		input     string
		supported []string
		want      string
		wantErr   string
	}{
		{name: "default text", toolName: "doctor", input: "", supported: []string{mcpOutputText, mcpOutputJSON}, want: mcpOutputText},
		{name: "supported value", toolName: "doctor", input: mcpOutputJSON, supported: []string{mcpOutputText, mcpOutputJSON}, want: mcpOutputJSON},
		{name: "unsupported value", toolName: "doctor", input: "xml", supported: []string{mcpOutputText, mcpOutputJSON, mcpOutputSARIF}, wantErr: `doctor: unsupported output "xml"; want text, json, or sarif`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMCPOutput(tt.toolName, tt.input, tt.supported...)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveMCPOutput: %v", err)
			}
			if got != tt.want {
				t.Fatalf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatMCPOutput(t *testing.T) {
	text, err := formatMCPOutput(mcpOutputText, map[string]outputWriter{
		mcpOutputText: func(w io.Writer) error {
			_, err := io.WriteString(w, "hello")
			return err
		},
	})
	if err != nil {
		t.Fatalf("formatMCPOutput: %v", err)
	}
	if text != "hello" {
		t.Fatalf("text = %q, want hello", text)
	}

	if _, err := formatMCPOutput(mcpOutputJSON, map[string]outputWriter{}); err == nil || err.Error() != `mcp: missing renderer for output "json"` {
		t.Fatalf("missing renderer err = %v", err)
	}

	boom := errors.New("boom")
	if _, err := formatMCPOutput(mcpOutputText, map[string]outputWriter{mcpOutputText: func(io.Writer) error { return boom }}); !errors.Is(err, boom) {
		t.Fatalf("writer err = %v, want boom", err)
	}
}

func TestBuildMCPResult(t *testing.T) {
	result := buildMCPResult("hello", true, map[string]any{"ok": false, "count": 2})
	if !result["isError"].(bool) {
		t.Fatalf("isError = false, want true")
	}
	content := result["content"].([]map[string]string)
	if len(content) != 1 || content[0]["text"] != "hello" {
		t.Fatalf("content = %+v", content)
	}
	if result["ok"] != false || result["count"] != 2 {
		t.Fatalf("fields = %+v", result)
	}
}

func TestStructuredMCPToolBuildResult(t *testing.T) {
	tool := structuredMCPTool[int]{
		name:      "demo",
		supported: []string{mcpOutputText, mcpOutputJSON},
		format: func(v int, output string) (string, error) {
			return output + ":" + strings.Repeat("x", v), nil
		},
		fields:  func(v int) map[string]any { return map[string]any{"count": v} },
		isError: func(v int) bool { return v > 2 },
	}

	output, err := tool.resolveOutput(mcpOutputJSON)
	if err != nil {
		t.Fatalf("resolveOutput: %v", err)
	}
	result, err := tool.buildResult(3, output)
	if err != nil {
		t.Fatalf("buildResult: %v", err)
	}
	if !result["isError"].(bool) || result["count"] != 3 {
		t.Fatalf("result = %+v", result)
	}
	text := result["content"].([]map[string]string)[0]["text"]
	if text != "json:xxx" {
		t.Fatalf("text = %q, want json:xxx", text)
	}
}
