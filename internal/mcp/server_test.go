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
	for _, want := range []string{"ncgo_version", "ncgo_doctor", "ncgo_ai_sync", "ncgo_i18n_report", "ncgo_i18n_check", "ncgo_protolint", "ncgo_add_infra", "ncgo_add_method"} {
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
	if !strings.Contains(resultText(result), "at least one file is required") {
		t.Fatalf("content = %q", resultText(result))
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
