package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/doctor"
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
	var doctorTool map[string]any
	var addInfra map[string]any
	var aiInitTool map[string]any
	var aiSyncTool map[string]any
	var i18nReportTool map[string]any
	var i18nCheckTool map[string]any
	var protolintTool map[string]any
	for _, item := range listed {
		tool := item.(map[string]any)
		name := tool["name"].(string)
		names = append(names, name)
		if name == "ncgo_doctor" {
			doctorTool = tool
		}
		if name == "ncgo_add_infra" {
			addInfra = tool
		}
		if name == "ncgo_ai_init_claude" {
			aiInitTool = tool
		}
		if name == "ncgo_ai_sync" {
			aiSyncTool = tool
		}
		if name == "ncgo_i18n_report" {
			i18nReportTool = tool
		}
		if name == "ncgo_i18n_check" {
			i18nCheckTool = tool
		}
		if name == "ncgo_protolint" {
			protolintTool = tool
		}
	}
	for _, want := range []string{"ncgo_version", "ncgo_doctor", "ncgo_ai_init_claude", "ncgo_ai_sync", "ncgo_i18n_report", "ncgo_i18n_check", "ncgo_protolint", "ncgo_add_infra", "ncgo_add_method"} {
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
	if _, ok := props["output"]; !ok {
		t.Fatalf("ncgo_add_infra schema missing output property: %+v", props)
	}
	aiInitProps := aiInitTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := aiInitProps["output"]; !ok {
		t.Fatalf("ncgo_ai_init_claude schema missing output property: %+v", aiInitProps)
	}
	if _, ok := aiInitProps["preset"]; !ok {
		t.Fatalf("ncgo_ai_init_claude schema missing preset property: %+v", aiInitProps)
	}
	aiSyncProps := aiSyncTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := aiSyncProps["output"]; !ok {
		t.Fatalf("ncgo_ai_sync schema missing output property: %+v", aiSyncProps)
	}
	i18nReportProps := i18nReportTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := i18nReportProps["output"]; !ok {
		t.Fatalf("ncgo_i18n_report schema missing output property: %+v", i18nReportProps)
	}
	i18nCheckProps := i18nCheckTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := i18nCheckProps["output"]; !ok {
		t.Fatalf("ncgo_i18n_check schema missing output property: %+v", i18nCheckProps)
	}
	protolintProps := protolintTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := protolintProps["ignoreRules"]; !ok {
		t.Fatalf("ncgo_protolint schema missing ignoreRules property: %+v", protolintProps)
	}
	if _, ok := protolintProps["ignoreFiles"]; !ok {
		t.Fatalf("ncgo_protolint schema missing ignoreFiles property: %+v", protolintProps)
	}
	if _, ok := protolintProps["output"]; !ok {
		t.Fatalf("ncgo_protolint schema missing output property: %+v", protolintProps)
	}
	doctorProps := doctorTool["inputSchema"].(map[string]any)["properties"].(map[string]any)
	if _, ok := doctorProps["output"]; !ok {
		t.Fatalf("ncgo_doctor schema missing output property: %+v", doctorProps)
	}
}

func TestServeToolCallVersion(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_version", "arguments": map[string]any{}},
	})
	var out bytes.Buffer
	if err := New("test-version", "test-assets", "test-build", "2026-05-06T12:00:00Z").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
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
	if !strings.Contains(content, "test-version") || !strings.Contains(content, "test-assets") || !strings.Contains(content, "test-build") || !strings.Contains(content, "2026-05-06T12:00:00Z") {
		t.Fatalf("version content = %q", content)
	}
}

func TestServeToolCallDoctor(t *testing.T) {
	old := runDoctorReport
	runDoctorReport = func(context.Context, doctor.Options) *doctor.Report {
		return &doctor.Report{
			Root:  "/repo/demo",
			Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{
				CheckCount: 2, PassedCount: 1, FailedCount: 1, ErrorCount: 1,
			},
			Checks: []doctor.Check{{ID: "tool.hz", OK: true, Severity: doctor.SeverityError, Message: "hz v0.9.7"}, {ID: "manifest.load", OK: false, Severity: doctor.SeverityError, Message: "manifest missing", File: "/repo/demo/.ncgo/manifest.yaml"}},
		}
	}
	defer func() { runDoctorReport = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_doctor", "arguments": map[string]any{"root": "/repo/demo"}},
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
		t.Fatalf("doctor failure unexpectedly succeeded: %+v", result)
	}
	if result["root"].(string) != "/repo/demo" || result["scope"].(string) != string(doctor.ScopeService) {
		t.Fatalf("header = %+v", result)
	}
	if !strings.Contains(resultText(result), "manifest missing") {
		t.Fatalf("content = %q", resultText(result))
	}
	if result["ok"].(bool) {
		t.Fatalf("ok = true, want false")
	}
	summary := result["summary"].(map[string]any)
	if summary["errorCount"].(float64) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if got := len(result["checks"].([]any)); got != 2 {
		t.Fatalf("checks len = %d, want 2", got)
	}
}

func TestServeToolCallDoctorSARIF(t *testing.T) {
	old := runDoctorReport
	runDoctorReport = func(context.Context, doctor.Options) *doctor.Report {
		return &doctor.Report{
			Root:  "/repo/demo",
			Scope: doctor.ScopeService,
			Summary: doctor.ReportSummary{
				CheckCount: 2, PassedCount: 1, FailedCount: 1, ErrorCount: 1,
			},
			Checks: []doctor.Check{{ID: "tool.hz", OK: true, Severity: doctor.SeverityError, Message: "hz v0.9.7"}, {ID: "manifest.load", OK: false, Severity: doctor.SeverityError, Message: "manifest missing", File: "/repo/demo/.ncgo/manifest.yaml"}},
		}
	}
	defer func() { runDoctorReport = old }()

	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_doctor", "arguments": map[string]any{"root": "/repo/demo", "output": "sarif"}},
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
		t.Fatalf("doctor sarif failure unexpectedly succeeded: %+v", result)
	}
	content := resultText(result)
	if !strings.Contains(content, `"version": "2.1.0"`) || !strings.Contains(content, `"name": "ncgo doctor"`) {
		t.Fatalf("content = %q", content)
	}
	if result["root"].(string) != "/repo/demo" || result["scope"].(string) != string(doctor.ScopeService) {
		t.Fatalf("header = %+v", result)
	}
}

func TestServeToolCallDoctorInvalidOutput(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_doctor", "arguments": map[string]any{"root": "/repo/demo", "output": "xml"}},
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
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), `doctor: unsupported output "xml"; want text, json, or sarif`) {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallAIInitClaude(t *testing.T) {
	root := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_init_claude", "arguments": map[string]any{"root": root}},
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
		t.Fatalf("ai init claude returned error: %s", resultText(result))
	}
	if len(result["written"].([]any)) == 0 {
		t.Fatalf("written = %+v, want starter files", result["written"])
	}
	nextSteps := result["nextSteps"].([]any)
	if len(nextSteps) != 1 || nextSteps[0].(string) != "run ncgo ai sync --root "+root+" --lang en" {
		t.Fatalf("nextSteps = %+v, want ai sync hint", nextSteps)
	}
	text := resultText(result)
	if !strings.Contains(text, "wrote .claude/README.md") || !strings.Contains(text, "next: run ncgo ai sync --root "+root+" --lang en") {
		t.Fatalf("text = %q", text)
	}
}

func TestServeToolCallAIInitClaudeJSON(t *testing.T) {
	root := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_init_claude", "arguments": map[string]any{"root": root, "preset": "team", "output": "json"}},
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
		t.Fatalf("ai init claude json returned error: %s", resultText(result))
	}
	textJSON := resultJSONObject(t, result)
	if len(textJSON["written"].([]any)) == 0 {
		t.Fatalf("text payload json = %+v, want written files", textJSON)
	}
	nextSteps := textJSON["nextSteps"].([]any)
	if len(nextSteps) != 1 {
		t.Fatalf("text payload json = %+v, want nextSteps", textJSON)
	}
}

func TestServeToolCallAIInitClaudeInvalidOutput(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_init_claude", "arguments": map[string]any{"root": "/repo/demo", "output": "xml"}},
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
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), `ai init claude: unsupported output "xml"; want text or json`) {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallAISyncIncludesStructuredFields(t *testing.T) {
	root := seedMCPProtoWorkspace(t)
	serviceRoot := filepath.Join(root, "services", "user-rpc")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_sync", "arguments": map[string]any{"root": serviceRoot, "dryRun": true}},
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
		t.Fatalf("ai sync returned error: %s", resultText(result))
	}
	if result["scope"] != "service" || result["sourceRef"] != ".ncgo/manifest.yaml" {
		t.Fatalf("structured metadata = %+v, want scope=service sourceRef=.ncgo/manifest.yaml", result)
	}
	workspace := result["workspace"].(map[string]any)
	if workspace["role"] != "member" || workspace["name"] != "commerce" || workspace["root"] != "../.." || workspace["serviceDir"] != "services/user-rpc" {
		t.Fatalf("workspace = %+v, want member/commerce/../../services/user-rpc", workspace)
	}
	if len(result["written"].([]any)) != 0 {
		t.Fatalf("dry-run should not write files: %+v", result["written"])
	}
	if len(result["skipped"].([]any)) != 4 {
		t.Fatalf("dry-run skipped = %+v, want 4 targets", result["skipped"])
	}
	text := resultText(result)
	if !strings.Contains(text, "info: detected parent micro workspace `../..` for this service root") {
		t.Fatalf("text = %q", text)
	}
	if !strings.Contains(text, "skipped AGENTS.md (dry-run)") {
		t.Fatalf("text = %q", text)
	}
}

func TestServeToolCallAISyncJSON(t *testing.T) {
	root := seedMCPProtoWorkspace(t)
	serviceRoot := filepath.Join(root, "services", "user-rpc")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_sync", "arguments": map[string]any{"root": serviceRoot, "dryRun": true, "output": "json"}},
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
		t.Fatalf("ai sync json returned error: %s", resultText(result))
	}
	textJSON := resultJSONObject(t, result)
	if textJSON["scope"] != "service" || textJSON["sourceRef"] != ".ncgo/manifest.yaml" {
		t.Fatalf("text payload json = %+v, want lower-case structured keys", textJSON)
	}
	workspace := textJSON["workspace"].(map[string]any)
	if workspace["role"] != "member" || workspace["name"] != "commerce" {
		t.Fatalf("json workspace = %+v", workspace)
	}
}

func TestServeToolCallAISyncInvalidOutput(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_ai_sync", "arguments": map[string]any{"root": "/repo/demo", "output": "xml"}},
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
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), `ai sync: unsupported output "xml"; want text or json`) {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallAddInfra(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_infra", "arguments": map[string]any{"root": root, "kind": "redis"}},
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
	if got := len(result["writtenPaths"].([]any)); got != 3 {
		t.Fatalf("writtenPaths len = %d, want 3", got)
	}
	if got := len(result["plan"].([]any)); got == 0 {
		t.Fatalf("plan is empty")
	}
	content := resultText(result)
	for _, want := range []string{"wrote ", "go-middleware", "go mod tidy"} {
		if !strings.Contains(content, want) {
			t.Fatalf("add infra content missing %q:\n%s", want, content)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis.go")); err != nil {
		t.Fatalf("redis file was not written: %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != "redis" {
		t.Fatalf("manifest.Infra = %v, want [redis]", m.Infra)
	}
}

func TestServeToolCallI18NReport(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_report", "arguments": map[string]any{"root": root}},
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
		t.Fatalf("i18n report returned error: %s", resultText(result))
	}
	if result["root"].(string) != root {
		t.Fatalf("root = %q, want %q", result["root"], root)
	}
	if result["sourceLocale"].(string) != "zh-CN" {
		t.Fatalf("sourceLocale = %q", result["sourceLocale"])
	}
	schema := result["schema"].(map[string]any)
	if schema["id"] != mcpI18NReportSchemaID || schema["path"] != mcpI18NReportSchemaPath {
		t.Fatalf("schema = %+v", schema)
	}
	report := result["report"].(map[string]any)
	if report["source_locale"].(string) != "zh-CN" {
		t.Fatalf("report.source_locale = %+v", report["source_locale"])
	}
	if got := len(result["nextSteps"].([]any)); got == 0 {
		t.Fatalf("nextSteps empty")
	}
	content := resultText(result)
	if !strings.Contains(content, "i18n report loaded for zh-CN") {
		t.Fatalf("report content = %q", content)
	}
}

func TestServeToolCallI18NReportJSON(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_report", "arguments": map[string]any{"root": root, "output": "json"}},
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
		t.Fatalf("i18n report json returned error: %s", resultText(result))
	}
	content := resultJSONObject(t, result)
	if content["root"] != root || content["sourceLocale"] != "zh-CN" {
		t.Fatalf("json content = %+v", content)
	}
	if result["root"].(string) != root {
		t.Fatalf("root = %q, want %q", result["root"], root)
	}
	if result["sourceLocale"].(string) != "zh-CN" {
		t.Fatalf("sourceLocale = %q", result["sourceLocale"])
	}
}

func TestServeToolCallI18NReportMissing(t *testing.T) {
	root := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_report", "arguments": map[string]any{"root": root}},
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
		t.Fatalf("i18n report missing unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), "run `make i18n-report`") {
		t.Fatalf("missing report content = %q", resultText(result))
	}
}

func TestServeToolCallI18NReportInvalidOutput(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_report", "arguments": map[string]any{"root": root, "output": "xml"}},
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
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), `i18n report: unsupported output "xml"; want text or json`) {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallI18NCheckDev(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_check", "arguments": map[string]any{"root": root, "mode": "dev"}},
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
		t.Fatalf("i18n check dev returned error: %s", resultText(result))
	}
	if !result["ok"].(bool) {
		t.Fatalf("ok = false, want true: %+v", result)
	}
	if result["mode"].(string) != "dev" {
		t.Fatalf("mode = %q", result["mode"])
	}
	if got := len(result["failures"].([]any)); got != 0 {
		t.Fatalf("failures len = %d, want 0", got)
	}
	if got := len(result["warnings"].([]any)); got != 1 {
		t.Fatalf("warnings len = %d, want 1", got)
	}
	if !strings.Contains(resultText(result), "i18n check (dev): ok") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallI18NCheckJSON(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_check", "arguments": map[string]any{"root": root, "mode": "dev", "output": "json"}},
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
		t.Fatalf("i18n check json returned error: %s", resultText(result))
	}
	content := resultJSONObject(t, result)
	if content["mode"] != "dev" || content["ok"] != true {
		t.Fatalf("json content = %+v", content)
	}
	if !result["ok"].(bool) {
		t.Fatalf("ok = false, want true: %+v", result)
	}
}

func TestServeToolCallI18NCheckRelease(t *testing.T) {
	root := seedMCPI18NCheckReleaseProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_check", "arguments": map[string]any{"root": root, "mode": "release"}},
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
		t.Fatalf("i18n check release unexpectedly succeeded: %+v", result)
	}
	if result["ok"].(bool) {
		t.Fatalf("ok = true, want false")
	}
	if got := len(result["failures"].([]any)); got < 3 {
		t.Fatalf("failures len = %d, want at least 3", got)
	}
	if !strings.Contains(resultText(result), "i18n check (release): failed") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallI18NCheckInvalidMode(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_check", "arguments": map[string]any{"root": root, "mode": "strict"}},
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
		t.Fatalf("invalid mode unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), "unsupported --mode") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallI18NCheckInvalidOutput(t *testing.T) {
	root := seedMCPI18NProject(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_i18n_check", "arguments": map[string]any{"root": root, "mode": "dev", "output": "xml"}},
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
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), `i18n check: unsupported output "xml"; want text or json`) {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallProtolintOK(t *testing.T) {
	root := mcpFixtureRoot(t, "..", "scaffold", "mono", "testdata", "mono-default", "idl")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"app/demo.proto"}, "rules": []string{"PIO101", "PIO102"}}},
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
		t.Fatalf("protolint ok returned error: %s", resultText(result))
	}
	if !result["ok"].(bool) {
		t.Fatalf("ok = false, want true: %+v", result)
	}
	if got := len(result["diagnostics"].([]any)); got != 0 {
		t.Fatalf("diagnostics len = %d, want 0", got)
	}
	if !strings.Contains(resultText(result), "protolint: ok") {
		t.Fatalf("content = %q", resultText(result))
	}
	if result["root"].(string) != root {
		t.Fatalf("root = %q, want %q", result["root"], root)
	}
	if got := len(result["rulesRun"].([]any)); got != 2 {
		t.Fatalf("rulesRun len = %d, want 2", got)
	}
}

func TestServeToolCallProtolintFailure(t *testing.T) {
	root := mcpFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"invalid.proto"}, "rules": []string{"PIO301"}}},
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
		t.Fatalf("protolint failure unexpectedly succeeded: %+v", result)
	}
	if result["ok"].(bool) {
		t.Fatalf("ok = true, want false")
	}
	if got := len(result["diagnostics"].([]any)); got != 1 {
		t.Fatalf("diagnostics len = %d, want 1", got)
	}
	if !strings.Contains(resultText(result), "PIO301") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallProtolintSARIF(t *testing.T) {
	root := mcpFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"invalid.proto"}, "rules": []string{"PIO301"}, "output": "sarif"}},
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
		t.Fatalf("protolint sarif failure unexpectedly succeeded: %+v", result)
	}
	content := resultText(result)
	if !strings.Contains(content, `"version": "2.1.0"`) || !strings.Contains(content, `"ruleId": "PIO301"`) {
		t.Fatalf("content = %q", content)
	}
	if got := len(result["diagnostics"].([]any)); got != 1 {
		t.Fatalf("diagnostics len = %d, want 1", got)
	}
}

func TestServeToolCallProtolintWarningsAreNonBlocking(t *testing.T) {
	root := mcpFixtureRoot(t, "..", "protolint", "testdata", "phase2warnings")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"invalid.proto"}, "rules": []string{"PIO111", "PIO112", "PIO113"}}},
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
		t.Fatalf("warning-only protolint should not be error: %+v", result)
	}
	if !result["ok"].(bool) {
		t.Fatalf("ok = false, want true: %+v", result)
	}
	if got := len(result["diagnostics"].([]any)); got != 5 {
		t.Fatalf("diagnostics len = %d, want 5", got)
	}
	if !strings.Contains(resultText(result), "! [PIO111]") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallProtolintIgnoreRule(t *testing.T) {
	root := mcpFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"invalid.proto"}, "rules": []string{"PIO301"}, "ignoreRules": []string{"PIO301"}}},
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
		t.Fatalf("ignored protolint should not be error: %+v", result)
	}
	if !result["ok"].(bool) {
		t.Fatalf("ok = false, want true: %+v", result)
	}
	if got := len(result["diagnostics"].([]any)); got != 0 {
		t.Fatalf("diagnostics len = %d, want 0", got)
	}
	if got := len(result["ignoredRules"].([]any)); got != 1 {
		t.Fatalf("ignoredRules len = %d, want 1", got)
	}
	if !strings.Contains(resultText(result), "suppressed=1") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallProtolintWorkspaceAutoDiscovery(t *testing.T) {
	root := seedMCPProtoWorkspace(t)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "rules": []string{"PIO301"}}},
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
		t.Fatalf("workspace protolint unexpectedly succeeded: %+v", result)
	}
	if result["ok"].(bool) {
		t.Fatalf("ok = true, want false")
	}
	if got := len(result["files"].([]any)); got != 2 {
		t.Fatalf("files len = %d, want 2", got)
	}
	if !strings.Contains(resultText(result), "services/order-rpc/idl/orderrpc.proto") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallProtolintInvalidArgs(t *testing.T) {
	root := mcpFixtureRoot(t, "..", "protolint", "testdata", "kitexenvelope")
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"invalid.proto"}, "rules": []string{"PIO999"}}},
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
		t.Fatalf("invalid rule unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), "unknown rule") {
		t.Fatalf("content = %q", resultText(result))
	}
	input = EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root}},
	})
	out.Reset()
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err = DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result = responses[0].Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("missing files unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), "at least one --file is required unless --root points to an ncgo service or micro workspace") {
		t.Fatalf("content = %q", resultText(result))
	}
	input = EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_protolint", "arguments": map[string]any{"root": root, "files": []string{"invalid.proto"}, "output": "xml"}},
	})
	out.Reset()
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err = DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result = responses[0].Result.(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), "unsupported output \"xml\"; want text, json, or sarif") {
		t.Fatalf("content = %q", resultText(result))
	}
}

func TestServeToolCallAddInfraDryRun(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_infra", "arguments": map[string]any{"root": root, "kind": "redis", "dryRun": true}},
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
	if _, err := os.Stat(filepath.Join(root, "internal", "base", "data", "redis.go")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote redis file: stat err = %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 0 {
		t.Fatalf("dry-run updated manifest infra = %v, want empty", m.Infra)
	}
}

func TestServeToolCallAddInfraJSON(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_infra", "arguments": map[string]any{"root": root, "kind": "redis", "dryRun": true, "output": "json"}},
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
		t.Fatalf("add infra json returned error: %s", resultText(result))
	}
	content := resultJSONObject(t, result)
	if content["dryRun"] != true || content["updated"] != true {
		t.Fatalf("json content = %+v", content)
	}
	if !mcpPlanContains(result["plan"].([]any), "file", "create") {
		t.Fatalf("plan missing file create: %+v", result["plan"])
	}
}

func TestServeToolCallAddInfraInvalidOutput(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_infra", "arguments": map[string]any{"root": root, "kind": "redis", "output": "xml"}},
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
		t.Fatalf("invalid output unexpectedly succeeded: %+v", result)
	}
	if !strings.Contains(resultText(result), `add infra: unsupported output "xml"; want text or json`) {
		t.Fatalf("content = %q", resultText(result))
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

func seedMCPProtoWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.SaveWorkspace(root, &manifest.Workspace{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMicro,
		Name:   "commerce",
		Module: "github.com/x/commerce",
		Services: []manifest.WorkspaceService{
			{Name: "user-rpc", Kind: manifest.KindKitex, Dir: "services/user-rpc"},
			{Name: "order-rpc", Kind: manifest.KindKitex, Dir: "services/order-rpc"},
		},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save workspace: %v", err)
	}
	seedMCPProtoService(t, root, "services/user-rpc", "user-rpc", "idl/userrpc.proto", `syntax = "proto3";

package user;

message PingReq {}
message PingResp {}

service User {
  rpc Ping(PingReq) returns (PingResp) {}
}
`)
	seedMCPProtoService(t, root, "services/order-rpc", "order-rpc", "idl/orderrpc.proto", `syntax = "proto3";

package order;

message GetOrderReq {}
message GetOrderResp {
  int32 code = 1;
  string msg = 2;
  bool success = 3;
  string order_id = 4;
}

service Order {
  rpc GetOrder(GetOrderReq) returns (GetOrderResp) {}
}
`)
	return root
}

func seedMCPProtoService(t *testing.T, workspaceRoot, serviceRel, name, idl, protoBody string) {
	t.Helper()
	serviceRoot := filepath.Join(workspaceRoot, serviceRel)
	if err := manifest.Save(serviceRoot, &manifest.Manifest{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:        manifest.ModeMono,
		Module:      "github.com/x/commerce/" + filepath.ToSlash(serviceRel),
		Service:     manifest.Service{Name: name, Kind: manifest.KindKitex, IDL: idl},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save service manifest: %v", err)
	}
	protoPath := filepath.Join(serviceRoot, filepath.FromSlash(idl))
	if err := os.MkdirAll(filepath.Dir(protoPath), 0o755); err != nil {
		t.Fatalf("mkdir proto dir: %v", err)
	}
	if err := os.WriteFile(protoPath, []byte(protoBody), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
}

func seedMCPI18NProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	metaDir := filepath.Join(root, "internal", "pkg", "i18n", ".meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	reportJSON := `{
	  "summary": {
	    "locale_count": 2,
	    "message_key_count": 1,
	    "missing_source_count": 0,
	    "missing_translations_count": 1,
	    "stale_translations_count": 0,
	    "draft_translations_count": 0,
	    "extra_keys_count": 0,
	    "glossary_hints_count": 1
	  },
	  "source_locale": "zh-CN",
	  "missing_source": [],
	  "missing_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "__TODO__: 内部错误", "status": "draft"}],
	  "stale_translations": [],
	  "draft_translations": [],
	  "extra_keys": [],
	  "glossary_hints": [{"language": "it-IT", "key": "internal_error", "term": "signature", "recommended": "firma", "current_text": "errore interno"}]
	}`
	if err := os.WriteFile(filepath.Join(metaDir, "report.json"), []byte(reportJSON), 0o644); err != nil {
		t.Fatalf("write report.json: %v", err)
	}
	return root
}

func seedMCPI18NCheckReleaseProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	metaDir := filepath.Join(root, "internal", "pkg", "i18n", ".meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	reportJSON := `{
	  "summary": {
	    "locale_count": 2,
	    "message_key_count": 1,
	    "missing_source_count": 0,
	    "missing_translations_count": 1,
	    "stale_translations_count": 1,
	    "draft_translations_count": 1,
	    "extra_keys_count": 0,
	    "glossary_hints_count": 1
	  },
	  "source_locale": "zh-CN",
	  "missing_source": [],
	  "missing_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "__TODO__: 内部错误", "status": "draft"}],
	  "stale_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "errore interno", "status": "stale"}],
	  "draft_translations": [{"language": "it-IT", "key": "internal_error", "source_text": "内部错误", "current_text": "errore interno", "status": "draft"}],
	  "extra_keys": [],
	  "glossary_hints": [{"language": "it-IT", "key": "internal_error", "term": "signature", "recommended": "firma", "current_text": "errore interno"}]
	}`
	if err := os.WriteFile(filepath.Join(metaDir, "report.json"), []byte(reportJSON), 0o644); err != nil {
		t.Fatalf("write report.json: %v", err)
	}
	return root
}

func mcpFixtureRoot(t *testing.T, elems ...string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join(elems...))
	if err != nil {
		t.Fatalf("fixture abs path: %v", err)
	}
	return root
}

func resultText(result map[string]any) string {
	return result["content"].([]any)[0].(map[string]any)["text"].(string)
}

func resultJSONObject(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(resultText(result)), &out); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, resultText(result))
	}
	return out
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
	return slices.Contains(xs, want)
}

func TestServeToolCallNew(t *testing.T) {
	dir := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_new", "arguments": map[string]any{"name": "demo", "module": "github.com/x/demo", "dir": dir, "noGenerate": true}},
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
		t.Fatalf("new returned error: %s", resultText(result))
	}
	if result["mode"].(string) != manifest.ModeMono {
		t.Fatalf("mode = %q, want %q", result["mode"], manifest.ModeMono)
	}
	if result["ranGenerate"].(bool) {
		t.Fatalf("ranGenerate = true, want false (noGenerate set)")
	}
	content := resultText(result)
	if !strings.Contains(content, "scaffolded") {
		t.Fatalf("content missing 'scaffolded': %s", content)
	}
	if !strings.Contains(content, "next steps") {
		t.Fatalf("content missing 'next steps': %s", content)
	}
}

func TestServeToolCallNewMicro(t *testing.T) {
	dir := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_new", "arguments": map[string]any{"name": "commerce", "module": "github.com/x/commerce", "mode": "micro", "dir": dir}},
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
		t.Fatalf("new micro returned error: %s", resultText(result))
	}
	if result["mode"].(string) != manifest.ModeMicro {
		t.Fatalf("mode = %q, want %q", result["mode"], manifest.ModeMicro)
	}
}

func TestServeToolCallNewMissingModule(t *testing.T) {
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_new", "arguments": map[string]any{"name": "demo"}},
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
		t.Fatalf("expected error for missing module")
	}
	if !strings.Contains(resultText(result), "module is required") {
		t.Fatalf("content = %q, want 'module is required'", resultText(result))
	}
}

func TestServeToolCallAddDomain(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_domain", "arguments": map[string]any{"name": "device", "root": root}},
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
		t.Fatalf("add domain returned error: %s", resultText(result))
	}
	if result["dryRun"].(bool) {
		t.Fatalf("dryRun = true, want false")
	}
	if !result["updated"].(bool) {
		t.Fatalf("updated = false, want true")
	}
	if got := len(result["writtenPaths"].([]any)); got != 3 {
		t.Fatalf("writtenPaths len = %d, want 3", got)
	}
	content := resultText(result)
	for _, want := range []string{"wrote ", "internal/usecase/device", "internal/repository/device", "internal/base/data/device_register"} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q:\n%s", want, content)
		}
	}
}

func TestServeToolCallAddDomainDryRun(t *testing.T) {
	root := seedMCPProject(t, manifest.KindHertz)
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_add_domain", "arguments": map[string]any{"name": "device", "root": root, "dryRun": true}},
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
		t.Fatalf("add domain dryRun returned error: %s", resultText(result))
	}
	if !result["dryRun"].(bool) {
		t.Fatalf("dryRun = false, want true")
	}
	content := resultText(result)
	if !strings.Contains(content, "would write") {
		t.Fatalf("content missing 'would write': %s", content)
	}
}
