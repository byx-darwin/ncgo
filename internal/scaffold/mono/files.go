package mono

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/manifest"
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
		return writeKitexTemplate(dir)
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
	return nil
}

// writeKitexTemplate copies every embedded kitex-template/*.yaml verbatim
// into <dir>/template/kitex-template/ so that both `kitex` (during
// scaffold) and the generated Makefile's `update` target can consume
// them at the same path.
func writeKitexTemplate(dir string) error {
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
		b, err := fs.ReadFile(srcFS, "kitex/kitex-template/"+name)
		if err != nil {
			return fmt.Errorf("scaffold: read embedded kitex/kitex-template/%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(tplDir, name), b, 0o644); err != nil {
			return fmt.Errorf("scaffold: write %s: %w", name, err)
		}
	}
	for _, extra := range []struct {
		asset string
		path  string
	}{
		{asset: "kitex/sqlc.yaml", path: "internal/db/sqlc.yaml"},
		{asset: "kitex/query/health.sql", path: "internal/db/query/health.sql"},
		{asset: "kitex/schema/000001_placeholder.sql", path: "internal/db/schema/000001_placeholder.sql"},
	} {
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
	return nil
}

// writeIDLPlaceholder drops the starter IDL files into the scaffold.
// Hertz follows the official api.proto + openapi annotation proto + service
// proto structure so `hz new` and Swagger generation can work out of the box;
// Kitex keeps its single service-named proto consumed by the kitex tool.
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
	body := renderIDLPlaceholder(opts)
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", full, err)
	}
	return nil
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
		`    (api.body) = "message",`,
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
	if rel == "" {
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
	if rel == "" {
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
