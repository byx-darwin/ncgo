package mono

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
	"github.com/byx-darwin/ncgo/internal/scaffold/shared"
	"gopkg.in/yaml.v3"
)

const hertzAPIProto = `syntax = "proto2";

package api;

import "google/protobuf/descriptor.proto";

option go_package = "/api";

extend google.protobuf.FieldOptions {
  optional string raw_body = 50101;
  optional string query = 50102;
  optional string header = 50103;
  optional string cookie = 50104;
  optional string body = 50105;
  optional string path = 50106;
  optional string vd = 50107;
  optional string form = 50108;
  optional string js_conv = 50109;
  optional string file_name = 50110;
  optional string none = 50111;

  // 50131~50160 used to extend field option by hz
  optional string form_compatible = 50131;
  optional string js_conv_compatible = 50132;
  optional string file_name_compatible = 50133;
  optional string none_compatible = 50134;

  optional string go_tag = 51001;
}

extend google.protobuf.MethodOptions {
  optional string get = 50201;
  optional string post = 50202;
  optional string put = 50203;
  optional string delete = 50204;
  optional string patch = 50205;
  optional string options = 50206;
  optional string head = 50207;
  optional string any = 50208;
  optional string gen_path = 50301;
  optional string api_version = 50302;
  optional string tag = 50303;
  optional string name = 50304;
  optional string api_level = 50305;
  optional string serializer = 50306;
  optional string param = 50307;
  optional string baseurl = 50308;
  optional string handler_path = 50309;

  // 50331~50360 used to extend method option by hz
  optional string handler_path_compatible = 50331;
}

extend google.protobuf.EnumValueOptions {
  optional int32 http_code = 50401;
}

extend google.protobuf.ServiceOptions {
  optional string base_domain = 50402;

  // 50731~50760 used to extend service option by hz
  optional string base_domain_compatible = 50731;
  optional string service_path = 50732;
}

extend google.protobuf.MessageOptions {
  optional string reserve = 50830;
}`

// writeTemplate copies the kind-specific custom-template files from the
// embedded snapshot into <dir>/template/. For hertz it also writes a
// freshly rendered data.json so hz picks up the user's values; kitex
// reads its variables inline so no extra file is needed.
func writeTemplate(dir string, opts Options) error {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return writeKitexTemplate(dir, opts.Preset, opts.Module)
	}
	return writeHertzTemplate(dir, opts)
}

func writeHertzTemplate(dir string, opts Options) error {
	tplDir := filepath.Join(dir, "template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", tplDir, err)
	}
	srcFS := assets.FS()
	for _, name := range []string{"layout.yaml", "package.yaml"} {
		b, err := fs.ReadFile(srcFS, "hertz/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded hertz/%s: %w", name, err)
		}
		if name == "layout.yaml" {
			b, err = expandIncludes(b, srcFS)
			if err != nil {
				return fmt.Errorf("scaffold: expand hertz/layout.yaml: %w", err)
			}
		}
		if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
	data, err := renderDataJSON(opts)
	if err != nil {
		return fmt.Errorf("scaffold: render data.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "data.json"), data, 0o644); err != nil {
		return fmt.Errorf("scaffold: write data.json: %w", err)
	}
	// Copy hertz-template/*.yaml from embedded assets to <dir>/template/hertz-template/.
	// These per-file yaml templates override hz-generated files with go-tools
	// integrated versions via template.Apply().
	if err := copyHertzTemplateYAML(dir, srcFS); err != nil {
		return fmt.Errorf("scaffold: copy hertz-template: %w", err)
	}
	// Also write root-level cover-type templates (e.g. Makefile) directly to
	// the project root so they are available before template.Apply() runs
	// (required for NoGenerate path where `make sqlc` etc. are invoked).
	if err := writeHertzRootTemplates(dir, opts, srcFS); err != nil {
		return fmt.Errorf("scaffold: write hertz root templates: %w", err)
	}
	// Generate rule center client when address is provided
	if opts.RuleCenterAddr != "" {
		b, err := shared.ReadSharedFragmentBody(srcFS, "ratelimit/rule_center_client", opts.Module)
		if err != nil {
			return fmt.Errorf("scaffold: %w", err)
		}
		targetDir := filepath.Join(dir, "internal", "pkg", "middleware")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir %s: %w", targetDir, err)
		}
		target := filepath.Join(targetDir, "rule_center_client.go")
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write rule_center_client.go: %w", err)
		}
	}
	return nil
}

// writeKitexTemplate copies every embedded kitex-template/*.yaml verbatim
// into <dir>/template/kitex-template/ so that both `kitex` (during
// scaffold) and the generated Makefile's `update` target can consume
// them at the same path. When a preset is specified, it also writes
// preset-specific layout and extra files.
func writeKitexTemplate(dir string, preset string, module string) error {
	tplDir := filepath.Join(dir, "template", "kitex-template")
	if err := os.MkdirAll(tplDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", tplDir, err)
	}
	srcFS := assets.FS()
	entries, err := fs.ReadDir(srcFS, "kitex/kitex-template")
	if err != nil {
		return fmt.Errorf("scaffold: read embedded kitex/kitex-template: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// Rule-center template files are only copied when preset is "rule-center".
		if preset != "rule-center" && strings.HasPrefix(name, "ratelimit_") {
			continue
		}
		// Rule-center preset provides its own ratelimit_handler/server/usecase/repository
		// templates under the rulecenter/ dirs; skip the default per-layer templates
		// so they don't generate duplicate ruleservice/ scaffolding.
		if preset == "rule-center" && (name == "handler.yaml" || name == "server.yaml" || name == "usecase.yaml" || name == "repository.yaml") {
			continue
		}
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded kitex/kitex-template/%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
	extras := []struct {
		asset string
		path  string
	}{
		{asset: "kitex/sqlc.yaml", path: "internal/db/sqlc.yaml"},
		{asset: "kitex/query/health.sql", path: "internal/db/query/health.sql"},
		{asset: "kitex/schema/000001_placeholder.sql", path: "internal/db/schema/000001_placeholder.sql"},
	}
	// Rule-center preset adds additional schema and query files.
	if preset == "rule-center" {
		// Write the rate_limit_rules schema as pure SQL, stripping the YAML
		// frontmatter that would otherwise break sqlc parsing.
		b, err := shared.ReadSharedFragmentBody(srcFS, "kitex/schema/000002_rate_limit_rules", module)
		if err != nil {
			return fmt.Errorf("scaffold: read rule-center schema: %w", err)
		}
		full := filepath.Join(dir, "internal", "db", "schema", "000002_rate_limit_rules.sql")
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir schema: %w", err)
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write schema: %w", err)
		}
		// Copy shared ratelimit fragments into the kitex-template dir as
		// ratelimit_shared_*.yaml so the existing ratelimit_ prefix filter
		// copies them only for the rule-center preset. The layout-rulecenter.yaml
		// references these names in its templates: list.
		sharedFragments := []string{
			"ratelimit/resolver",
			"ratelimit/resolver_test",
			"ratelimit/store",
			"ratelimit/store_test",
			"ratelimit/rule_center_client",
		}
		for _, name := range sharedFragments {
			base := name[strings.LastIndex(name, "/")+1:]
			targetName := "ratelimit_shared_" + base + ".yaml"
			b, err := fs.ReadFile(srcFS, name+".yaml")
			if err != nil {
				return fmt.Errorf("scaffold: read embedded %s.yaml: %w", name, err)
			}
			if err := os.WriteFile(filepath.Join(tplDir, targetName), b, 0o644); err != nil {
				return fmt.Errorf("scaffold: write %s: %w", targetName, err)
			}
		}
	}
	for _, extra := range extras {
		b, err := fs.ReadFile(srcFS, extra.asset)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded %s: %w", extra.asset, err)
		}
		full := filepath.Join(dir, filepath.FromSlash(extra.path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", full, err)
		}
	}
	// Write preset-specific layout.yaml if specified.
	if preset == "rule-center" {
		layoutDir := filepath.Join(dir, "template")
		if err := os.MkdirAll(layoutDir, 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir %s: %w", layoutDir, err)
		}
		b, err := fs.ReadFile(srcFS, "kitex/layout-rulecenter.yaml")
		if err != nil {
			return fmt.Errorf("scaffold: read embedded kitex/layout-rulecenter.yaml: %w", err)
		}
		if err := os.WriteFile(filepath.Join(layoutDir, "layout.yaml"), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write layout.yaml: %w", err)
		}
	}
	return nil
}

var includeDirectiveRE = regexp.MustCompile(`^(\s*)#\s*\{\{include:\s*([A-Za-z0-9_/.-]+)\}\}\s*$`)

// expandIncludes replaces "# {{include: <asset>}}" directive lines in a hertz
// layout.yaml with the referenced shared fragment rendered as a layout entry.
// Fragments use the canonical kitex template format (path/body, {{.Module}}
// placeholder, 2-space body indent); hz consumes the expanded result, so
// directives never leave ncgo. Output is deterministic for golden tests.
func expandIncludes(layout []byte, srcFS fs.FS) ([]byte, error) {
	lines := strings.Split(string(layout), "\n")
	var out []string
	for _, line := range lines {
		m := includeDirectiveRE.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			continue
		}
		name := m[2]
		entry, err := renderSharedFragment(srcFS, name)
		if err != nil {
			return nil, err
		}
		out = append(out, entry...)
	}
	return []byte(strings.Join(out, "\n")), nil
}

func renderSharedFragment(srcFS fs.FS, name string) ([]string, error) {
	b, err := fs.ReadFile(srcFS, name+".yaml")
	if err != nil {
		return nil, fmt.Errorf("scaffold: read shared fragment %s: %w", name, err)
	}
	var frag struct {
		Path string `yaml:"path"`
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &frag); err != nil {
		return nil, fmt.Errorf("scaffold: parse shared fragment %s: %w", name, err)
	}
	body := strings.ReplaceAll(frag.Body, "{{.Module}}", "{{.GoModule}}")
	entry := []string{
		"  - path: " + frag.Path,
		`    delims: ["{{", "}}"]`,
		"    body: |-",
	}
	for _, bl := range strings.Split(body, "\n") {
		if bl == "" {
			entry = append(entry, "")
			continue
		}
		entry = append(entry, "      "+bl)
	}
	return entry, nil
}

// writeKitexGoMod pre-writes the project go.mod for a kitex scaffold so the
// generated module pins `go 1.26.5` and requires the go-tools modules at
// v0.1.0. The kitex tool only runs `go mod init` when go.mod is absent (which
// leaves the go-tools versions unpinned until `go mod tidy` resolves them);
// when go.mod already exists with a matching module path, kitex reuses it
// as-is and skips its own init. This mirrors the Hertz layout.yaml go.mod
// entry so the version is template-locked and reproducible. `go mod tidy`
// (run later) preserves these directives and only appends the kitex-runtime
// requires; go-middleware is added by tidy when WithDatabase imports it.
func writeKitexGoMod(dir, module string) error {
	body := fmt.Sprintf("module %s\n\ngo 1.26.5\n\nrequire (\n\tgithub.com/byx-darwin/go-tools/go-common v0.1.0\n\tgithub.com/byx-darwin/go-tools/go-framework v0.1.0\n)\n", module)
	full := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", full, err)
	}
	return nil
}

// writeHertzRootTemplates reads hertz-template files whose target path is at
// the project root (e.g. Makefile) and writes them directly to the scaffold
// directory with template variables rendered. This ensures make targets like
// sqlc are available before template.Apply() runs (NoGenerate / post-generate).
func writeHertzRootTemplates(dir string, opts Options, srcFS fs.FS) error {
	entries, err := fs.ReadDir(srcFS, "hertz/hertz-template")
	if err != nil {
		return nil // hertz-template doesn't exist yet
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		b, err := fs.ReadFile(srcFS, "hertz/hertz-template/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded hertz/hertz-template/%s: %w", name, err)
		}
		// Only write root-level templates (no subdirectory in path).
		// Parse the yaml to find the target path.
		content := string(b)
		targetPath, isRoot := parseHertzTemplatePath(content)
		if !isRoot {
			continue
		}
		// Render template variables.
		rendered := renderHertzTemplate(content, opts)
		full := filepath.Join(dir, filepath.FromSlash(targetPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", targetPath, err)
		}
	}
	return nil
}

// parseHertzTemplatePath extracts the target path from a hertz-template yaml body
// and reports whether it is a root-level file (no "/" in path).
func parseHertzTemplatePath(content string) (path string, isRoot bool) {
	// Look for "path: <value>" line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "path:") {
			p := strings.TrimSpace(strings.TrimPrefix(trimmed, "path:"))
			return p, !strings.Contains(p, "/")
		}
	}
	return "", false
}

// renderHertzTemplate applies template variable substitution for a hertz-template
// yaml body. It extracts the body content and renders {{.Module}} and {{.ServiceName}}.
func renderHertzTemplate(content string, opts Options) string {
	// Extract the body section from the yaml
	idx := strings.Index(content, "body:")
	if idx < 0 {
		return content
	}
	bodyStart := idx + len("body:")
	// Skip the "|-" or "|" or "|+" indicator
	rest := content[bodyStart:]
	if len(rest) > 0 && rest[0] == ' ' {
		rest = rest[1:]
	}
	if strings.HasPrefix(rest, "|-\n") {
		rest = rest[3:]
	} else if strings.HasPrefix(rest, "|+\n") {
		rest = rest[3:]
	} else if strings.HasPrefix(rest, "|\n") {
		rest = rest[2:]
	} else if rest[0] == '\n' {
		rest = rest[1:]
	}
	// Render template variables
	rendered := strings.ReplaceAll(rest, "{{.Module}}", opts.Module)
	rendered = strings.ReplaceAll(rendered, "{{.ServiceName | ToLower}}", strings.ToLower(opts.Name))
	rendered = strings.ReplaceAll(rendered, "{{ToLower .ServiceName}}", strings.ToLower(opts.Name))
	// Strip common indentation from yaml block scalar body
	rendered = dedentBody(rendered)
	return rendered
}

// copyHertzTemplateYAML copies every embedded hertz/hertz-template/*.yaml verbatim
// into <dir>/template/hertz-template/ so that template.Apply() can override
// hz-generated files with go-tools integrated versions.
func copyHertzTemplateYAML(dir string, srcFS fs.FS) error {
	hertzDir := filepath.Join(dir, "template", "hertz-template")
	if err := os.MkdirAll(hertzDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", hertzDir, err)
	}
	entries, err := fs.ReadDir(srcFS, "hertz/hertz-template")
	if err != nil {
		// hertz-template directory doesn't exist — nothing to copy
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		b, err := fs.ReadFile(srcFS, "hertz/hertz-template/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded hertz/hertz-template/%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(hertzDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
	return nil
}

// writeIDLPlaceholder drops the starter IDL files into the scaffold.
// Hertz follows the official api.proto + openapi annotation proto + service
// proto structure so `hz new` and Swagger generation can work out of the box;
// Kitex keeps its single service-named proto consumed by the kitex tool.
//
// The rule-center preset is special: its real proto lives in the
// ratelimit_proto.yaml kitex template, so instead of an empty placeholder we
// write the full preset proto at scaffold time. That way kitex parses the real
// IDL (kitex_gen/api/ratelimit/v1) on its first run instead of only after the
// user runs `make update`. Any IDL that already exists on disk is left alone.
func writeIDLPlaceholder(dir, idl string, opts Options) error {
	if defaultKind(opts.Kind) == manifest.KindHertz {
		if err := writeHertzProtoSupportFiles(dir); err != nil {
			return err
		}
	}
	full := filepath.Join(dir, filepath.FromSlash(idl))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(full), err)
	}
	if opts.Preset == "rule-center" && filepath.ToSlash(idl) == "idl/rule-center.proto" {
		body, err := ruleCenterIDLBody(assets.FS())
		if err != nil {
			return err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", idl, err)
		}
		return nil
	}
	// Never clobber an IDL that already exists on disk (e.g. one produced by a
	// preset template or a previous generate step).
	if _, err := os.Stat(full); err == nil {
		return nil
	}
	body := renderIDLPlaceholder(opts)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", full, err)
	}
	return nil
}

// ruleCenterIDLBody extracts the preset proto body from the embedded
// ratelimit_proto.yaml kitex template so the full IDL exists before kitex
// renders templates. The template body is static proto (no placeholders), so
// the rendered bytes match what kitex itself writes on the `update` target.
func ruleCenterIDLBody(srcFS fs.FS) ([]byte, error) {
	b, err := fs.ReadFile(srcFS, "kitex/kitex-template/ratelimit_proto.yaml")
	if err != nil {
		return nil, fmt.Errorf("scaffold: read embedded ratelimit_proto.yaml: %w", err)
	}
	var tpl struct {
		Body string `yaml:"body"`
	}
	if err := yaml.Unmarshal(b, &tpl); err != nil {
		return nil, fmt.Errorf("scaffold: parse ratelimit_proto.yaml: %w", err)
	}
	return []byte(tpl.Body), nil
}

func writeHertzProtoSupportFiles(dir string) error {
	if err := writeHertzAPIProto(dir); err != nil {
		return err
	}
	srcFS := assets.FS()
	for _, name := range []string{"annotations.proto", "openapi.proto"} {
		assetPath := filepath.ToSlash(filepath.Join("hertz", "openapi", name))
		body, err := fs.ReadFile(srcFS, assetPath)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded %s: %w", assetPath, err)
		}
		full := filepath.Join(dir, "idl", "openapi", name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", full, err)
		}
	}
	// validate.proto (PGV) ships in its own idl/validate/ subdir so the service
	// proto's `import "validate/validate.proto";` resolves under `-I idl` (hz)
	// and protolint's [root, root/idl] import roots.
	validateBody, err := fs.ReadFile(srcFS, filepath.ToSlash(filepath.Join("hertz", "validate", "validate.proto")))
	if err != nil {
		return fmt.Errorf("scaffold: read embedded hertz/validate/validate.proto: %w", err)
	}
	validatePath := filepath.Join(dir, "idl", "validate", "validate.proto")
	if err := os.MkdirAll(filepath.Dir(validatePath), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(validatePath), err)
	}
	if err := os.WriteFile(validatePath, validateBody, 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", validatePath, err)
	}
	return nil
}

func writeHertzAPIProto(dir string) error {
	full := filepath.Join(dir, "idl", "api.proto")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(hertzAPIProto), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", full, err)
	}
	return nil
}

func renderIDLPlaceholder(opts Options) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		base := kitexIDLBase(opts)
		service := exportName(opts.Name)
		return fmt.Sprintf(`syntax = "proto3";

package %s;

option go_package = "%s/kitex_gen/%s;%s";

service %s {
}
`, base, opts.Module, base, base, service)
	}
	service := exportName(opts.Name)
	return strings.Join([]string{
		`syntax = "proto3";`,
		``,
		`package app;`,
		``,
		fmt.Sprintf(`option go_package = %q;`, opts.Module+`/internal/pb;pb`),
		``,
		`import "api.proto";`,
		`import "openapi/annotations.proto";`,
		`import "validate/validate.proto";`,
		``,
		`option (openapi.document) = {`,
		`  info: {`,
		fmt.Sprintf(`    title: %q;`, service+` API`),
		`    version: "v1";`,
		fmt.Sprintf(`    description: %q;`, `Generated by ncgo for Hertz HTTP APIs.`),
		`  };`,
		`};`,
		``,
		`message PingReq {`,
		`  string name = 1 [`,
		`    (api.query) = "name",`,
		`    (openapi.parameter) = { required: true },`,
		`    (validate.rules) = { string: { min_len: 1, max_len: 64 } },`,
		`    (openapi.property) = {`,
		`      title: "Name";`,
		`      description: "Ping 请求中的 name 查询参数";`,
		`      type: "string";`,
		`      min_length: 1;`,
		`      max_length: 64;`,
		`    }`,
		`  ];`,
		`}`,
		``,
		`message PingResp {`,
		`  option (openapi.schema) = {`,
		`    title: "Ping response";`,
		`    description: "Ping 接口返回结果";`,
		`    required: ["message"];`,
		`  };`,
		``,
		`  string message = 1 [`,
		`    (openapi.property) = {`,
		`      title: "Response message";`,
		`      description: "服务返回的响应文本";`,
		`      type: "string";`,
		`      min_length: 1;`,
		`      max_length: 128;`,
		`    }`,
		`  ];`,
		`}`,
		``,
		fmt.Sprintf(`service %sService {`, service),
		`  rpc Ping(PingReq) returns (PingResp) {`,
		`    option (api.get) = "/ping";`,
		`    option (openapi.operation) = {`,
		`      summary: "Ping";`,
		`      description: "基础连通性测试接口";`,
		`    };`,
		`  }`,
		`}`,
		``,
	}, "\n")
}

func buildManifest(opts Options, idl string) *manifest.Manifest {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &manifest.Manifest{
		Ncgo: manifest.Meta{
			Version:       opts.NCGOVersion,
			AssetsVersion: opts.AssetsVersion,
		},
		Mode:   manifest.ModeMono,
		Module: opts.Module,
		Service: manifest.Service{
			Name:         opts.Name,
			Kind:         defaultKind(opts.Kind),
			WithDatabase: opts.WithDatabase,
			IDL:          idl,
		},
		GeneratedAt: now,
	}
}

// writeManifest delegates to internal/manifest.Save for the project-root
// .ncgo/manifest.yaml so the schema and atomic-write semantics stay in one
// place.
func writeManifest(dir string, opts Options, idl string) (*manifest.Manifest, error) {
	m := buildManifest(opts, idl)
	if err := manifest.Save(dir, m); err != nil {
		return nil, err
	}
	return m, nil
}

// nextSteps is the agent-facing handoff: the exact shell sequence to run
// when ncgo did not (or could not) call the generator itself. The hz/kitex
// invocation differs by Kind; everything before and after is shared.
//
// Important ordering rule: when the generated tree imports internal/db/gen
// (all Kitex scaffolds, plus Hertz scaffolds with WithDatabase enabled), we
// must run `make sqlc` before the first `go mod tidy`; otherwise Go treats the
// missing local package as an external module path and resolution fails.
func nextSteps(opts Options, idl string) []string {
	rel, _ := filepath.Rel(mustCwd(), opts.Dir)
	// When the user runs ncgo new inside the target directory itself,
	// filepath.Rel returns "."; show the directory basename instead so
	// the "cd" hint is meaningful.
	if rel == "." || rel == "" {
		rel = filepath.Base(opts.Dir)
	}
	steps := []string{
		fmt.Sprintf("cd %s", rel),
		fmt.Sprintf("go mod init %s", opts.Module),
		generatorCommand(opts, idl),
	}
	for _, kind := range opts.Infra {
		steps = append(steps, fmt.Sprintf("ncgo add infra %s --root .", kind))
	}
	if requiresSQLCBeforeTidy(opts) {
		steps = append(steps, "make sqlc")
	}
	steps = append(steps, "go mod tidy")
	if opts.WithDatabase {
		steps = append(steps, "make migrate-up")
	}
	steps = append(steps, "make dev")
	return steps
}

// generatorCommand returns the literal shell line a user can paste to
// invoke the appropriate code generator.
func generatorCommand(opts Options, idl string) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return fmt.Sprintf("kitex -module %s -template-dir template/kitex-template -type protobuf %s", opts.Module, idl)
	}
	return fmt.Sprintf("hz new --mod=%s --idl=%s -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml", opts.Module, idl)
}

// postGenerateNextSteps is what we print after hz/kitex already ran
// successfully. The generator has already created go.mod, so only tidy
// and runtime follow-ups remain.
//
// The same sqlc-before-tidy ordering from nextSteps still matters here:
// generation may have written code that already imports internal/db/gen.
func postGenerateNextSteps(opts Options) []string {
	rel, _ := filepath.Rel(mustCwd(), opts.Dir)
	// When the user runs ncgo new inside the target directory itself,
	// filepath.Rel returns "."; show the directory basename instead so
	// the "cd" hint is meaningful.
	if rel == "." || rel == "" {
		rel = filepath.Base(opts.Dir)
	}
	steps := []string{
		fmt.Sprintf("cd %s", rel),
	}
	if requiresSQLCBeforeTidy(opts) {
		steps = append(steps, "make sqlc")
	}
	steps = append(steps, "go mod tidy")
	if opts.WithDatabase {
		steps = append(steps, "make migrate-up")
	}
	steps = append(steps, "make dev")
	steps = append(steps, "ncgo ai sync --target all --root .")
	return steps
}

// requiresSQLCBeforeTidy reports whether the scaffolded code references
// internal/db/gen before the user has had a chance to run `go mod tidy`.
// Kitex always wires base/data + repository placeholders, while Hertz only
// needs sqlc first when the database scaffold is enabled.
func requiresSQLCBeforeTidy(opts Options) bool {
	return defaultKind(opts.Kind) == manifest.KindKitex || opts.WithDatabase
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// exportName converts "user-api" to "UserApi" for use as a proto service name.
func exportName(s string) string {
	out := make([]byte, 0, len(s))
	upper := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' || c == '_' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
		upper = false
	}
	return string(out)
}

// dedentBody strips the common leading whitespace prefix from all non-empty
// lines in a yaml block-scalar body, so the rendered output is correctly
// indented as Go source code.
func dedentBody(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent <= 0 {
		return s
	}
	for i, line := range lines {
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}
	return strings.Join(lines, "\n")
}
