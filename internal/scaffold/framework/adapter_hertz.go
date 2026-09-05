package framework

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(hertzAdapter{}) }

type hertzAdapter struct{}

func (hertzAdapter) Kind() string { return manifest.KindHertz }

func (hertzAdapter) OptionalAssetPath(infraKind string) (string, bool) {
	return "hertz/optional/" + infraKind + ".go", true
}

func (hertzAdapter) HertzConfigAssetPath(infraKind string) (string, bool) {
	return filepath.ToSlash(filepath.Join("hertz", "optional-config", infraKind+".yaml")), true
}

func (hertzAdapter) MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error) {
	if current == nil {
		return &ConfigWrite{Body: []byte(wrapHertzConfigSnippet(string(snippet), infraKind) + "\n"), Action: "create"}, nil
	}
	merged, changed, err := mergeHertzConfig(current, string(snippet), infraKind, hertzConfigKey, force)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, nil
	}
	return &ConfigWrite{Body: []byte(merged), Action: "update"}, nil
}

func (hertzAdapter) MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil // Kitex-only in current behavior.
}

// mergeHertzConfig/wrapHertzConfigSnippet/hertzConfigMarkers/
// replaceMarkedHertzConfigBlock are moved verbatim from
// internal/scaffold/infra/infra.go:409-465.
func mergeHertzConfig(current []byte, snippet, infraKind, hertzConfigKey string, force bool) (string, bool, error) {
	src := string(current)
	startMarker, endMarker := hertzConfigMarkers(infraKind)
	if strings.Contains(src, startMarker) || strings.Contains(src, endMarker) {
		if !strings.Contains(src, startMarker) || !strings.Contains(src, endMarker) {
			return "", false, fmt.Errorf("infra: malformed config markers for %q in %s", infraKind, filepath.FromSlash("conf/dev/conf.yaml"))
		}
		if !force {
			return src, false, nil
		}
		return replaceMarkedHertzConfigBlock(src, wrapHertzConfigSnippet(snippet, infraKind), startMarker, endMarker)
	}
	if hasTopLevelConfigKey(src, hertzConfigKey) {
		return src, false, nil
	}
	block := wrapHertzConfigSnippet(snippet, infraKind)
	trimmed := strings.TrimRight(src, "\n")
	if trimmed == "" {
		return block + "\n", true, nil
	}
	return trimmed + "\n\n" + block + "\n", true, nil
}

func wrapHertzConfigSnippet(snippet, infraKind string) string {
	startMarker, endMarker := hertzConfigMarkers(infraKind)
	return startMarker + "\n" + strings.TrimRight(snippet, "\n") + "\n" + endMarker
}

func hertzConfigMarkers(infraKind string) (string, string) {
	return "# ncgo:add-infra:start " + infraKind, "# ncgo:add-infra:end " + infraKind
}

func replaceMarkedHertzConfigBlock(src, block, startMarker, endMarker string) (string, bool, error) {
	start := strings.Index(src, startMarker)
	if start < 0 {
		return src, false, nil
	}
	end := strings.Index(src[start:], endMarker)
	if end < 0 {
		return "", false, fmt.Errorf("infra: malformed config markers: missing %q", endMarker)
	}
	end += start
	lineEnd := end + len(endMarker)
	if lineEnd < len(src) && src[lineEnd] == '\r' {
		lineEnd++
	}
	if lineEnd < len(src) && src[lineEnd] == '\n' {
		lineEnd++
	}
	out := src[:start] + block
	if lineEnd < len(src) {
		out += src[lineEnd:]
	} else {
		out += "\n"
	}
	return out, true, nil
}

func (hertzAdapter) DockerConfigBlocks(m *manifest.Manifest) []string {
	blocks := []string{
		"config_center:\n  nacos:\n    server_addr: nacos:8848\n  polaris:\n    addresses:\n      - polaris:8093",
		"release:\n  discovery:\n    nacos:\n      server_addr: nacos:8848\n    polaris:\n      addresses:\n        - polaris:8091\n  rules:\n    nacos:\n      server_addr: nacos:8848\n    polaris:\n      addresses:\n        - polaris:8093",
	}
	if m.Service.WithDatabase {
		blocks = append(blocks, fmt.Sprintf("database:\n  enabled: true\n  dsn: %q", postgresDSN(m.Service.Name)))
	}
	if manifestHasInfra(m, "redis") {
		blocks = append(blocks, "redis:\n  addrs:\n    - redis:6379")
	}
	if manifestHasInfra(m, "kafka") {
		blocks = append(blocks, "kafka:\n  producer:\n    brokers:\n      - kafka:9092\n  consumer:\n    brokers:\n      - kafka:9092")
	}
	if manifestHasInfra(m, "es") {
		blocks = append(blocks, "es:\n  addresses:\n    - http://elasticsearch:9200")
	}
	if manifestHasInfra(m, "clickhouse") {
		blocks = append(blocks, "clickhouse:\n  addr:\n    - clickhouse:9000")
	}
	if m.Service.WithDatabase && manifestHasInfra(m, "redis") {
		blocks = append(blocks, fmt.Sprintf(`rate_limit:
  enabled: true
  source:
    type: database
    cache_ttl_seconds: 60s
    fallback_on_error: true
  database:
    query_timeout_milliseconds: 200ms
  rule_center:
    address: "${RULE_CENTER_ADDR:}"
    query_timeout_milliseconds: 200ms
  backend: redis
  fail_open: false
  key_prefix: "%s:rate_limit"
  pre_auth:
    enabled: true
    default_rule:
      enabled: true
      key_by:
        - ip
      strategy: fixed_window
      window_seconds: 60s
      max_requests: 100
      client_ttl_seconds: 300s
    rules: []`, m.Service.Name))
	}
	return blocks
}

func (hertzAdapter) ContainerPort() int { return 8080 }

func (hertzAdapter) ComposeFeatures(withDatabase bool) ComposeFeatureFlags {
	return ComposeFeatureFlags{Nacos: true, Polaris: true, Vegeta: withDatabase}
}

func (hertzAdapter) IDLPath(opts GeneratorOptions) string {
	return filepath.ToSlash(filepath.Join("idl", "app", opts.Name+".proto"))
}

func (hertzAdapter) IDLNameToken(opts GeneratorOptions) string {
	return strings.ToLower(opts.Name)
}

func (hertzAdapter) GeneratorCommand(opts GeneratorOptions, idl string) string {
	return fmt.Sprintf("hz new --mod=%s --idl=%s -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml", opts.Module, idl)
}

func (hertzAdapter) RenderIDLPlaceholder(opts GeneratorOptions) string {
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
		`    (api.vd) = "len($) > 0 && len($) < 65",`,
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

func (hertzAdapter) WriteIDLSupportFiles(dir string) error {
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
		if err := mkdirAllFor(full); err != nil {
			return err
		}
		if err := writeFileFor(full, body); err != nil {
			return err
		}
	}
	validateBody, err := fs.ReadFile(srcFS, filepath.ToSlash(filepath.Join("hertz", "validate", "validate.proto")))
	if err != nil {
		return fmt.Errorf("scaffold: read embedded hertz/validate/validate.proto: %w", err)
	}
	validatePath := filepath.Join(dir, "idl", "validate", "validate.proto")
	if err := mkdirAllFor(validatePath); err != nil {
		return err
	}
	return writeFileFor(validatePath, validateBody)
}

func writeHertzAPIProto(dir string) error {
	full := filepath.Join(dir, "idl", "api.proto")
	if err := mkdirAllFor(full); err != nil {
		return err
	}
	return writeFileFor(full, []byte(hertzAPIProto))
}

func (hertzAdapter) RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error) {
	return exec.HZ(ctx, r, dir, hzArgs(opts, idl)...)
}

func hzArgs(opts GeneratorOptions, idl string) []string {
	return []string{
		"new",
		"--mod=" + opts.Module,
		"--idl=" + idl,
		"-I", "idl",
		"--handler_dir=internal/handler",
		"--model_dir=internal/pb",
		"--router_dir=internal/router",
		"--customize_layout=template/layout.yaml",
		"--customize_layout_data_path=template/data.json",
		"--customize_package=template/package.yaml",
	}
}

func (hertzAdapter) RequiresSQLCBeforeTidy(withDatabase bool) bool { return withDatabase }

func (hertzAdapter) ServerFilePath() string {
	return "internal/base/server/server.go"
}

// manifestHasInfra/postgresDSN/mkdirAllFor/writeFileFor are small shared
// helpers used by both this file and adapter_kitex.go's DockerConfigBlocks.
func manifestHasInfra(m *manifest.Manifest, kind string) bool {
	for _, k := range m.Infra {
		if k == kind {
			return true
		}
	}
	return false
}

func postgresDSN(serviceName string) string {
	return fmt.Sprintf("postgres://postgres:postgres@postgres:5432/%s?sslmode=disable", serviceName)
}

func mkdirAllFor(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", filepath.Dir(path), err)
	}
	return nil
}

func writeFileFor(path string, body []byte) error {
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", path, err)
	}
	return nil
}
