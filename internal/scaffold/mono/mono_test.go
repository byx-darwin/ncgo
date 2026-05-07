package mono

import (
	"context"
	"encoding/json"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

type fakeRunner struct {
	calls []exec.Cmd
}

func (f *fakeRunner) Run(_ context.Context, c exec.Cmd) (exec.Result, error) {
	f.calls = append(f.calls, c)
	return exec.Result{}, nil
}

func baseOpts(t *testing.T) Options {
	t.Helper()
	return Options{
		Name:          "demo",
		Module:        "github.com/x/demo",
		Dir:           filepath.Join(t.TempDir(), "demo"),
		AssetsVersion: "test-assets",
		NCGOVersion:   "0.0.0-test",
		Now:           time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		NoGenerate:    true,
	}
}

func TestGenerateNoGenerateProducesGoldenTree(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.RanGenerate {
		t.Errorf("RanGenerate = true, want false (NoGenerate set)")
	}
	got := walk(t, res.Dir)
	want := []string{
		".ncgo/manifest.yaml",
		".pre-commit-config.yaml",
		"idl/api.proto",
		"idl/app/demo.proto",
		"idl/openapi/annotations.proto",
		"idl/openapi/openapi.proto",
		"scripts/run-go-module-checks.sh",
		"template/data.json",
		"template/layout.yaml",
		"template/package.yaml",
	}
	if !equal(got, want) {
		t.Errorf("tree mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestGenerateWritesHertzStarterIDLs(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	apiBody, err := os.ReadFile(filepath.Join(res.Dir, "idl", "api.proto"))
	if err != nil {
		t.Fatalf("read api.proto: %v", err)
	}
	for _, want := range []string{
		`syntax = "proto2";`,
		`package api;`,
		`import "google/protobuf/descriptor.proto";`,
		`optional string file_name = 50110;`,
		`optional string base_domain = 50402;`,
		`optional string get = 50201;`,
	} {
		if !strings.Contains(string(apiBody), want) {
			t.Errorf("api.proto missing %q\n---\n%s", want, string(apiBody))
		}
	}
	annotationsBody, err := os.ReadFile(filepath.Join(res.Dir, "idl", "openapi", "annotations.proto"))
	if err != nil {
		t.Fatalf("read annotations.proto: %v", err)
	}
	for _, want := range []string{
		`package openapi;`,
		`import "openapi/openapi.proto";`,
		`openapi.v3.Document document = 1143;`,
		`openapi.v3.Operation operation = 1143;`,
	} {
		if !strings.Contains(string(annotationsBody), want) {
			t.Errorf("annotations.proto missing %q\n---\n%s", want, string(annotationsBody))
		}
	}
	serviceBody, err := os.ReadFile(filepath.Join(res.Dir, "idl", "app", "demo.proto"))
	if err != nil {
		t.Fatalf("read demo.proto: %v", err)
	}
	for _, want := range []string{
		`package app;`,
		`option go_package = "github.com/x/demo/internal/pb;pb";`,
		`import "api.proto";`,
		`import "openapi/annotations.proto";`,
		`option (openapi.document) = {`,
		`message PingReq {`,
		`(openapi.parameter) = { required: true },`,
		`message PingResp {`,
		`(api.body) = "message",`,
		`service DemoService {`,
		`rpc Ping(PingReq) returns (PingResp) {`,
		`option (api.get) = "/ping";`,
		`option (openapi.operation) = {`,
	} {
		if !strings.Contains(string(serviceBody), want) {
			t.Errorf("demo.proto missing %q\n---\n%s", want, string(serviceBody))
		}
	}
}

func TestGenerateHertzNextStepsIncludeIDLImportPath(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	joined := strings.Join(res.NextSteps, "\n")
	want := "hz new --mod=github.com/x/demo --idl=idl/app/demo.proto -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml"
	if !strings.Contains(joined, want) {
		t.Fatalf("next steps missing updated hz command %q\n---\n%s", want, joined)
	}
}

func TestGenerateHertzTemplateIncludesSafeOptionalWiringAnchors(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("read hertz layout: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"Optional structured logging wiring",
		"// ncgo:wire:logging:init",
		"// ncgo:wire:logging:server-middleware",
		"import \"{{.GoModule}}/internal/base/logging\"",
		"h.Use(logging.HertzRecovery())",
		"h.Use(logging.HertzRequestID())",
		"h.Use(logging.HertzAccessLog())",
		"Optional release canary wiring",
		"// ncgo:wire:canary:server-traffic",
		"import \"{{.GoModule}}/internal/base/release\"",
		"h.Use(release.HertzTraffic())",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz layout missing optional wiring anchor %q", want)
		}
	}
}

func TestGenerateWritesRepositoryHookFiles(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	config, err := os.ReadFile(filepath.Join(res.Dir, ".pre-commit-config.yaml"))
	if err != nil {
		t.Fatalf("read pre-commit config: %v", err)
	}
	for _, want := range []string{"default_install_hook_types", "./scripts/run-go-module-checks.sh vet", "go-build-all-modules"} {
		if !strings.Contains(string(config), want) {
			t.Errorf("pre-commit config missing %q", want)
		}
	}
	script, err := os.ReadFile(filepath.Join(res.Dir, "scripts", "run-go-module-checks.sh"))
	if err != nil {
		t.Fatalf("read pre-push helper script: %v", err)
	}
	for _, want := range []string{"find . -name go.mod", "usage: $0 <vet|test|build>", "go test ./... -count=1"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("pre-push helper missing %q", want)
		}
	}
}

func TestGenerateHertzTemplateRendersMakefileRecipes(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("read hertz layout: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"build: ; @echo \"Building $(APP_NAME)...\"",
		"dev: ; @which air > /dev/null 2>&1 && air || go run .",
		"i18n-sync: ; @echo \"Synchronizing i18n locales...\"; go run ./tools/i18n/sync -root . -locales internal/pkg/i18n/locales -status internal/pkg/i18n/.meta/status.json -source zh-CN",
		"i18n-report: ; @echo \"Writing i18n report...\"; go run ./tools/i18n/report -locales internal/pkg/i18n/locales -status internal/pkg/i18n/.meta/status.json -glossary internal/pkg/i18n/glossary.json -source zh-CN",
		"i18n-check: ; @echo \"Checking i18n locales...\"; go run ./tools/i18n/check -locales internal/pkg/i18n/locales -status internal/pkg/i18n/.meta/status.json -glossary internal/pkg/i18n/glossary.json -source zh-CN -mode dev",
		"i18n-check-release: ; @echo \"Checking i18n locales (release mode)...\"; go run ./tools/i18n/check -locales internal/pkg/i18n/locales -status internal/pkg/i18n/.meta/status.json -glossary internal/pkg/i18n/glossary.json -source zh-CN -mode release",
		"i18n: ; @echo \"Generating i18n catalog...\"; go run ./tools/i18n/gen -in internal/pkg/i18n/locales -out internal/pkg/i18n/catalog_gen.go",
		"update: ; @echo \"Generating Hertz code from IDL...\"; hz update --idl=idl/app/{{.ServiceName}}.proto -I idl --handler_dir=internal/handler --model_dir=internal/pb --customize_package=template/package.yaml; echo \"Code generation complete\"",
		"swagger: ; @command -v protoc >/dev/null 2>&1",
		"protoc-gen-http-swagger is required for make swagger",
		"go install github.com/hertz-contrib/swagger-generate/protoc-gen-http-swagger@latest",
		"Swagger generation complete → internal/docs/swagger (rebuild/restart to embed updated spec)",
		"generate: i18n-sync i18n-check i18n update swagger sqlc",
		"test: ; @go test -race -count=1 ./...",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz Makefile recipe missing %q", want)
		}
	}
}

func TestGenerateHertzTemplateEmbedsSwaggerSpec(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("read hertz layout: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"path: internal/docs/docs.go",
		"//go:embed swagger/openapi.yaml",
		"path: internal/docs/swagger/openapi.yaml",
		"docs.OpenAPIYAML()",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz swagger embed missing %q", want)
		}
	}
}

func TestGenerateHertzTemplateIncludesI18N(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("read hertz layout: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"path: internal/pkg/i18n/i18n.go",
		"HeaderAcceptLanguage",
		"Accept-Language",
		"Content-Language",
		"RegisterLanguage(language string, aliases ...string) string",
		"TraditionalChinese    = \"zh-TW\"",
		"Japanese              = \"ja-JP\"",
		"Korean                = \"ko-KR\"",
		"French                = \"fr-FR\"",
		"German                = \"de-DE\"",
		"Spanish               = \"es-ES\"",
		"localizePayload(c, payload)",
		"接口不存在",
		"介面不存在",
		"インターフェースが存在しません",
		"인터페이스가 존재하지 않습니다",
		"interface introuvable",
		"Schnittstelle nicht gefunden",
		"interfaz no encontrada",
		"path: internal/pkg/i18n/locales/de-DE.json",
		"path: internal/pkg/i18n/locales/es-ES.json",
		"path: internal/pkg/i18n/locales/fr-FR.json",
		"path: internal/pkg/i18n/locales/it-IT.json",
		"path: internal/pkg/i18n/locales/ja-JP.json",
		"path: internal/pkg/i18n/locales/ko-KR.json",
		"path: internal/pkg/i18n/locales/zh-CN.json",
		"path: internal/pkg/i18n/locales/zh-TW.json",
		"path: internal/pkg/i18n/.meta/status.json",
		"path: internal/pkg/i18n/glossary.json",
		"path: tools/i18n/util/i18nutil.go",
		"path: tools/i18n/util/i18nutil_test.go",
		"path: tools/i18n/sync/main.go",
		"path: tools/i18n/report/main.go",
		"path: tools/i18n/check/main.go",
		"path: tools/i18n/gen/main.go",
		"PlaceholderPrefix = \"__TODO__: \"",
		"TestScanMessageKeysIncludesPublicStringLiterals",
		"TestBuildReportIncludesGlossaryHintsAndSummary",
		"source_locale",
		"Glossary hints",
		"i18n Report",
		"TestRegisterLanguageSupportsDynamicLanguage",
		"interfaccia non trovata",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz i18n template missing %q", want)
		}
	}
}

func TestGenerateHertzTemplateIncludesChineseConfigComments(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "layout.yaml"))
	if err != nil {
		t.Fatalf("read hertz layout: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"# 本地开发环境配置（默认由 GO_ENV=dev 加载）",
		"# env 会决定默认读取 conf/<env>/conf.yaml；常见值为 dev/test/staging/prod",
		"# 以下超时时间单位均为秒",
		"# 运维建议：pool_size 需结合单实例并发、Redis 实例连接上限和副本数综合评估",
		"# Swagger / OpenAPI 文档配置",
		"# 是否启用 Swagger UI；建议仅在开发/测试环境开启",
		"# 当 key_by 包含 ak / ak_user_uuid 等维度时，从该请求头读取 app key",
		"# 开发环境静态密钥；生产环境建议改为配置中心或密钥管理系统",
		"# 运维建议：不要把真实生产密钥直接写入仓库或镜像",
		"# 携带 token 的请求头名；中间件通常支持 Bearer 前缀",
		"# token 签发者；校验时需与 token 中 iss 对齐",
		"# 运维建议：有效期越长，泄漏后的风险窗口越大；需结合登录态策略权衡",
		"# token 有效期，单位秒",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz config comments missing %q", want)
		}
	}
}

func TestGenerateRendersDataJSON(t *testing.T) {
	opts := baseOpts(t)
	opts.WithDatabase = true
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, "template", "data.json"))
	if err != nil {
		t.Fatalf("read data.json: %v", err)
	}
	var parsed map[string]map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("parse data.json: %v", err)
	}
	star := parsed["*"]
	if star["GoModule"] != "github.com/x/demo" {
		t.Errorf("GoModule = %v, want github.com/x/demo", star["GoModule"])
	}
	if star["ServiceName"] != "demo" {
		t.Errorf("ServiceName = %v", star["ServiceName"])
	}
	if star["WithDatabase"] != true {
		t.Errorf("WithDatabase = %v, want true", star["WithDatabase"])
	}
}

func TestGenerateWritesManifest(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, ".ncgo", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		"mode: mono",
		"module: github.com/x/demo",
		"name: demo",
		"kind: hertz",
		"idl: idl/app/demo.proto",
		"version: 0.0.0-test",
		"assets_version: test-assets",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("manifest missing %q\n---\n%s", want, s)
		}
	}
}

func TestGenerateInvokesHZViaRunner(t *testing.T) {
	opts := baseOpts(t)
	opts.NoGenerate = false
	r := &fakeRunner{}
	opts.Runner = r
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Errorf("RanGenerate = false, want true")
	}
	if strings.Contains(strings.Join(res.NextSteps, "\n"), "go mod init") {
		t.Errorf("post-generate next steps must not include go mod init: %v", res.NextSteps)
	}
	if len(r.calls) != 1 || r.calls[0].Name != "hz" {
		t.Fatalf("expected one hz call, got %+v", r.calls)
	}
	if r.calls[0].Dir != res.Dir {
		t.Errorf("hz call Dir = %q, want %q", r.calls[0].Dir, res.Dir)
	}
	args := strings.Join(r.calls[0].Args, " ")
	for _, want := range []string{
		"new", "--mod=github.com/x/demo", "--idl=idl/app/demo.proto", "-I idl",
		"--customize_layout=template/layout.yaml",
		"--customize_layout_data_path=template/data.json",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("hz args missing %q in %q", want, args)
		}
	}
}

// TestGenerateHertzCompiles invokes hz with the real runner, tidies the module,
// builds the generated project, and runs the i18n tests to verify the built-in
// language catalog compiles and behaves correctly.
func TestGenerateHertzCompiles(t *testing.T) {
	if _, err := exec.LookPath("hz"); err != nil {
		t.Skip("hz not found on PATH")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not found on PATH")
	}

	opts := baseOpts(t)
	opts.NoGenerate = false // use the real exec.Default runner

	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Fatal("expected RanGenerate = true")
	}

	// Synchronize locale keys and initialize translation status metadata.
	cmd := osexec.CommandContext(context.Background(), "make", "i18n-sync")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make i18n-sync in %s: %v\n%s", res.Dir, err, out)
	}
	statusBody, err := os.ReadFile(filepath.Join(res.Dir, "internal", "pkg", "i18n", ".meta", "status.json"))
	if err != nil {
		t.Fatalf("read generated status: %v", err)
	}
	statusText := string(statusBody)
	for _, want := range []string{"\"source_locale\": \"zh-CN\"", "\"it-IT\"", "\"draft\"", "\"reviewed\""} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("generated i18n status missing %q:\n%s", want, statusText)
		}
	}

	// Emit machine-readable and human-readable report files for Agent workflows.
	cmd = osexec.CommandContext(context.Background(), "make", "i18n-report")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make i18n-report in %s: %v\n%s", res.Dir, err, out)
	}
	reportJSON, err := os.ReadFile(filepath.Join(res.Dir, "internal", "pkg", "i18n", ".meta", "report.json"))
	if err != nil {
		t.Fatalf("read generated report json: %v", err)
	}
	for _, want := range []string{"summary", "missing_translations", "glossary_hints", "it-IT", "internal_error"} {
		if !strings.Contains(string(reportJSON), want) {
			t.Fatalf("generated i18n report missing %q:\n%s", want, reportJSON)
		}
	}

	// Development mode check allows draft placeholders after sync.
	cmd = osexec.CommandContext(context.Background(), "make", "i18n-check")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make i18n-check in %s: %v\n%s", res.Dir, err, out)
	}

	// Release mode should still fail while draft placeholder translations exist.
	cmd = osexec.CommandContext(context.Background(), "make", "i18n-check-release")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("make i18n-check-release in %s unexpectedly succeeded\n%s", res.Dir, out)
	}

	// Generate dynamic i18n catalog from locale JSON via the generated Makefile.
	cmd = osexec.CommandContext(context.Background(), "make", "i18n")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make i18n in %s: %v\n%s", res.Dir, err, out)
	}
	generatedCatalog, err := os.ReadFile(filepath.Join(res.Dir, "internal", "pkg", "i18n", "catalog_gen.go"))
	if err != nil {
		t.Fatalf("read generated catalog: %v", err)
	}
	s := string(generatedCatalog)
	for _, want := range []string{
		`RegisterLanguage("zh-CN"`,
		`RegisterLanguage("zh-TW"`,
		`RegisterLanguage("ja-JP"`,
		`RegisterLanguage("ko-KR"`,
		`RegisterLanguage("fr-FR"`,
		`RegisterLanguage("de-DE"`,
		`RegisterLanguage("es-ES"`,
		`RegisterLanguage("it-IT", "it")`,
		"interfaccia non trovata",
		"接口不存在",
		"介面不存在",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("generated i18n catalog missing %q:\n%s", want, s)
		}
	}

	// go mod tidy resolves and downloads the generated project's dependencies.
	cmd = osexec.CommandContext(context.Background(), "go", "mod", "tidy")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy in %s: %v\n%s", res.Dir, err, out)
	}

	// Build the service binary to ensure all packages compile.
	cmd = osexec.CommandContext(context.Background(), "go", "build", ".")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build . in %s: %v\n%s", res.Dir, err, out)
	}

	// Run the generated project's own i18n tests (built-in language assertions).
	cmd = osexec.CommandContext(context.Background(), "go", "test", "-race", "-count=1", "./internal/pkg/i18n/...")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./internal/pkg/i18n/... in %s: %v\n%s", res.Dir, err, out)
	}

	cmd = osexec.CommandContext(context.Background(), "go", "test", "-race", "-count=1", "./tools/...")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./tools/... in %s: %v\n%s", res.Dir, err, out)
	}
}

func TestGenerateRejectsNonEmptyDir(t *testing.T) {
	opts := baseOpts(t)
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(opts.Dir, "stray"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := Generate(context.Background(), opts); err == nil {
		t.Fatalf("expected error for non-empty dir")
	}
}

func TestGenerateKitexNoGenerateProducesTree(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := walk(t, res.Dir)
	for _, want := range []string{
		".pre-commit-config.yaml",
		".ncgo/manifest.yaml",
		"idl/demo.proto",
		"scripts/run-go-module-checks.sh",
		"template/kitex-template/main.yaml",
		"template/kitex-template/server.yaml",
		"template/kitex-template/handler.yaml",
		"template/kitex-template/makefile.yaml",
	} {
		if !contains(got, want) {
			t.Errorf("kitex tree missing %q\n got: %v", want, got)
		}
	}
	for _, unwanted := range []string{
		"template/data.json",
		"template/layout.yaml",
		"template/package.yaml",
		"idl/api.proto",
		"idl/app/demo.proto",
		"idl/openapi/annotations.proto",
		"idl/openapi/openapi.proto",
	} {
		if contains(got, unwanted) {
			t.Errorf("kitex tree must not include hertz file %q", unwanted)
		}
	}
}

func TestGenerateKitexTemplatesIncludeSafeOptionalWiringAnchors(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	serverBody, err := os.ReadFile(filepath.Join(res.Dir, "template", "kitex-template", "server.yaml"))
	if err != nil {
		t.Fatalf("read kitex server template: %v", err)
	}
	clientBody, err := os.ReadFile(filepath.Join(res.Dir, "template", "kitex-template", "client.yaml"))
	if err != nil {
		t.Fatalf("read kitex client template: %v", err)
	}
	serverTemplate := string(serverBody)
	for _, want := range []string{
		"Optional structured logging wiring",
		"// ncgo:wire:logging:init",
		"// ncgo:wire:logging:server-middleware",
		"import \"{{.Module}}/internal/base/logging\"",
		"logging.KitexRequestID()",
		"logging.KitexAccessLog()",
		"logging.KitexRecovery()",
		"Optional release canary wiring",
		"// ncgo:wire:canary:server-traffic",
		"import \"{{.Module}}/internal/base/release\"",
		"release.KitexTraffic()",
	} {
		if !strings.Contains(serverTemplate, want) {
			t.Errorf("kitex server template missing optional wiring anchor %q", want)
		}
	}
	clientTemplate := string(clientBody)
	for _, want := range []string{
		"Optional client-side structured RPC logs",
		"// ncgo:wire:kitex-client:middleware",
		"options = append(options, kitexclient.WithMiddleware(endpoint.Chain(",
		"logging.KitexRequestID()",
		"logging.KitexAccessLog()",
		"options = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))",
		"release.NewKitexCanaryLoadBalancer(cfg.ServiceName, ruleProvider, nil)",
	} {
		if !strings.Contains(clientTemplate, want) {
			t.Errorf("kitex client template missing optional wiring anchor %q", want)
		}
	}
}

func TestGenerateKitexTemplateIncludesChineseConfigComments(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "template", "kitex-template", "conf_dev.yaml"))
	if err != nil {
		t.Fatalf("read kitex conf_dev template: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"# 本地开发环境配置（默认由 GO_ENV=dev 加载）",
		"# env 会决定默认读取 conf/<env>/conf.yaml；常见值为 dev/test/staging/prod",
		"# 连接读写超时，单位秒",
		"# 运维建议：应覆盖正常请求耗时，但也不要大到掩盖下游故障",
		"# 从哪个请求头读取调用方服务名",
		"# allowed_callers 中填写允许访问当前服务的上游服务名",
		"# 运维建议：开启 enabled 且 allow_missing=false 时，这里必须显式配置",
		"# PostgreSQL DSN；例如 postgres://user:pass@host:5432/dbname?sslmode=disable",
		"# 连接池最大连接数；需结合数据库实例上限和服务副本数评估",
		"# 运维建议：略小于数据库或代理层连接回收时间，避免同时批量失效",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("kitex config comments missing %q", want)
		}
	}
}

func TestGenerateKitexWritesManifest(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, ".ncgo", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	s := string(b)
	for _, want := range []string{
		"kind: kitex",
		"idl: idl/demo.proto",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("kitex manifest missing %q\n---\n%s", want, s)
		}
	}
}

func TestGenerateKitexInvokesKitexViaRunner(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	opts.NoGenerate = false
	r := &fakeRunner{}
	opts.Runner = r
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(strings.Join(res.NextSteps, "\n"), "go mod init") {
		t.Errorf("post-generate next steps must not include go mod init: %v", res.NextSteps)
	}
	if len(r.calls) != 1 || r.calls[0].Name != "kitex" {
		t.Fatalf("expected one kitex call, got %+v", r.calls)
	}
	args := strings.Join(r.calls[0].Args, " ")
	for _, want := range []string{
		"-module github.com/x/demo",
		"-template-dir template/kitex-template",
		"-type protobuf",
		"idl/demo.proto",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("kitex args missing %q in %q", want, args)
		}
	}
}

func TestGenerateKitexNormalizesHyphenatedServiceName(t *testing.T) {
	opts := baseOpts(t)
	opts.Name = "user-api"
	opts.Module = "github.com/x/user-api"
	opts.Dir = filepath.Join(t.TempDir(), "user-api")
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(res.Dir, "idl", "userapi.proto"))
	if err != nil {
		t.Fatalf("read normalized kitex IDL: %v", err)
	}
	for _, want := range []string{"package userapi;", "kitex_gen/userapi;userapi", "service UserApi"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("kitex IDL missing %q\n---\n%s", want, string(body))
		}
	}
}

func TestValidateRejectsBadInputs(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Options)
	}{
		{"bad-name", func(o *Options) { o.Name = "Bad_Name" }},
		{"empty-module", func(o *Options) { o.Module = "" }},
		{"flat-module", func(o *Options) { o.Module = "demo" }},
		{"empty-version", func(o *Options) { o.NCGOVersion = "" }},
		{"bad-kind", func(o *Options) { o.Kind = "grpc" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := baseOpts(t)
			tc.mut(&o)
			if _, err := Generate(context.Background(), o); err == nil {
				t.Errorf("expected error")
			}
		})
	}
}

func walk(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(xs []string, want string) bool {
	return slices.Contains(xs, want)
}
