package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/doctor"
)

func TestServeToolCallCheck(t *testing.T) {
	old := runCheckReport
	runCheckReport = func(string) (*doctor.Report, error) {
		return &doctor.Report{
			Root:  "/repo/demo",
			Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{
				CheckCount: 2, PassedCount: 1, FailedCount: 1, ErrorCount: 1,
			},
			Checks: []doctor.Check{
				{ID: "check.anchor", OK: true, Severity: doctor.SeverityError, Message: "all usecase files have paired method anchors"},
				{ID: "check.manifest.consistency", OK: false, Severity: doctor.SeverityError, Message: "manifest domains mismatch"},
			},
		}, nil
	}
	defer func() { runCheckReport = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_check", "arguments": map[string]any{"root": "/repo/demo"}},
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
		t.Fatalf("check failure unexpectedly succeeded: %+v", result)
	}
	if result["root"].(string) != "/repo/demo" || result["scope"].(string) != string(doctor.ScopeService) {
		t.Fatalf("header = %+v", result)
	}
	if result["ok"].(bool) {
		t.Fatalf("ok = true, want false")
	}
	if !strings.Contains(resultText(result), "manifest domains mismatch") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallCheckJSON(t *testing.T) {
	old := runCheckReport
	runCheckReport = func(string) (*doctor.Report, error) {
		return &doctor.Report{
			Root: "/repo/demo", Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{CheckCount: 1, PassedCount: 1},
			Checks:  []doctor.Check{{ID: "check.anchor", OK: true, Severity: doctor.SeverityError, Message: "ok"}},
		}, nil
	}
	defer func() { runCheckReport = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_check", "arguments": map[string]any{"root": "/repo/demo", "output": "json"}},
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
	if result["isError"].(bool) {
		t.Fatalf("check unexpectedly failed: %+v", result)
	}
	if !strings.Contains(resultText(result), `"id": "check.anchor"`) {
		t.Fatalf("json content missing check id: %s", resultText(result))
	}
}

func TestServeToolCallCheckInvalidOutput(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_check", "arguments": map[string]any{"root": "/repo/demo", "output": "sarif"}},
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
		t.Fatalf("expected error for unsupported output=sarif, got: %+v", result)
	}
}
