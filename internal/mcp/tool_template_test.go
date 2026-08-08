package mcp

import (
	"bytes"
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	goexec "github.com/byx-darwin/ncgo/internal/exec"
)

// registryFixture builds a local git repository at t.TempDir() containing the
// given files (relative path -> contents) committed to a branch. Tests skip
// when git is unavailable on PATH.
func registryFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := goexec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	repo := t.TempDir()
	for name, content := range files {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		cmd := osexec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", ".")
	run("-c", "user.email=test@example.com", "-c", "user.name=ncgo test", "commit", "-q", "-m", "fixture")
	return repo
}

// isolateCache redirects os.UserCacheDir() to a fresh temp dir so registry
// cache clones land in a per-test location instead of the shared user cache.
func isolateCache(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// toolText extracts content[0].text from a result produced by a direct
// handler call (before JSON round-trip, where content is []map[string]string).
func toolText(result map[string]any) string {
	content, ok := result["content"].([]map[string]string)
	if !ok || len(content) == 0 {
		return ""
	}
	return content[0]["text"]
}

func TestCallTemplateListFixture(t *testing.T) {
	repo := registryFixture(t, map[string]string{
		"base-kitex/template.yaml": "name: base-kitex\nkind: kitex\ndescription: base kitex\n",
	})
	isolateCache(t)

	result, err := callTemplateList([]byte(`{"registry":"` + repo + `"}`))
	if err != nil {
		t.Fatalf("callTemplateList: %v", err)
	}
	if result["isError"].(bool) {
		t.Fatalf("isError = true, want false: %s", toolText(result))
	}
	templates, ok := result["templates"].([]map[string]any)
	if !ok {
		t.Fatalf("result missing templates field or wrong type: %T", result["templates"])
	}
	if len(templates) != 1 {
		t.Fatalf("templates = %+v, want exactly the fixture entry", templates)
	}
	entry := templates[0]
	if entry["name"] != "base-kitex" || entry["kind"] != "kitex" || entry["description"] != "base kitex" {
		t.Errorf("entry = %+v, want name/kind/description", entry)
	}
	if !strings.Contains(toolText(result), "base-kitex") {
		t.Errorf("content text missing template name: %s", toolText(result))
	}
}

func TestCallTemplateListRegistryUnavailable(t *testing.T) {
	if _, err := goexec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	isolateCache(t)

	result, err := callTemplateList([]byte(`{"registry":"/definitely/not/a/real/registry"}`))
	if err != nil {
		t.Fatalf("callTemplateList: %v", err)
	}
	if !result["isError"].(bool) {
		t.Fatalf("isError = false, want true: %s", toolText(result))
	}
	if !strings.Contains(toolText(result), "registry unavailable") {
		t.Errorf("text missing 'registry unavailable': %s", toolText(result))
	}
}

func TestCallTemplatePullMissing(t *testing.T) {
	repo := registryFixture(t, map[string]string{
		"base-hertz/template.yaml": "name: base-hertz\nkind: hertz\ndescription: base hertz\n",
	})
	isolateCache(t)

	result, err := callTemplatePull([]byte(`{"name":"nope","registry":"` + repo + `"}`))
	if err != nil {
		t.Fatalf("callTemplatePull: %v", err)
	}
	if !result["isError"].(bool) {
		t.Fatalf("isError = false, want true: %s", toolText(result))
	}
	if !strings.Contains(toolText(result), "not found in registry") {
		t.Errorf("text missing 'not found in registry': %s", toolText(result))
	}
}

func TestCallTemplatePullFixture(t *testing.T) {
	repo := registryFixture(t, map[string]string{
		"base-kitex/template.yaml": "name: base-kitex\nkind: kitex\ndescription: base kitex\n",
	})
	isolateCache(t)

	result, err := callTemplatePull([]byte(`{"name":"base-kitex","registry":"` + repo + `"}`))
	if err != nil {
		t.Fatalf("callTemplatePull: %v", err)
	}
	if result["isError"].(bool) {
		t.Fatalf("isError = true, want false: %s", toolText(result))
	}
	name, ok := result["name"].(string)
	if !ok || name != "base-kitex" {
		t.Errorf("result name = %v, want base-kitex", result["name"])
	}
	dir, ok := result["dir"].(string)
	if !ok || !strings.HasSuffix(dir, "base-kitex") {
		t.Errorf("result dir = %v, want path ending in base-kitex", result["dir"])
	}
	if !strings.Contains(toolText(result), "pulled base-kitex") {
		t.Errorf("text missing 'pulled base-kitex': %s", toolText(result))
	}
}

func TestTemplateToolsRegistered(t *testing.T) {
	s := New("test-version", "test-assets")
	var names []string
	for _, tt := range s.tools() {
		names = append(names, tt.Name)
	}
	for _, want := range []string{"ncgo_template_list", "ncgo_template_pull"} {
		if !contains(names, want) {
			t.Errorf("tools/list missing %q in %v", want, names)
		}
	}
}

func TestTemplateToolsSchema(t *testing.T) {
	s := New("test-version", "test-assets")
	var listSchema, pullSchema map[string]any
	for _, tt := range s.tools() {
		switch tt.Name {
		case "ncgo_template_list":
			listSchema = tt.InputSchema
		case "ncgo_template_pull":
			pullSchema = tt.InputSchema
		}
	}
	if listSchema == nil || pullSchema == nil {
		t.Fatal("new template tools missing from tools/list")
	}
	if req, ok := listSchema["required"].([]string); !ok || len(req) != 0 {
		t.Errorf("ncgo_template_list required = %v, want empty (no required fields)", listSchema["required"])
	}
	if req, ok := pullSchema["required"].([]string); !ok || len(req) != 1 || req[0] != "name" {
		t.Errorf("ncgo_template_pull required = %v, want [name]", pullSchema["required"])
	}
	props := listSchema["properties"].(map[string]any)
	for _, key := range []string{"registry", "output"} {
		if _, ok := props[key]; !ok {
			t.Errorf("ncgo_template_list missing property %q", key)
		}
	}
	props = pullSchema["properties"].(map[string]any)
	for _, key := range []string{"name", "registry", "output"} {
		if _, ok := props[key]; !ok {
			t.Errorf("ncgo_template_pull missing property %q", key)
		}
	}
}

// TestServeTemplateListDispatch exercises the full MCP server path for
// ncgo_template_list, verifying the tools/call switch dispatch and the
// JSON round-trip shape of the structured templates field.
func TestServeTemplateListDispatch(t *testing.T) {
	repo := registryFixture(t, map[string]string{
		"base-kitex/template.yaml": "name: base-kitex\nkind: kitex\ndescription: base kitex\n",
	})
	isolateCache(t)

	var out bytes.Buffer
	input := EncodeMessage(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "ncgo_template_list", "arguments": map[string]any{"registry": repo}},
	})
	if err := New("test-version", "test-assets").Serve(context.Background(), bytes.NewReader(input), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	responses, err := DecodeResponses(out.Bytes())
	if err != nil {
		t.Fatalf("DecodeResponses: %v", err)
	}
	result := responses[0].Result.(map[string]any)
	if result["isError"].(bool) {
		t.Fatalf("isError = true, want false: %s", resultText(result))
	}
	templates, ok := result["templates"].([]any)
	if !ok || len(templates) != 1 {
		t.Fatalf("templates = %#v, want one entry after JSON round-trip", result["templates"])
	}
	entry := templates[0].(map[string]any)
	if entry["name"] != "base-kitex" || entry["kind"] != "kitex" || entry["description"] != "base kitex" {
		t.Errorf("entry = %+v, want name/kind/description", entry)
	}
	if !strings.Contains(resultText(result), "base-kitex") {
		t.Errorf("content text missing template name: %s", resultText(result))
	}
}

func TestExportTemplatesDescription(t *testing.T) {
	s := New("test-version", "test-assets")
	var desc string
	for _, tt := range s.tools() {
		if tt.Name == "ncgo_export_templates" {
			desc = tt.Description
			break
		}
	}
	if desc == "" {
		t.Fatal("ncgo_export_templates not found in tools/list")
	}
	if strings.Contains(desc, "embedded scaffold") {
		t.Errorf("description still says 'embedded scaffold': %q", desc)
	}
	if !strings.Contains(desc, "template/<kind>-template/") {
		t.Errorf("description missing 'template/<kind>-template/': %q", desc)
	}
}
