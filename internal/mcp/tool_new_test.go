package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/postgenerate"
)

func newAutoStepTestResult() *newResult {
	ran := true
	return &newResult{
		Dir:         "/tmp/demo",
		NextSteps:   []string{"make dev", "make migrate-up"},
		Mode:        "mono",
		RanGenerate: &ran,
		AutoSteps: []postgenerate.StepResult{
			{Name: "go mod tidy", Status: "succeeded", Detail: "(0.5s)"},
			{Name: "ai sync", Status: "skipped", Detail: "target=none"},
		},
	}
}

// TestNewMCPToolSchemaDeclaresAutoStepArgs verifies the ncgo_new InputSchema
// declares aiTarget and noAutoSteps. schemaObject sets additionalProperties
// to false, so without these declarations MCP clients cannot pass the args.
func TestNewMCPToolSchemaDeclaresAutoStepArgs(t *testing.T) {
	var newTool tool
	for _, tl := range (&Server{}).tools() {
		if tl.Name == "ncgo_new" {
			newTool = tl
			break
		}
	}
	if newTool.Name == "" {
		t.Fatal("ncgo_new tool not found in tools()")
	}
	props, ok := newTool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("ncgo_new schema missing properties: %+v", newTool.InputSchema)
	}
	aiTarget, ok := props["aiTarget"].(map[string]any)
	if !ok || aiTarget["type"] != "string" {
		t.Fatalf("ncgo_new schema aiTarget = %+v, want string property", props["aiTarget"])
	}
	noAutoSteps, ok := props["noAutoSteps"].(map[string]any)
	if !ok || noAutoSteps["type"] != "boolean" {
		t.Fatalf("ncgo_new schema noAutoSteps = %+v, want boolean property", props["noAutoSteps"])
	}
}

// TestNewMCPToolFieldsAutoSteps verifies the top-level structured fields
// include autoSteps when auto steps ran.
func TestNewMCPToolFieldsAutoSteps(t *testing.T) {
	res := newAutoStepTestResult()
	fields := newMCPTool.fields(res)
	steps, ok := fields["autoSteps"].([]postgenerate.StepResult)
	if !ok {
		t.Fatalf("fields missing autoSteps or wrong type: %+v", fields)
	}
	if len(steps) != 2 {
		t.Fatalf("autoSteps len = %d, want 2", len(steps))
	}
}

// TestNewMCPToolFieldsNoAutoSteps verifies autoSteps is omitted from the
// top-level fields when no auto steps ran (e.g. micro mode or noGenerate).
func TestNewMCPToolFieldsNoAutoSteps(t *testing.T) {
	ran := false
	res := &newResult{Dir: "/tmp/demo", NextSteps: []string{"go mod tidy"}, Mode: "mono", RanGenerate: &ran}
	fields := newMCPTool.fields(res)
	if _, ok := fields["autoSteps"]; ok {
		t.Fatalf("fields should not include autoSteps when empty: %+v", fields)
	}
}

// TestFormatMCPNewOutputAutoStepsText verifies text output renders auto steps.
func TestFormatMCPNewOutputAutoStepsText(t *testing.T) {
	got, err := formatMCPNewOutput(newAutoStepTestResult(), mcpOutputText)
	if err != nil {
		t.Fatalf("formatMCPNewOutput text: %v", err)
	}
	for _, want := range []string{
		"auto steps:",
		"go mod tidy: succeeded (0.5s)",
		"ai sync: skipped target=none",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("text output missing %q:\n%s", want, got)
		}
	}
}

// TestFormatMCPNewOutputAutoStepsJSON verifies JSON output includes autoSteps
// with the expected name/status/detail shape.
func TestFormatMCPNewOutputAutoStepsJSON(t *testing.T) {
	got, err := formatMCPNewOutput(newAutoStepTestResult(), mcpOutputJSON)
	if err != nil {
		t.Fatalf("formatMCPNewOutput json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, got)
	}
	steps, ok := parsed["autoSteps"].([]any)
	if !ok {
		t.Fatalf("json output missing autoSteps: %+v", parsed)
	}
	if len(steps) != 2 {
		t.Fatalf("autoSteps len = %d, want 2", len(steps))
	}
	first, ok := steps[0].(map[string]any)
	if !ok {
		t.Fatalf("autoSteps[0] not an object: %+v", steps[0])
	}
	if first["name"] != "go mod tidy" || first["status"] != "succeeded" {
		t.Fatalf("autoSteps[0] = %+v, want go mod tidy succeeded", first)
	}
}

// TestFormatMCPNewOutputNoAutoSteps verifies auto steps are not rendered when
// none ran.
func TestFormatMCPNewOutputNoAutoSteps(t *testing.T) {
	ran := true
	res := &newResult{Dir: "/tmp/demo", NextSteps: []string{"make dev"}, Mode: "mono", RanGenerate: &ran}

	text, err := formatMCPNewOutput(res, mcpOutputText)
	if err != nil {
		t.Fatalf("formatMCPNewOutput text: %v", err)
	}
	if strings.Contains(text, "auto steps") {
		t.Fatalf("text should not contain auto steps section:\n%s", text)
	}

	js, err := formatMCPNewOutput(res, mcpOutputJSON)
	if err != nil {
		t.Fatalf("formatMCPNewOutput json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(js), &parsed); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, js)
	}
	if _, ok := parsed["autoSteps"]; ok {
		t.Fatalf("json output should not include autoSteps: %+v", parsed)
	}
}

// TestServeToolCallNewAutoStepArgs verifies ncgo_new accepts aiTarget and
// noAutoSteps args end-to-end and omits autoSteps when generation is skipped.
func TestServeToolCallNewAutoStepArgs(t *testing.T) {
	allowAnyRootForTest(t)
	dir := t.TempDir()
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_new", "arguments": map[string]any{
			"name": "demo", "module": "github.com/x/demo", "dir": dir,
			"noGenerate": true, "aiTarget": "none", "noAutoSteps": true,
		}},
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
	if _, ok := result["autoSteps"]; ok {
		t.Fatalf("noGenerate should not surface autoSteps: %+v", result)
	}
}

func TestServeToolCallNewDefaultsToAllAgentContextsWithoutGenerator(t *testing.T) {
	allowAnyRootForTest(t)
	for _, tc := range []struct {
		name string
		mode string
	}{
		{name: "mono", mode: manifest.ModeMono},
		{name: "micro", mode: manifest.ModeMicro},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "demo")
			args := map[string]any{
				"name": "demo", "module": "github.com/x/demo", "dir": dir,
				"mode": tc.mode,
			}
			if tc.mode == manifest.ModeMono {
				args["noGenerate"] = true
			}
			input := EncodeMessage(map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": "ncgo_new", "arguments": args},
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
			for _, rel := range []string{"AGENTS.md", "CLAUDE.md", ".claude/skills/ncgo-dev/SKILL.md", ".claude/generated/project-context.md", ".cursor/rules/ncgo.mdc"} {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
					t.Errorf("default ncgo_new did not write %s: %v", rel, err)
				}
			}
		})
	}
}

func TestCallNewMonoUsesTemplateDir(t *testing.T) {
	allowAnyRootForTest(t)
	templateDir := t.TempDir()
	kitexTemplates := filepath.Join(templateDir, "kitex-template")
	if err := os.MkdirAll(kitexTemplates, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kitexTemplates, "main_go.yaml"), []byte("path: main.go\nupdate_behavior:\n  type: cover\nbody: |\n  package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "demo")
	args, _ := json.Marshal(map[string]any{
		"name": "demo", "module": "github.com/x/demo", "dir": target,
		"kind": "kitex", "templateDir": templateDir, "noGenerate": true,
		"aiTarget": "none", "noAutoSteps": true,
	})
	result, err := callNew(context.Background(), args, "test-version", "test-assets")
	if err != nil {
		t.Fatalf("callNew: %v", err)
	}
	if result["isError"].(bool) {
		t.Fatalf("callNew returned error: %s", toolText(result))
	}
	entries, err := os.ReadDir(filepath.Join(target, "template", "kitex-template"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "main_go.yaml" {
		t.Fatalf("mono template package was not used: %v", entries)
	}
}
