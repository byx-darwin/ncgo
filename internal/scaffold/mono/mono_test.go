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
		".dockerignore",
		".ncgo/manifest.yaml",
		".pre-commit-config.yaml",
		"Dockerfile",
		"compose.yaml",
		"conf/docker/conf.yaml",
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

func TestGenerateWritesHertzContainerFiles(t *testing.T) {
	opts := baseOpts(t)
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	dockerfile, err := os.ReadFile(filepath.Join(res.Dir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	for _, want := range []string{"EXPOSE 8080", "ENTRYPOINT [\"./app\"]", "ENV GO_ENV=docker"} {
		if !strings.Contains(string(dockerfile), want) {
			t.Fatalf("Dockerfile missing %q\n---\n%s", want, dockerfile)
		}
	}
	composeBody, err := os.ReadFile(filepath.Join(res.Dir, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	for _, want := range []string{"name: demo", "8080:8080", "dockerfile: Dockerfile", "config-center-nacos", "config-center-polaris", "GO_ENV: docker"} {
		if !strings.Contains(string(composeBody), want) {
			t.Fatalf("compose.yaml missing %q\n---\n%s", want, composeBody)
		}
	}
	dockerConf, err := os.ReadFile(filepath.Join(res.Dir, "conf", "docker", "conf.yaml"))
	if err != nil {
		t.Fatalf("read conf/docker/conf.yaml: %v", err)
	}
	for _, want := range []string{"env: docker", "config_center:", "server_addr: nacos:8848", "release:", "- polaris:8091", "- polaris:8093"} {
		if !strings.Contains(string(dockerConf), want) {
			t.Fatalf("docker conf missing %q\n---\n%s", want, dockerConf)
		}
	}
}

func TestGenerateWithDatabaseComposeIncludesPostgres(t *testing.T) {
	opts := baseOpts(t)
	opts.WithDatabase = true
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	composeBody, err := os.ReadFile(filepath.Join(res.Dir, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	for _, want := range []string{"postgres:", "DATABASE_URL: postgres://postgres:${POSTGRES_PASSWORD:-postgres}@postgres:5432/demo?sslmode=disable", "5432:5432"} {
		if !strings.Contains(string(composeBody), want) {
			t.Fatalf("database compose missing %q\n---\n%s", want, composeBody)
		}
	}
	dockerConf, err := os.ReadFile(filepath.Join(res.Dir, "conf", "docker", "conf.yaml"))
	if err != nil {
		t.Fatalf("read conf/docker/conf.yaml: %v", err)
	}
	for _, want := range []string{"env: docker", "database:", "enabled: true", `dsn: "postgres://postgres:postgres@postgres:5432/demo?sslmode=disable"`} {
		if !strings.Contains(string(dockerConf), want) {
			t.Fatalf("docker conf missing %q\n---\n%s", want, dockerConf)
		}
	}
}

func TestGenerateWritesKitexContainerFiles(t *testing.T) {
	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	dockerfile, err := os.ReadFile(filepath.Join(res.Dir, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	if !strings.Contains(string(dockerfile), "EXPOSE 8888") {
		t.Fatalf("kitex Dockerfile missing EXPOSE 8888\n---\n%s", dockerfile)
	}
	if !strings.Contains(string(dockerfile), "ENV GO_ENV=docker") {
		t.Fatalf("kitex Dockerfile missing GO_ENV=docker\n---\n%s", dockerfile)
	}
	composeBody, err := os.ReadFile(filepath.Join(res.Dir, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	for _, want := range []string{"8888:8888", "GO_ENV: docker"} {
		if !strings.Contains(string(composeBody), want) {
			t.Fatalf("kitex compose missing %q\n---\n%s", want, composeBody)
		}
	}
	dockerConf, err := os.ReadFile(filepath.Join(res.Dir, "conf", "docker", "conf.yaml"))
	if err != nil {
		t.Fatalf("read conf/docker/conf.yaml: %v", err)
	}
	if !strings.Contains(string(dockerConf), "env: docker") {
		t.Fatalf("kitex docker conf missing env docker\n---\n%s", dockerConf)
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

func TestNextStepsMakeTargetsMatchTemplates(t *testing.T) {
	cases := []struct {
		name       string
		kind       string
		withDB     bool
		expectSQLC bool
		steps      func(Options) []string
	}{
		{
			name:       "prepare-hertz-db",
			kind:       manifest.KindHertz,
			withDB:     true,
			expectSQLC: true,
			steps: func(opts Options) []string {
				return nextSteps(opts, defaultIDL(opts))
			},
		},
		{
			name:       "prepare-kitex-default",
			kind:       manifest.KindKitex,
			expectSQLC: true,
			steps: func(opts Options) []string {
				return nextSteps(opts, defaultIDL(opts))
			},
		},
		{
			name:       "prepare-kitex-db",
			kind:       manifest.KindKitex,
			withDB:     true,
			expectSQLC: true,
			steps: func(opts Options) []string {
				return nextSteps(opts, defaultIDL(opts))
			},
		},
		{
			name:       "post-generate-hertz-db",
			kind:       manifest.KindHertz,
			withDB:     true,
			expectSQLC: true,
			steps: func(opts Options) []string {
				return postGenerateNextSteps(opts)
			},
		},
		{
			name:       "post-generate-kitex-default",
			kind:       manifest.KindKitex,
			expectSQLC: true,
			steps: func(opts Options) []string {
				return postGenerateNextSteps(opts)
			},
		},
		{
			name:       "post-generate-kitex-db",
			kind:       manifest.KindKitex,
			withDB:     true,
			expectSQLC: true,
			steps: func(opts Options) []string {
				return postGenerateNextSteps(opts)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := baseOpts(t)
			opts.Kind = tc.kind
			opts.WithDatabase = tc.withDB
			steps := tc.steps(opts)
			joined := strings.Join(steps, "\n")
			if strings.Contains(joined, "make sqlc-gen") {
				t.Fatalf("next steps must not reference removed target sqlc-gen:\n%s", joined)
			}
			if tc.expectSQLC && !strings.Contains(joined, "make sqlc") {
				t.Fatalf("next steps missing make sqlc:\n%s", joined)
			}
			if tc.expectSQLC && strings.Index(joined, "make sqlc") > strings.Index(joined, "go mod tidy") {
				t.Fatalf("database next steps must run make sqlc before go mod tidy:\n%s", joined)
			}

			makefileBody := scaffoldMakeTemplate(t, opts)
			assertStepMakeTargetsExist(t, steps, makefileBody)
		})
	}
}

// TestResultNextStepsSafePrefixExecutes replays the user-facing NextSteps from
// the prepare-only path (NoGenerate=true) and executes every safe step against a
// fresh scaffold. We intentionally skip long-running/runtime-only steps:
//   - make migrate-up: requires an external DATABASE_URL-backed Postgres instance
//   - make dev: starts a persistent development server / watcher
func TestResultNextStepsSafePrefixExecutes(t *testing.T) {
	for _, tc := range nextStepsSmokeCases(false) {
		t.Run(tc.name, func(t *testing.T) {
			requireTools(t, tc.reqTools...)
			opts := baseOpts(t)
			opts.Kind = tc.kind
			opts.WithDatabase = tc.withDB

			res, err := Generate(context.Background(), opts)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if res.RanGenerate {
				t.Fatalf("RanGenerate = true, want false for next-steps smoke test")
			}

			executeSafeNextSteps(t, res.Dir, res.NextSteps)
		})
	}
}

// TestPostGenerateResultNextStepsSafePrefixExecutes replays the post-generate
// handoff after hz/kitex already ran. It uses the same skip policy as the
// prepare-path smoke test so we verify the actionable prefix without turning
// unit tests into database-dependent or long-running integration jobs.
func TestPostGenerateResultNextStepsSafePrefixExecutes(t *testing.T) {
	for _, tc := range nextStepsSmokeCases(true) {
		t.Run(tc.name, func(t *testing.T) {
			requireTools(t, tc.reqTools...)
			opts := baseOpts(t)
			opts.Kind = tc.kind
			opts.WithDatabase = tc.withDB
			opts.NoGenerate = tc.noGenerate

			res, err := Generate(context.Background(), opts)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if !res.RanGenerate {
				t.Fatalf("RanGenerate = false, want true for post-generate next-steps smoke test")
			}

			executeSafeNextSteps(t, res.Dir, res.NextSteps)
		})
	}
}

func TestShouldSkipNextStep(t *testing.T) {
	for _, tc := range []struct {
		step string
		want bool
	}{
		{step: "make migrate-up", want: true},
		{step: "make dev", want: true},
		{step: "ncgo add infra redis --root .", want: true},
		{step: "go mod tidy", want: false},
		{step: "make sqlc", want: false},
	} {
		if got := shouldSkipNextStep(tc.step); got != tc.want {
			t.Fatalf("shouldSkipNextStep(%q) = %v, want %v", tc.step, got, tc.want)
		}
	}
}

func TestNextStepsSmokeCases(t *testing.T) {
	for _, tc := range []struct {
		name           string
		postGenerate   bool
		wantNoGenerate bool
	}{
		{name: "prepare", postGenerate: false, wantNoGenerate: true},
		{name: "post-generate", postGenerate: true, wantNoGenerate: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cases := nextStepsSmokeCases(tc.postGenerate)
			if len(cases) != 4 {
				t.Fatalf("len(nextStepsSmokeCases(%v)) = %d, want 4", tc.postGenerate, len(cases))
			}
			for _, c := range cases {
				if c.noGenerate != tc.wantNoGenerate {
					t.Fatalf("case %q noGenerate = %v, want %v", c.name, c.noGenerate, tc.wantNoGenerate)
				}
				if c.kind == manifest.KindKitex && !slices.Contains(c.reqTools, "sqlc") {
					t.Fatalf("kitex case %q missing sqlc tool requirement: %v", c.name, c.reqTools)
				}
			}
		})
	}
}

func TestRequiresSQLCBeforeTidy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   string
		withDB bool
		want   bool
	}{
		{name: "hertz-default", kind: manifest.KindHertz, want: false},
		{name: "hertz-with-db", kind: manifest.KindHertz, withDB: true, want: true},
		{name: "kitex-default", kind: manifest.KindKitex, want: true},
		{name: "kitex-with-db", kind: manifest.KindKitex, withDB: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := baseOpts(t)
			opts.Kind = tc.kind
			opts.WithDatabase = tc.withDB
			if got := requiresSQLCBeforeTidy(opts); got != tc.want {
				t.Fatalf("requiresSQLCBeforeTidy(%s, withDB=%v) = %v, want %v", tc.kind, tc.withDB, got, tc.want)
			}
		})
	}
}

func TestNextStepsSequenceShapes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		kind         string
		withDB       bool
		postGenerate bool
		want         []string
	}{
		{name: "prepare-hertz-default", kind: manifest.KindHertz, want: []string{"cd", "go mod init", "<generate>", "go mod tidy", "make dev"}},
		{name: "prepare-hertz-with-db", kind: manifest.KindHertz, withDB: true, want: []string{"cd", "go mod init", "<generate>", "make sqlc", "go mod tidy", "make migrate-up", "make dev"}},
		{name: "prepare-kitex-default", kind: manifest.KindKitex, want: []string{"cd", "go mod init", "<generate>", "make sqlc", "go mod tidy", "make dev"}},
		{name: "post-hertz-default", kind: manifest.KindHertz, postGenerate: true, want: []string{"cd", "go mod tidy", "make dev"}},
		{name: "post-hertz-with-db", kind: manifest.KindHertz, withDB: true, postGenerate: true, want: []string{"cd", "make sqlc", "go mod tidy", "make migrate-up", "make dev"}},
		{name: "post-kitex-default", kind: manifest.KindKitex, postGenerate: true, want: []string{"cd", "make sqlc", "go mod tidy", "make dev"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := baseOpts(t)
			opts.Kind = tc.kind
			opts.WithDatabase = tc.withDB

			var got []string
			if tc.postGenerate {
				got = stepSequenceShape(postGenerateNextSteps(opts))
			} else {
				got = stepSequenceShape(nextSteps(opts, defaultIDL(opts)))
			}
			if !equal(got, tc.want) {
				t.Fatalf("step sequence mismatch\n got: %v\nwant: %v", got, tc.want)
			}
		})
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
		"cfg.Logging / cfg.Release.Info",
		"h.Use(logging.HertzRecovery())",
		"h.Use(logging.HertzRequestID())",
		"h.Use(logging.HertzAccessLog())",
		"Optional release canary wiring",
		"// ncgo:wire:canary:server-traffic",
		"import \"{{.GoModule}}/internal/base/release\"",
		"if cfg.Release.Enabled {",
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
		"# 配置中心：用于在本地文件之上叠加远程配置；默认关闭",
		"# provider 需与 conf.RegisterConfigCenterLoader 注册名一致",
		"# 数据库配置",
		"# PostgreSQL DSN；例如 postgres://user:pass@host:5432/dbname?sslmode=disable",
		"# 运维建议：略小于数据库或代理层连接回收时间，避免同时批量失效",
		"# 当 key_by 包含 ak / ak_user_uuid 等维度时，从该请求头读取 app key",
		"# 开发环境静态密钥；生产环境建议改为配置中心或密钥管理系统",
		"# 运维建议：不要把真实生产密钥直接写入仓库或镜像",
		"# 携带 token 的请求头名；中间件通常支持 Bearer 前缀",
		"# token 签发者；校验时需与 token 中 iss 对齐",
		"# 运维建议：有效期越长，泄漏后的风险窗口越大；需结合登录态策略权衡",
		"# token 有效期，单位秒",
		"# 结构化日志配置（与 ncgo add infra logging 生成的 optional 兼容）",
		"# 发布/灰度配置（与 ncgo add infra canary 生成的 optional 兼容）",
		"# 规则中心提供方：config 仅使用本地规则文件；nacos / polaris 通过配置中心加载灰度规则",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz config comments missing %q", want)
		}
	}
}

func TestGenerateHertzTemplateIncludesConfigCenterAndOptionalConfigModels(t *testing.T) {
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
		"ConfigCenter ConfigCenterConfig",
		"Database     DatabaseConfig",
		"Redis        RedisConfig",
		"Logging      LoggingConfig",
		"Release      ReleaseConfig",
		"type DatabaseConfig struct",
		"type RedisConfig = RateLimitRedisConfig",
		"func NewPostgresConfigFromDatabase(cfg conf.DatabaseConfig)",
		"type ConfigCenterLoader func(ConfigCenterConfig) ([]byte, error)",
		"func RegisterConfigCenterLoader(provider string, loader ConfigCenterLoader)",
		"func mergeConfigCenter(cfg *Config) error",
		"func (c *Config) applyRedisFallbacks()",
		"func sharedRedisClient(cfg conf.RedisConfig) redis.UniversalClient",
		"func SharedRedisClient(cfg RedisConfig) redis.UniversalClient",
		"defer data.CloseSharedRedisClients()",
		"func TestRedisStoresReuseSharedClient(t *testing.T)",
		"database.dsn is empty",
		"type ReleaseRulesConfig struct",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("hertz config model missing %q", want)
		}
	}
}

func TestGenerateHertzWithDatabaseRendersTopLevelDatabaseConfig(t *testing.T) {
	requireTools(t, "hz")

	opts := baseOpts(t)
	opts.NoGenerate = false
	opts.WithDatabase = true

	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Fatal("expected RanGenerate = true")
	}

	confBody, err := os.ReadFile(filepath.Join(res.Dir, "conf", "dev", "conf.yaml"))
	if err != nil {
		t.Fatalf("read rendered conf.yaml: %v", err)
	}
	for _, want := range []string{
		"database:",
		"enabled: false",
		"dsn: \"\"",
		"server_addr: \"\"",
		"addresses: []",
		"max_conns: 20",
		"health_check_period_seconds: 30",
		"# Redis 连接配置：作为共享默认值供 rate_limit / idempotency / signature nonce 复用",
		"context_timeout_enabled: true",
		"# 可选：仅当需要覆盖顶层 redis 连接时填写；留空则复用顶层 redis",
		"redis: {}",
	} {
		if !strings.Contains(string(confBody), want) {
			t.Fatalf("rendered conf/dev/conf.yaml missing %q\n---\n%s", want, confBody)
		}
	}

	goBody, err := os.ReadFile(filepath.Join(res.Dir, "internal", "base", "conf", "conf.go"))
	if err != nil {
		t.Fatalf("read rendered conf.go: %v", err)
	}
	for _, want := range []string{
		"Database DatabaseConfig",
		"Redis       RedisConfig",
		"ServerAddr  string `json:\"server_addr\" yaml:\"server_addr\"`",
		"Addresses []string `json:\"addresses\" yaml:\"addresses\"`",
		"type DatabaseConfig struct",
		"type RedisConfig = RateLimitRedisConfig",
		"func (c *Config) applyRedisFallbacks()",
		"c.RateLimit.Redis = mergeRedisConfig(c.RateLimit.Redis, c.Redis)",
		"database.dsn is empty",
		"database pool settings must not be negative",
	} {
		if !strings.Contains(string(goBody), want) {
			t.Fatalf("rendered internal/base/conf/conf.go missing %q\n---\n%s", want, goBody)
		}
	}

	dataBody, err := os.ReadFile(filepath.Join(res.Dir, "internal", "base", "data", "data.go"))
	if err != nil {
		t.Fatalf("read rendered data.go: %v", err)
	}
	for _, want := range []string{
		"func NewPostgresConfigFromDatabase(cfg conf.DatabaseConfig)",
		"pgCfg.MaxConns = cfg.MaxConns",
		"pgCfg.HealthCheckPeriod = time.Duration(cfg.HealthCheckPeriodSeconds) * time.Second",
	} {
		if !strings.Contains(string(dataBody), want) {
			t.Fatalf("rendered internal/base/data/data.go missing %q\n---\n%s", want, dataBody)
		}
	}

	serverBody, err := os.ReadFile(filepath.Join(res.Dir, "internal", "base", "server", "server.go"))
	if err != nil {
		t.Fatalf("read rendered server.go: %v", err)
	}
	for _, want := range []string{
		"if cfg.Database.Enabled {",
		"startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)",
		"pgCfg, err := data.NewPostgresConfigFromDatabase(cfg.Database)",
		"pool, err := data.NewPostgres(startupCtx, pgCfg)",
		"dbData, cleanup, err := data.New(pool)",
		"defer cleanup()",
		"do.ProvideValue[context.Context](injector, startupCtx)",
		"do.ProvideValue(injector, pool)",
		"do.ProvideValue(injector, dbData)",
		"dbData = do.MustInvoke[*data.Data](injector)",
		"repository.NewRateLimitRuleRepository(dbData)",
	} {
		if !strings.Contains(string(serverBody), want) {
			t.Fatalf("rendered internal/base/server/server.go missing %q\n---\n%s", want, serverBody)
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

func TestGenerateNoGenerateWithRedisAddsHandoffStep(t *testing.T) {
	opts := baseOpts(t)
	opts.Infra = []string{"redis"}
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "ncgo add infra redis --root .") {
		t.Fatalf("next steps missing redis handoff\n---\n%s", joined)
	}
	if _, err := os.Stat(filepath.Join(res.Dir, "internal", "base", "data", "redis.go")); !os.IsNotExist(err) {
		t.Fatalf("redis add-on should not be materialized before generator run: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, ".ncgo", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if strings.Contains(string(b), "infra:") {
		t.Fatalf("manifest should not record redis before infra add runs\n---\n%s", string(b))
	}
}

func TestGeneratePostGenerateAddsRedisInfra(t *testing.T) {
	opts := baseOpts(t)
	opts.NoGenerate = false
	opts.Infra = []string{"redis"}
	opts.Runner = &fakeRunner{}
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Fatal("expected RanGenerate = true")
	}
	if strings.Contains(strings.Join(res.NextSteps, "\n"), "ncgo add infra redis --root .") {
		t.Fatalf("post-generate next steps should not include redis handoff: %v", res.NextSteps)
	}
	redisPath := filepath.Join(res.Dir, "internal", "base", "data", "redis.go")
	if _, err := os.Stat(redisPath); err != nil {
		t.Fatalf("stat redis.go: %v", err)
	}
	helperPath := filepath.Join(res.Dir, "internal", "base", "data", "redis_shared.go")
	if _, err := os.Stat(helperPath); err != nil {
		t.Fatalf("stat redis_shared.go: %v", err)
	}
	redisBody, err := os.ReadFile(redisPath)
	if err != nil {
		t.Fatalf("read redis.go: %v", err)
	}
	for _, want := range []string{"func NewRedis(ctx context.Context, cfg *Config)", "SharedRedisClient(cfg.Redis)"} {
		if !strings.Contains(string(redisBody), want) {
			t.Fatalf("redis.go missing %q\n---\n%s", want, redisBody)
		}
	}
	helperBody, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read redis_shared.go: %v", err)
	}
	for _, want := range []string{"func SharedRedisClient(cfg RedisConfig) redis.UniversalClient", "type Config = conf.Config"} {
		if !strings.Contains(string(helperBody), want) {
			t.Fatalf("redis_shared.go missing %q\n---\n%s", want, helperBody)
		}
	}
	confBody, err := os.ReadFile(filepath.Join(res.Dir, "conf", "dev", "conf.yaml"))
	if err != nil {
		t.Fatalf("read conf/dev/conf.yaml: %v", err)
	}
	if !strings.Contains(string(confBody), "# ncgo:add-infra:start redis") {
		t.Fatalf("conf/dev/conf.yaml missing redis block\n---\n%s", confBody)
	}
	b, err := os.ReadFile(filepath.Join(res.Dir, ".ncgo", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(b), "infra:") || !strings.Contains(string(b), "- redis") {
		t.Fatalf("manifest missing redis infra entry\n---\n%s", string(b))
	}
}

func TestGenerateRejectsUnsupportedCreationInfra(t *testing.T) {
	opts := baseOpts(t)
	opts.Infra = []string{"kafka"}
	if _, err := Generate(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "not supported by ncgo new yet") {
		t.Fatalf("err = %v, want unsupported infra error", err)
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
// builds the generated project, and runs the generated project's core package
// tests (including i18n and dynamic rate-limit packages) to verify template
// code compiles and behaves correctly after generation.
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

	// Run the generated project's own rate-limit packages to verify the shipped
	// dynamic resolver, repository hook, middleware, and their smoke tests work
	// in a fresh project.
	cmd = osexec.CommandContext(context.Background(), "go", "test", "-race", "-count=1", "./internal/base/conf/...", "./internal/repository/...", "./internal/pkg/ratelimit/...", "./internal/pkg/middleware/...")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test rate-limit packages in %s: %v\n%s", res.Dir, err, out)
	}

	cmd = osexec.CommandContext(context.Background(), "go", "test", "-race", "-count=1", "./tools/...")
	cmd.Dir = res.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./tools/... in %s: %v\n%s", res.Dir, err, out)
	}
}

func TestGenerateHertzWithDatabaseCompiles(t *testing.T) {
	requireTools(t, "hz", "make", "sqlc")

	opts := baseOpts(t)
	opts.NoGenerate = false
	opts.WithDatabase = true

	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Fatal("expected RanGenerate = true")
	}

	runInDir(t, res.Dir, "make", "sqlc")
	runInDir(t, res.Dir, "go", "mod", "tidy")
	runInDir(t, res.Dir, "make", "i18n")
	runInDir(t, res.Dir, "go", "build", ".")
	runInDir(t, res.Dir, "go", "test", "-race", "-count=1", "./internal/base/conf/...", "./internal/base/data/...", "./internal/repository/...", "./internal/pkg/ratelimit/...", "./internal/pkg/middleware/...")
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
		"internal/db/query/health.sql",
		"internal/db/schema/000001_placeholder.sql",
		"internal/db/sqlc.yaml",
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

func TestGenerateKitexCompiles(t *testing.T) {
	requireTools(t, "kitex", "make", "sqlc")

	opts := baseOpts(t)
	opts.Kind = manifest.KindKitex
	opts.NoGenerate = false

	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.RanGenerate {
		t.Fatal("expected RanGenerate = true")
	}

	runInDir(t, res.Dir, "make", "sqlc")
	runInDir(t, res.Dir, "go", "mod", "tidy")
	runInDir(t, res.Dir, "go", "build", ".")
	runInDir(t, res.Dir, "go", "test", "-race", "-count=1", "./internal/pkg/interceptor/...", "./internal/pkg/rpcerror/...", "./pkg/client/...")
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

func scaffoldMakeTemplate(t *testing.T, opts Options) string {
	t.Helper()
	res, err := Generate(context.Background(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	path := filepath.Join(res.Dir, "template", "layout.yaml")
	if defaultKind(opts.Kind) == manifest.KindKitex {
		path = filepath.Join(res.Dir, "template", "kitex-template", "makefile.yaml")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read makefile template %s: %v", path, err)
	}
	return string(body)
}

func assertStepMakeTargetsExist(t *testing.T, steps []string, makefileBody string) {
	t.Helper()
	for _, step := range steps {
		if !strings.HasPrefix(step, "make ") {
			continue
		}
		target := strings.Fields(strings.TrimPrefix(step, "make "))[0]
		if !strings.Contains(makefileBody, target+":") {
			t.Fatalf("make target %q from next steps missing in template:\n%s", target, makefileBody)
		}
	}
}

var skippedNextStepReasons = map[string]string{
	"make migrate-up":               "requires external database configuration/state",
	"make dev":                      "starts a long-running development process",
	"ncgo add infra redis --root .": "requires the ncgo CLI binary in PATH; scaffold tests cover this path separately",
}

type nextStepsSmokeCase struct {
	name       string
	kind       string
	withDB     bool
	noGenerate bool
	reqTools   []string
}

func nextStepsSmokeCases(postGenerate bool) []nextStepsSmokeCase {
	cases := []nextStepsSmokeCase{
		{name: "hertz-default", kind: manifest.KindHertz, noGenerate: !postGenerate, reqTools: []string{"hz"}},
		{name: "hertz-with-db", kind: manifest.KindHertz, withDB: true, noGenerate: !postGenerate, reqTools: []string{"hz", "make", "sqlc"}},
		{name: "kitex-default", kind: manifest.KindKitex, noGenerate: !postGenerate, reqTools: []string{"kitex", "make", "sqlc"}},
		{name: "kitex-with-db", kind: manifest.KindKitex, withDB: true, noGenerate: !postGenerate, reqTools: []string{"kitex", "make", "sqlc"}},
	}
	return cases
}

// executeSafeNextSteps runs the command prefix from Result.NextSteps that is
// safe and deterministic inside unit tests. The explicit skip list documents
// which handoff steps are intentionally left for higher-level/manual validation.
func executeSafeNextSteps(t *testing.T, projectDir string, steps []string) {
	t.Helper()
	cwd := mustCwd()
	for _, step := range steps {
		switch {
		case strings.HasPrefix(step, "cd "):
			cwd = filepath.Clean(filepath.Join(cwd, strings.TrimSpace(strings.TrimPrefix(step, "cd "))))
		case shouldSkipNextStep(step):
			t.Logf("skipping next step %q: %s", step, skippedNextStepReasons[step])
			continue
		default:
			runStep(t, cwd, step)
		}
	}
	if got, want := filepath.Clean(cwd), filepath.Clean(projectDir); got != want {
		t.Fatalf("next steps cd resolved to %q, want %q", got, want)
	}
}

func shouldSkipNextStep(step string) bool {
	_, ok := skippedNextStepReasons[step]
	return ok
}

func runStep(t *testing.T, dir, step string) {
	t.Helper()
	parts := strings.Fields(step)
	if len(parts) == 0 {
		return
	}
	runInDir(t, dir, parts[0], parts[1:]...)
}

func stepSequenceShape(steps []string) []string {
	shape := make([]string, 0, len(steps))
	for _, step := range steps {
		switch {
		case strings.HasPrefix(step, "cd "):
			shape = append(shape, "cd")
		case strings.HasPrefix(step, "go mod init "):
			shape = append(shape, "go mod init")
		case strings.HasPrefix(step, "hz new "), strings.HasPrefix(step, "kitex "):
			shape = append(shape, "<generate>")
		default:
			shape = append(shape, step)
		}
	}
	return shape
}

func requireTools(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s not found on PATH", name)
		}
	}
}

func runInDir(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	cmd := osexec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s in %s: %v\n%s", name, strings.Join(args, " "), dir, err, out)
	}
	return out
}
