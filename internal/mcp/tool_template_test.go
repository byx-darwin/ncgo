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
	"github.com/byx-darwin/ncgo/internal/manifest"
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
		// pre-commit injects its hook wrapper into child git processes (via
		// GIT_CONFIG* env) when tests run under pre-commit / pre-push; the
		// fixture repo has no .pre-commit-config.yaml, so silence that check.
		cmd.Env = append(os.Environ(), "PRE_COMMIT_ALLOW_NO_CONFIG=1")
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

// scrubGitEnv unsets git environment variables that the outer pre-push hook
// leaks into the test process (git sets GIT_DIR and friends when running a
// hook, and pre-commit forwards them into `go test`). Left in place, fixture
// git commands resolve against the outer repository instead of t.TempDir().
// Original values are restored when the test finishes.
func scrubGitEnv(t *testing.T) {
	t.Helper()
	unset := func(key string) {
		if v, ok := os.LookupEnv(key); ok {
			_ = os.Unsetenv(key)
			key, value := key, v
			t.Cleanup(func() { _ = os.Setenv(key, value) })
		}
	}
	for _, key := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR",
		"GIT_OBJECT_DIRECTORY", "GIT_CEILING_DIRECTORIES",
		"GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM",
	} {
		unset(key)
	}
	// pre-commit forwards GIT_CONFIG_KEY_*/GIT_CONFIG_VALUE_* entries; scrub
	// any remaining GIT_CONFIG-prefixed variable.
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "GIT_CONFIG") {
			unset(key)
		}
	}
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
	scrubGitEnv(t)
	repo := registryFixture(t, map[string]string{
		"base-kitex/template.yaml": "name: base-kitex\nkind: kitex\ndescription: base kitex\n",
	})
	isolateCache(t)

	result, err := callTemplateList(context.Background(), []byte(`{"registry":"`+repo+`"}`))
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
	scrubGitEnv(t)
	if _, err := goexec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	isolateCache(t)

	result, err := callTemplateList(context.Background(), []byte(`{"registry":"/definitely/not/a/real/registry"}`))
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
	scrubGitEnv(t)
	repo := registryFixture(t, map[string]string{
		"base-hertz/template.yaml": "name: base-hertz\nkind: hertz\ndescription: base hertz\n",
	})
	isolateCache(t)

	result, err := callTemplatePull(context.Background(), []byte(`{"name":"nope","registry":"`+repo+`"}`))
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
	scrubGitEnv(t)
	repo := registryFixture(t, map[string]string{
		"base-kitex/template.yaml": "name: base-kitex\nkind: kitex\ndescription: base kitex\n",
	})
	isolateCache(t)

	result, err := callTemplatePull(context.Background(), []byte(`{"name":"base-kitex","registry":"`+repo+`"}`))
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
	scrubGitEnv(t)
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

// seedMCPExportProject creates a minimal Hertz/Kitex project with a valid
// manifest and source files matching the export template rules, but NO idl/
// dir, so the zero-IDL export branch (idls == []) is exercised. It mirrors
// the shape used by internal/cli/export_test.go seedExportProject.
func seedMCPExportProject(t *testing.T, kind string) string {
	t.Helper()
	root := seedMCPProject(t, kind)
	write := func(rel, content string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", abs, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	if kind == manifest.KindKitex {
		for _, rel := range []string{
			"main.go",
			"conf/dev/conf.yaml",
			"internal/base/conf/conf.go",
			"internal/base/server/server.go",
			"internal/base/data/data.go",
			"internal/pkg/utils/helper.go",
			"internal/base/middleware/mw.go",
			"internal/base/release/release.go",
			"internal/base/logging/log.go",
		} {
			write(rel, "package x\n")
		}
		return root
	}
	for _, rel := range []string{
		"main.go",
		"conf/dev/conf.yaml",
		"internal/base/conf/conf.go",
		"internal/base/server/server.go",
		"internal/base/data/data.go",
		"internal/router/demo/router.go",
		"internal/pkg/utils/helper.go",
		"internal/base/logging/log.go",
	} {
		write(rel, "package x\n")
	}
	return root
}

// TestCallExportTemplatesZeroIDL covers the ncgo_export_templates `idls`
// structured field: a project without an idl/ dir must report an empty slice
// (rendered as [] in JSON, never null) and use the zero-IDL text branch.
func TestCallExportTemplatesZeroIDL(t *testing.T) {
	root := seedMCPExportProject(t, manifest.KindHertz)

	// callExportTemplates sandboxes roots to the MCP workspace (cwd); relax
	// the boundary for this hermetic temp-project test and restore after.
	orig := resolvePath
	resolvePath = func(target string) (string, error) { return filepath.Abs(target) }
	t.Cleanup(func() { resolvePath = orig })

	// Text (default) output.
	res, err := callExportTemplates([]byte(`{"root":"` + root + `"}`))
	if err != nil {
		t.Fatalf("callExportTemplates: %v", err)
	}
	if res["isError"].(bool) {
		t.Fatalf("isError = true, want false: %s", toolText(res))
	}
	idls, ok := res["idls"].([]string)
	if !ok {
		t.Fatalf("idls = %T, want []string", res["idls"])
	}
	if len(idls) != 0 {
		t.Fatalf("idls = %v, want empty (no idl/ dir)", idls)
	}
	if txt := toolText(res); !strings.Contains(txt, "exported ") || strings.Contains(txt, "IDL files") {
		t.Fatalf("zero-IDL text branch wrong: %s", txt)
	}

	// JSON output renders idls as [] (not null).
	jsonRes, err := callExportTemplates([]byte(`{"root":"` + root + `","output":"json"}`))
	if err != nil {
		t.Fatalf("callExportTemplates json: %v", err)
	}
	if jsonRes["isError"].(bool) {
		t.Fatalf("json isError = true, want false: %s", toolText(jsonRes))
	}
	if json := toolText(jsonRes); !strings.Contains(json, `"idls": []`) {
		t.Fatalf("json idls not rendered as []: %s", json)
	}
}
