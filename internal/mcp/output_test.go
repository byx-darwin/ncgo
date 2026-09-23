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
	diagnostic := map[string]any{"severity": "warning", "code": "demo_warning", "message": "review me"}
	result := buildMCPResult("hello", false, map[string]any{
		"ok":          true,
		"count":       2,
		"diagnostics": []map[string]any{diagnostic},
		"nextSteps":   []string{"continue"},
	})
	if result["isError"].(bool) {
		t.Fatalf("isError = true, want false")
	}
	content := result["content"].([]map[string]string)
	if len(content) != 1 || content[0]["text"] != "hello" {
		t.Fatalf("content = %+v", content)
	}
	if result["ok"] != true || result["count"] != 2 {
		t.Fatalf("legacy fields = %+v", result)
	}
	envelope := result["structuredContent"].(map[string]any)
	if envelope["schemaVersion"] != mcpResultSchemaVersion || envelope["ok"] != true || envelope["error"] != nil {
		t.Fatalf("envelope header = %+v", envelope)
	}
	if envelope["data"].(map[string]any)["count"] != 2 {
		t.Fatalf("envelope data = %+v", envelope["data"])
	}
	if got := envelope["diagnostics"].([]map[string]any); len(got) != 1 || got[0]["code"] != "demo_warning" {
		t.Fatalf("envelope diagnostics = %+v", got)
	}
	if got := envelope["nextSteps"].([]string); len(got) != 1 || got[0] != "continue" {
		t.Fatalf("envelope nextSteps = %+v", got)
	}
}

func TestTextResultErrorEnvelope(t *testing.T) {
	result := textResult("boom", true)
	if !result["isError"].(bool) {
		t.Fatalf("isError = false, want true")
	}
	content := result["content"].([]map[string]string)
	if len(content) != 1 || content[0]["text"] != "boom" {
		t.Fatalf("content = %+v", content)
	}
	envelope := result["structuredContent"].(map[string]any)
	if envelope["ok"] != false {
		t.Fatalf("ok = true, want false: %+v", envelope)
	}
	toolErr := envelope["error"].(mcpError)
	if toolErr.Code != mcpInternalErrorCode || toolErr.Message != "boom" || toolErr.Retryable || toolErr.Remediation == "" {
		t.Fatalf("error = %+v", toolErr)
	}
	for _, key := range []string{"effects", "diagnostics", "nextSteps"} {
		if got := envelope[key].([]any); len(got) != 0 {
			t.Fatalf("%s = %+v, want empty", key, got)
		}
	}
}

func TestErrorHelpersExposeStableRecoverySemantics(t *testing.T) {
	invalid := invalidArgumentResult("bad input")
	invalidErr := invalid["structuredContent"].(map[string]any)["error"].(mcpError)
	if invalidErr.Code != mcpInvalidArgsCode || invalidErr.Retryable || !strings.Contains(invalidErr.Remediation, "inputSchema") {
		t.Fatalf("invalid argument error = %+v", invalidErr)
	}

	network := networkErrorResult("template pull", errors.New("remote unavailable"))
	networkErr := network["structuredContent"].(map[string]any)["error"].(mcpError)
	if networkErr.Code != mcpNetworkErrorCode || !networkErr.Retryable || !strings.Contains(networkErr.Remediation, "network") {
		t.Fatalf("network error = %+v", networkErr)
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
	toolErr := result["structuredContent"].(map[string]any)["error"].(mcpError)
	if toolErr.Code != "ncgo_demo_failed" || toolErr.Retryable || toolErr.Remediation == "" {
		t.Fatalf("structured tool error = %+v", toolErr)
	}
}
