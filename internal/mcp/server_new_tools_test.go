package mcp

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

// TestServeToolCallUpgrade — call ncgo_upgrade, verify plan output + structured fields
func TestServeToolCallUpgrade(t *testing.T) {
	t.Skip("needs real workspace: upgrade.Run requires a valid ncgo project with manifest")
	root := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_upgrade", "arguments": map[string]any{"root": root}},
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
		t.Fatalf("upgrade returned error: %s", resultText(result))
	}
	// Verify structured fields
	if _, ok := result["upToDate"]; !ok {
		t.Fatalf("result missing upToDate field")
	}
	items, ok := result["items"].([]any)
	if !ok {
		t.Fatalf("result missing items field or wrong type")
	}
	if len(items) == 0 {
		t.Fatalf("items is empty, want at least metadata item")
	}
	content := resultText(result)
	if !strings.Contains(content, "upgrade plan for") {
		t.Fatalf("content missing 'upgrade plan for': %s", content)
	}
}

// TestServeToolCallExtractDomain — call ncgo_extract_domain with name+root, verify plan
func TestServeToolCallExtractDomain(t *testing.T) {
	t.Skip("needs real workspace: extract.PlanDomain requires a valid micro workspace")
	root := seedMCPProtoWorkspace(t)
	serviceRoot := filepath.Join(root, "services", "user-rpc")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_extract_domain", "arguments": map[string]any{"name": "user", "root": serviceRoot}},
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
		t.Fatalf("extract_domain returned error: %s", resultText(result))
	}
	// Verify structured fields
	name, ok := result["name"].(string)
	if !ok || name != "user" {
		t.Fatalf("result missing or wrong name field: %v", result["name"])
	}
	sources, ok := result["sources"].([]any)
	if !ok {
		t.Fatalf("result missing sources field or wrong type")
	}
	if len(sources) == 0 {
		t.Fatalf("sources is empty, want at least one source file")
	}
	nextSteps, ok := result["nextSteps"].([]any)
	if !ok {
		t.Fatalf("result missing nextSteps field or wrong type")
	}
	if len(nextSteps) == 0 {
		t.Fatalf("nextSteps is empty, want at least one step")
	}
	content := resultText(result)
	if !strings.Contains(content, "planned extraction for domain user") {
		t.Fatalf("content missing 'planned extraction for domain user': %s", content)
	}
}

// TestServeToolCallExportTemplates — call ncgo_export_templates, verify output
func TestServeToolCallExportTemplates(t *testing.T) {
	t.Skip("needs real workspace: template.Export requires a valid ncgo project")
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_export_templates", "arguments": map[string]any{"root": root}},
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
		t.Fatalf("export_templates returned error: %s", resultText(result))
	}
	// Verify structured fields
	outputDir, ok := result["outputDir"].(string)
	if !ok {
		t.Fatalf("result missing outputDir field or wrong type")
	}
	if outputDir == "" {
		t.Fatalf("outputDir is empty")
	}
	kind, ok := result["kind"].(string)
	if !ok {
		t.Fatalf("result missing kind field or wrong type")
	}
	if kind != manifest.KindHertz && kind != manifest.KindKitex {
		t.Fatalf("kind = %q, want hertz or kitex", kind)
	}
	templates, ok := result["templates"].([]any)
	if !ok {
		t.Fatalf("result missing templates field or wrong type")
	}
	if len(templates) == 0 {
		t.Fatalf("templates is empty, want at least one template")
	}
	content := resultText(result)
	if !strings.Contains(content, "exported ") {
		t.Fatalf("content missing 'exported ': %s", content)
	}
}

// TestServeToolCallAddRPC — call ncgo_add_rpc with name+root, verify result
func TestServeToolCallAddRPC(t *testing.T) {
	t.Skip("needs real workspace: rpc.Add requires a valid micro workspace with ncgo.workspace")
	root := seedMCPProtoWorkspace(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_rpc", "arguments": map[string]any{"name": "payment-rpc", "root": root}},
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
		t.Fatalf("add_rpc returned error: %s", resultText(result))
	}
	// Verify structured fields
	serviceDir, ok := result["serviceDir"].(string)
	if !ok {
		t.Fatalf("result missing serviceDir field or wrong type")
	}
	if serviceDir == "" {
		t.Fatalf("serviceDir is empty")
	}
	module, ok := result["module"].(string)
	if !ok {
		t.Fatalf("result missing module field or wrong type")
	}
	if module == "" {
		t.Fatalf("module is empty")
	}
	if _, ok := result["updated"]; !ok {
		t.Fatalf("result missing updated field")
	}
	if _, ok := result["dryRun"]; !ok {
		t.Fatalf("result missing dryRun field")
	}
	nextSteps, ok := result["nextSteps"].([]any)
	if !ok {
		t.Fatalf("result missing nextSteps field or wrong type")
	}
	if len(nextSteps) == 0 {
		t.Fatalf("nextSteps is empty, want at least one step")
	}
	content := resultText(result)
	if !strings.Contains(content, "wrote ") && !strings.Contains(content, "would write ") {
		t.Fatalf("content missing wrote/would write: %s", content)
	}
}

// TestServeToolCallAddBFF — call ncgo_add_bff with name+root, verify result
func TestServeToolCallAddBFF(t *testing.T) {
	t.Skip("needs real workspace: bff.Add requires a valid micro workspace with ncgo.workspace")
	root := seedMCPProtoWorkspace(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_bff", "arguments": map[string]any{"name": "user-api", "root": root}},
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
		t.Fatalf("add_bff returned error: %s", resultText(result))
	}
	// Verify structured fields
	serviceDir, ok := result["serviceDir"].(string)
	if !ok {
		t.Fatalf("result missing serviceDir field or wrong type")
	}
	if serviceDir == "" {
		t.Fatalf("serviceDir is empty")
	}
	module, ok := result["module"].(string)
	if !ok {
		t.Fatalf("result missing module field or wrong type")
	}
	if module == "" {
		t.Fatalf("module is empty")
	}
	if _, ok := result["updated"]; !ok {
		t.Fatalf("result missing updated field")
	}
	if _, ok := result["dryRun"]; !ok {
		t.Fatalf("result missing dryRun field")
	}
	nextSteps, ok := result["nextSteps"].([]any)
	if !ok {
		t.Fatalf("result missing nextSteps field or wrong type")
	}
	if len(nextSteps) == 0 {
		t.Fatalf("nextSteps is empty, want at least one step")
	}
	content := resultText(result)
	if !strings.Contains(content, "wrote ") && !strings.Contains(content, "would write ") {
		t.Fatalf("content missing wrote/would write: %s", content)
	}
}

// TestToolsListIncludesNewTools — verify tools/list includes all 5 new tool names
func TestToolsListIncludesNewTools(t *testing.T) {
	input := EncodeMessage(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	var out bytes.Buffer
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	listed := responses[0].Result.(map[string]any)["tools"].([]any)
	var names []string
	for _, item := range listed {
		tool := item.(map[string]any)
		names = append(names, tool["name"].(string))
	}
	for _, want := range []string{"ncgo_upgrade", "ncgo_extract_domain", "ncgo_export_templates", "ncgo_add_rpc", "ncgo_add_bff"} {
		if !contains(names, want) {
			t.Errorf("tools/list missing %q in %v", want, names)
		}
	}
}
