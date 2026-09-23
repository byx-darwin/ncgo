package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRegisteredToolsPublishCompleteSafetyAndOutputContracts(t *testing.T) {
	tools := New("test", "test").tools()
	if len(tools) != len(mcpToolSafety) {
		t.Fatalf("registered tools = %d, safety entries = %d", len(tools), len(mcpToolSafety))
	}
	seen := make(map[string]bool, len(tools))
	for _, registered := range tools {
		if seen[registered.Name] {
			t.Fatalf("duplicate registered tool %q", registered.Name)
		}
		seen[registered.Name] = true
		safety, ok := mcpToolSafety[registered.Name]
		if !ok {
			t.Fatalf("tool %q has no safety classification", registered.Name)
		}
		if registered.Annotations.ReadOnlyHint != safety.ReadOnly ||
			registered.Annotations.DestructiveHint != safety.Destructive ||
			registered.Annotations.IdempotentHint != safety.Idempotent ||
			registered.Annotations.OpenWorldHint != safety.Network {
			t.Fatalf("tool %q annotations = %+v, safety = %+v", registered.Name, registered.Annotations, safety)
		}
		behavior, ok := registered.Meta["io.github.byx-darwin.ncgo/toolBehavior"].(map[string]bool)
		if !ok {
			t.Fatalf("tool %q missing namespaced behavior metadata: %+v", registered.Name, registered.Meta)
		}
		if behavior["networkAccess"] != safety.Network || behavior["externalProcess"] != safety.ExternalProcess || behavior["supportsDryRun"] != safety.SupportsDryRun {
			t.Fatalf("tool %q behavior = %+v, safety = %+v", registered.Name, behavior, safety)
		}
		if !safety.ReadOnly && !safety.SupportsDryRun && !strings.Contains(registered.Description, "No dryRun") {
			t.Fatalf("mutating tool %q neither supports dryRun nor documents the exception", registered.Name)
		}
		assertResultOutputSchema(t, registered.Name, registered.OutputSchema)
	}
	for name := range mcpToolSafety {
		if !seen[name] {
			t.Fatalf("safety entry %q has no registered tool", name)
		}
	}
}

func TestServeToolsListPublishesAnnotationsMetadataAndOutputSchema(t *testing.T) {
	input := EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	var out bytes.Buffer
	if err := New("test", "test").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	listed := responses[0].Result.(map[string]any)["tools"].([]any)
	if len(listed) != len(mcpToolSafety) {
		t.Fatalf("listed tools = %d, want %d", len(listed), len(mcpToolSafety))
	}
	for _, item := range listed {
		wireTool := item.(map[string]any)
		name := wireTool["name"].(string)
		annotations := wireTool["annotations"].(map[string]any)
		for _, key := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			if _, ok := annotations[key].(bool); !ok {
				t.Fatalf("tool %q annotation %q missing or not boolean: %+v", name, key, annotations)
			}
		}
		meta := wireTool["_meta"].(map[string]any)
		behavior := meta["io.github.byx-darwin.ncgo/toolBehavior"].(map[string]any)
		for _, key := range []string{"networkAccess", "externalProcess", "supportsDryRun"} {
			if _, ok := behavior[key].(bool); !ok {
				t.Fatalf("tool %q behavior %q missing or not boolean: %+v", name, key, behavior)
			}
		}
		outputSchema := wireTool["outputSchema"].(map[string]any)
		if outputSchema["type"] != "object" {
			t.Fatalf("tool %q output schema = %+v", name, outputSchema)
		}
	}
}

func TestServeResultEnvelopeSuccessWarningAndError(t *testing.T) {
	allowAnyRootForTest(t)
	warningRoot := mcpFixtureRoot(t, "..", "protolint", "testdata", "phase2warnings")
	input := append(EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_version", "arguments": map[string]any{}},
	}), EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": warningRoot, "files": []string{"invalid.proto"}, "rules": []string{"PIO111", "PIO112", "PIO113"}}},
	})...)
	input = append(input, EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{}},
	})...)

	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	if len(responses) != 3 {
		t.Fatalf("responses = %d, want 3", len(responses))
	}

	success := responses[0].Result.(map[string]any)
	assertWireEnvelope(t, success, true, 0)
	if !strings.Contains(resultText(success), "test-version") {
		t.Fatalf("success content changed: %q", resultText(success))
	}

	warning := responses[1].Result.(map[string]any)
	assertWireEnvelope(t, warning, true, 5)
	if warning["isError"].(bool) {
		t.Fatalf("warning-only response isError=true: %+v", warning)
	}

	failure := responses[2].Result.(map[string]any)
	assertWireEnvelope(t, failure, false, 0)
	if !failure["isError"].(bool) {
		t.Fatalf("error response isError=false: %+v", failure)
	}
	envelope := failure["structuredContent"].(map[string]any)
	toolErr := envelope["error"].(map[string]any)
	if toolErr["code"] != mcpInvalidArgsCode || toolErr["message"] == "" || toolErr["retryable"] != false || toolErr["remediation"] == "" {
		t.Fatalf("error contract = %+v", toolErr)
	}
}

func assertResultOutputSchema(t *testing.T, toolName string, schema map[string]any) {
	t.Helper()
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("tool %q output schema header = %+v", toolName, schema)
	}
	required := schema["required"].([]string)
	for _, key := range []string{"schemaVersion", "ok", "data", "effects", "diagnostics", "error", "nextSteps"} {
		found := false
		for _, item := range required {
			if item == key {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("tool %q output schema missing required %q: %+v", toolName, key, required)
		}
	}
}

func assertWireEnvelope(t *testing.T, result map[string]any, wantOK bool, wantDiagnostics int) {
	t.Helper()
	envelope, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("missing structuredContent: %+v", result)
	}
	if envelope["schemaVersion"] != mcpResultSchemaVersion || envelope["ok"] != wantOK {
		t.Fatalf("envelope header = %+v", envelope)
	}
	if _, ok := envelope["data"].(map[string]any); !ok {
		t.Fatalf("envelope data is not object: %+v", envelope["data"])
	}
	if got := len(envelope["effects"].([]any)); got != 0 {
		t.Fatalf("effects len = %d, want 0", got)
	}
	if got := len(envelope["diagnostics"].([]any)); got != wantDiagnostics {
		t.Fatalf("diagnostics len = %d, want %d", got, wantDiagnostics)
	}
	if _, ok := envelope["nextSteps"].([]any); !ok {
		t.Fatalf("nextSteps is not array: %+v", envelope["nextSteps"])
	}
	if wantOK && envelope["error"] != nil {
		t.Fatalf("success/warning error = %+v, want null", envelope["error"])
	}
}
