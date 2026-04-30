package mcp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestServeInitializeAndToolsList(t *testing.T) {
	input := append(EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize"}),
		EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})...)
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want 2", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("initialize error: %+v", responses[0].Error)
	}
	listed := responses[1].Result.(map[string]any)["tools"].([]any)
	var names []string
	var addInfra map[string]any
	for _, item := range listed {
		tool := item.(map[string]any)
		name := tool["name"].(string)
		names = append(names, name)
		if name == "ncgo_add_infra" {
			addInfra = tool
		}
	}
	for _, want := range []string{"ncgo_version", "ncgo_doctor", "ncgo_ai_sync", "ncgo_add_infra", "ncgo_add_method"} {
		if !contains(names, want) {
			t.Errorf("tools/list missing %s in %v", want, names)
		}
	}
	props := addInfra["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := props["wire"]; !ok {
		t.Fatalf("ncgo_add_infra schema missing wire property: %+v", props)
	}
	if _, ok := props["dryRun"]; !ok {
		t.Fatalf("ncgo_add_infra schema missing dryRun property: %+v", props)
	}
}

func TestServeToolCallVersion(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_version", "arguments": map[string]any{}},
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
		t.Fatalf("version call returned isError")
	}
	content := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(content, "test-version") || !strings.Contains(content, "test-assets") {
		t.Fatalf("version content = %q", content)
	}
}

func TestServeToolCallAddInfra(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_infra", "arguments": map[string]any{"root": root, "kind": "otel"}},
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
		t.Fatalf("add infra returned error: %s", resultText(result))
	}
	if result["dryRun"].(bool) {
		t.Fatalf("dryRun = true, want false")
	}
	if !result["updated"].(bool) {
		t.Fatalf("updated = false, want true")
	}
	if got := len(result["writtenPaths"].([]any)); got != 1 {
		t.Fatalf("writtenPaths len = %d, want 1", got)
	}
	if got := len(result["plan"].([]any)); got == 0 {
		t.Fatalf("plan is empty")
	}
	content := resultText(result)
	for _, want := range []string{"wrote ", "loongsuite-go-agent", "otel go build ./..."} {
		if !strings.Contains(content, want) {
			t.Fatalf("add infra content missing %q:\n%s", want, content)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "observability", "otel.go")); err != nil {
		t.Fatalf("otel file was not written: %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != "observability_otel" {
		t.Fatalf("manifest.Infra = %v, want [observability_otel]", m.Infra)
	}
}

func TestServeToolCallAddInfraDryRun(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_infra", "arguments": map[string]any{"root": root, "kind": "otel", "dryRun": true}},
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
		t.Fatalf("add infra dry-run returned error: %s", resultText(result))
	}
	if !result["dryRun"].(bool) {
		t.Fatalf("dryRun = false, want true")
	}
	if !result["updated"].(bool) {
		t.Fatalf("updated = false, want true")
	}
	if !mcpPlanContains(result["plan"].([]any), "file", "create") {
		t.Fatalf("plan missing file create: %+v", result["plan"])
	}
	if !mcpPlanContains(result["plan"].([]any), "manifest", "add") {
		t.Fatalf("plan missing manifest add: %+v", result["plan"])
	}
	content := resultText(result)
	for _, want := range []string{"would write ", "dry-run: manifest would be updated", "dry-run: no files were written"} {
		if !strings.Contains(content, want) {
			t.Fatalf("add infra dry-run content missing %q:\n%s", want, content)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "observability", "otel.go")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote otel file: stat err = %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 0 {
		t.Fatalf("dry-run updated manifest infra = %v, want empty", m.Infra)
	}
}

func TestServeUnknownMethod(t *testing.T) {
	input := EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "missing"})
	var out bytes.Buffer
	if err := New("v", "a").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	if responses[0].Error == nil || responses[0].Error.Code != -32601 {
		t.Fatalf("error = %+v, want -32601", responses[0].Error)
	}
}

func seedMCPProject(t *testing.T, kind string) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/demo",
		Service: manifest.Service{
			Name: "demo", Kind: kind, IDL: "idl/app/demo.proto",
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return root
}

func resultText(result map[string]any) string {
	return result["content"].([]any)[0].(map[string]any)["text"].(string)
}

func mcpPlanContains(plan []any, kind, action string) bool {
	for _, raw := range plan {
		item := raw.(map[string]any)
		if item["kind"] == kind && item["action"] == action {
			return true
		}
	}
	return false
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
