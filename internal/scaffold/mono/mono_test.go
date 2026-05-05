package mono

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
		"idl/app/demo.proto",
		"template/data.json",
		"template/layout.yaml",
		"template/package.yaml",
	}
	if !equal(got, want) {
		t.Errorf("tree mismatch\n got: %v\nwant: %v", got, want)
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
		"update: ; @echo \"Generating Hertz code from IDL...\"; hz update --idl=idl/app/{{.ServiceName}}.proto -I idl --handler_dir=internal/handler --model_dir=internal/pb --customize_package=template/package.yaml; echo \"Code generation complete\"",
		"test: ; @go test -race -count=1 ./...",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz Makefile recipe missing %q", want)
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
		"new", "--mod=github.com/x/demo", "--idl=idl/app/demo.proto",
		"--customize_layout=template/layout.yaml",
		"--customize_layout_data_path=template/data.json",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("hz args missing %q in %q", want, args)
		}
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
		".ncgo/manifest.yaml",
		"idl/demo.proto",
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
		"idl/app/demo.proto",
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
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
