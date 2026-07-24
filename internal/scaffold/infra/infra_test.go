package infra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func seedProject(t *testing.T, infra []string) string {
	t.Helper()
	return seedProjectKind(t, manifest.KindHertz, "idl/app/demo.proto", infra)
}

func seedKitexProject(t *testing.T, infra []string) string {
	t.Helper()
	return seedProjectKind(t, manifest.KindKitex, "idl/demo.proto", infra)
}

func seedWorkspaceServiceProject(t *testing.T, kind string, serviceInfra []string) (string, string) {
	t.Helper()
	root := t.TempDir()
	w := &manifest.Workspace{
		Ncgo:        manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:        manifest.ModeMicro,
		Name:        "commerce",
		Module:      "github.com/x/commerce",
		Services:    []manifest.WorkspaceService{{Name: "demo", Kind: kind, Dir: "services/demo"}},
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.SaveWorkspace(root, w); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	serviceRoot := filepath.Join(root, "services", "demo")
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/commerce/services/demo",
		Service: manifest.Service{
			Name: "demo", Kind: kind, IDL: "idl/demo.proto",
		},
		Infra:       serviceInfra,
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if kind == manifest.KindHertz {
		m.Service.IDL = "idl/app/demo.proto"
	}
	if err := manifest.Save(serviceRoot, m); err != nil {
		t.Fatalf("seed workspace service: %v", err)
	}
	return root, serviceRoot
}

func seedProjectKind(t *testing.T, kind, idl string, infra []string) string {
	t.Helper()
	root := t.TempDir()
	m := &manifest.Manifest{
		Ncgo:   manifest.Meta{Version: "0.0.0-test", AssetsVersion: "test"},
		Mode:   manifest.ModeMono,
		Module: "github.com/x/demo",
		Service: manifest.Service{
			Name: "demo", Kind: kind, IDL: idl,
		},
		Infra:       infra,
		GeneratedAt: time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
	}
	if err := manifest.Save(root, m); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return root
}

func TestAddRedisCopiesFileAndUpdatesManifest(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindRedis})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !res.Updated {
		t.Errorf("Updated = false, want true")
	}
	if !planContains(res.Plan, "file", "create", res.WrittenPath, "") {
		t.Errorf("Plan missing file create for %s: %+v", res.WrittenPath, res.Plan)
	}
	if !planContains(res.Plan, "manifest", "add", filepath.Join(".ncgo", "manifest.yaml"), "") {
		t.Errorf("Plan missing manifest add: %+v", res.Plan)
	}
	if !planContains(res.Plan, "next_step", "run", "", "go get github.com/byx-darwin/go-tools/go-middleware") {
		t.Errorf("Plan missing redis next step: %+v", res.Plan)
	}
	if want := filepath.Join(root, "internal", "base", "data", "redis.go"); res.WrittenPath != want {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, want)
	}
	helperPath := filepath.Join(root, "internal", "base", "data", "redis_shared.go")
	if !sliceContains(res.WrittenPaths, helperPath) {
		t.Errorf("WrittenPaths missing redis helper %q: %v", helperPath, res.WrittenPaths)
	}
	body, err := os.ReadFile(res.WrittenPath)
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	for _, want := range []string{
		"package data",
		"func NewRedis(ctx context.Context, cfg *Config)",
		"SharedRedisClient(cfg.Redis)",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("written file missing %q:\n%s", want, body)
		}
	}
	helperBody, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}
	for _, want := range []string{
		"package data",
		"type Config = conf.Config",
		"func SharedRedisClient(cfg RedisConfig) redis.UniversalClient",
	} {
		if !strings.Contains(string(helperBody), want) {
			t.Errorf("helper missing %q:\n%s", want, helperBody)
		}
	}
	confPath := filepath.Join(root, "conf", "dev", "conf.yaml")
	if !planContains(res.Plan, "file", "create", confPath, "") {
		t.Errorf("Plan missing config create for %s: %+v", confPath, res.Plan)
	}
	confBody, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(confBody), "# ncgo:add-infra:start redis") || !strings.Contains(string(confBody), "redis:") {
		t.Errorf("redis config block missing in conf/dev/conf.yaml:\n%s", confBody)
	}
	dockerConfPath := filepath.Join(root, "conf", "docker", "conf.yaml")
	dockerConfBody, err := os.ReadFile(dockerConfPath)
	if err != nil {
		t.Fatalf("read docker config: %v", err)
	}
	for _, want := range []string{"env: docker", "redis:", "- redis:6379"} {
		if !strings.Contains(string(dockerConfBody), want) {
			t.Fatalf("docker conf missing %q\n---\n%s", want, dockerConfBody)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "conf", "dev", "redis.yaml")); !os.IsNotExist(err) {
		t.Errorf("redis should no longer write a standalone redis.yaml: stat err = %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindRedis {
		t.Errorf("manifest.Infra = %v, want [redis]", m.Infra)
	}
	composeBody, err := os.ReadFile(filepath.Join(root, "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	for _, want := range []string{"demo:", "redis:", "config-center-nacos"} {
		if !strings.Contains(string(composeBody), want) {
			t.Fatalf("compose missing %q\n---\n%s", want, composeBody)
		}
	}
}

func TestAddRedisRefreshesParentWorkspaceCompose(t *testing.T) {
	workspaceRoot, serviceRoot := seedWorkspaceServiceProject(t, manifest.KindHertz, nil)
	if _, err := Add(Options{Root: serviceRoot, Kind: KindRedis}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	composeBody, err := os.ReadFile(filepath.Join(workspaceRoot, "compose.yaml"))
	if err != nil {
		t.Fatalf("read workspace compose: %v", err)
	}
	for _, want := range []string{"./services/demo", "redis:", "18080:8080"} {
		if !strings.Contains(string(composeBody), want) {
			t.Fatalf("workspace compose missing %q\n---\n%s", want, composeBody)
		}
	}
}

func TestAddRedisSkipsExistingSharedHelper(t *testing.T) {
	root := seedProject(t, nil)
	helperPath := filepath.Join(root, "internal", "base", "data", "redis_shared.go")
	if err := os.MkdirAll(filepath.Dir(helperPath), 0o755); err != nil {
		t.Fatalf("mkdir helper dir: %v", err)
	}
	const helperBody = "package data\n\n// pre-existing helper\n"
	if err := os.WriteFile(helperPath, []byte(helperBody), 0o644); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	res, err := Add(Options{Root: root, Kind: KindRedis})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if sliceContains(res.WrittenPaths, helperPath) {
		t.Fatalf("WrittenPaths should not rewrite pre-existing helper: %v", res.WrittenPaths)
	}
	if got := readFile(t, helperPath); got != helperBody {
		t.Fatalf("helper was modified\n--- got ---\n%s\n--- want ---\n%s", got, helperBody)
	}
	redisBody := readFile(t, filepath.Join(root, "internal", "base", "data", "redis.go"))
	if !strings.Contains(redisBody, "SharedRedisClient(cfg.Redis)") {
		t.Fatalf("redis.go should still reuse shared helper\n---\n%s", redisBody)
	}
	assertManifestInfra(t, root, KindRedis)
}

func TestAddHertzDataInfraWritesConfigIntoSingleConfFile(t *testing.T) {
	for _, tc := range []struct {
		kind       string
		wantKey    string
		standalone string
	}{
		{kind: KindKafka, wantKey: "kafka:", standalone: filepath.Join("conf", "dev", "kafka.yaml")},
		{kind: KindES, wantKey: "es:", standalone: filepath.Join("conf", "dev", "es.yaml")},
		{kind: KindClickHouse, wantKey: "clickhouse:", standalone: filepath.Join("conf", "dev", "clickhouse.yaml")},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			root := seedProject(t, nil)
			res, err := Add(Options{Root: root, Kind: tc.kind})
			if err != nil {
				t.Fatalf("Add %s: %v", tc.kind, err)
			}
			confPath := filepath.Join(root, "conf", "dev", "conf.yaml")
			if !pathListed(res.WrittenPaths, confPath) {
				t.Fatalf("WrittenPaths = %v, want to include %s", res.WrittenPaths, confPath)
			}
			if !planContains(res.Plan, "file", "create", confPath, "") {
				t.Fatalf("Plan missing config create for %s: %+v", confPath, res.Plan)
			}
			body, err := os.ReadFile(confPath)
			if err != nil {
				t.Fatalf("read config %s: %v", confPath, err)
			}
			s := string(body)
			if !strings.Contains(s, tc.wantKey) {
				t.Fatalf("config %s missing %q\n---\n%s", confPath, tc.wantKey, s)
			}
			if !strings.Contains(s, "# ncgo:add-infra:start "+tc.kind) {
				t.Fatalf("config %s missing generated marker for %s\n---\n%s", confPath, tc.kind, s)
			}
			joined := strings.Join(res.NextSteps, "\n")
			if !strings.Contains(joined, filepath.Join("conf", "dev", "conf.yaml")) {
				t.Fatalf("next steps missing conf/dev/conf.yaml guidance\n---\n%s", joined)
			}
			if _, err := os.Stat(filepath.Join(root, tc.standalone)); !os.IsNotExist(err) {
				t.Fatalf("standalone snippet file should not exist (%s): %v", tc.standalone, err)
			}
			dockerConfPath := filepath.Join(root, "conf", "docker", "conf.yaml")
			dockerBody, err := os.ReadFile(dockerConfPath)
			if err != nil {
				t.Fatalf("read docker config %s: %v", dockerConfPath, err)
			}
			if !strings.Contains(string(dockerBody), "env: docker") {
				t.Fatalf("docker conf missing env docker\n---\n%s", dockerBody)
			}
			var dockerWant string
			switch tc.kind {
			case KindKafka:
				dockerWant = "- kafka:9092"
			case KindES:
				dockerWant = "- http://elasticsearch:9200"
			case KindClickHouse:
				dockerWant = "- clickhouse:9000"
			}
			if dockerWant != "" && !strings.Contains(string(dockerBody), dockerWant) {
				t.Fatalf("docker conf missing %q\n---\n%s", dockerWant, dockerBody)
			}
		})
	}
}

func TestAddHertzDataInfraAppendsToExistingConfFile(t *testing.T) {
	root := seedProject(t, nil)
	confPath := filepath.Join(root, "conf", "dev", "conf.yaml")
	writeTestFile(t, confPath, "env: dev\nserver:\n  name: demo\n")

	res, err := Add(Options{Root: root, Kind: KindKafka})
	if err != nil {
		t.Fatalf("Add kafka: %v", err)
	}
	if !planContains(res.Plan, "file", "update", confPath, "") {
		t.Fatalf("Plan missing config update for %s: %+v", confPath, res.Plan)
	}
	body := readFile(t, confPath)
	for _, want := range []string{"env: dev", "server:", "# ncgo:add-infra:start kafka", "kafka:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("conf/dev/conf.yaml missing %q\n---\n%s", want, body)
		}
	}
}

func TestAddIsIdempotent(t *testing.T) {
	root := seedProject(t, []string{KindRedis})
	dst := filepath.Join(root, "internal", "base", "data", "redis.go")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dst, []byte("// pre-existing\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := Add(Options{Root: root, Kind: KindRedis}); err == nil {
		t.Fatalf("expected error for existing file without --force")
	}
	res, err := Add(Options{Root: root, Kind: KindRedis, Force: true})
	if err != nil {
		t.Fatalf("Add force: %v", err)
	}
	if res.Updated {
		t.Errorf("Updated = true, want false (already in manifest)")
	}
	if !planContains(res.Plan, "file", "overwrite", dst, "") {
		t.Errorf("Plan missing file overwrite for %s: %+v", dst, res.Plan)
	}
	if !planContains(res.Plan, "manifest", "already_present", filepath.Join(".ncgo", "manifest.yaml"), "") {
		t.Errorf("Plan missing manifest already_present: %+v", res.Plan)
	}
	body, _ := os.ReadFile(dst)
	if strings.Contains(string(body), "pre-existing") {
		t.Errorf("force did not overwrite file")
	}
}

func TestAddDedupesAndSorts(t *testing.T) {
	root := seedProject(t, []string{KindKafka})
	if _, err := Add(Options{Root: root, Kind: KindClickHouse}); err != nil {
		t.Fatalf("Add clickhouse: %v", err)
	}
	if _, err := Add(Options{Root: root, Kind: KindRedis}); err != nil {
		t.Fatalf("Add redis: %v", err)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	want := []string{"clickhouse", "kafka", "redis"}
	if len(m.Infra) != len(want) {
		t.Fatalf("Infra = %v, want %v", m.Infra, want)
	}
	for i := range want {
		if m.Infra[i] != want[i] {
			t.Errorf("Infra[%d] = %q, want %q", i, m.Infra[i], want[i])
		}
	}
}

func TestAddRejectsUnknownKind(t *testing.T) {
	root := seedProject(t, nil)
	if _, err := Add(Options{Root: root, Kind: "mongo"}); err == nil {
		t.Fatalf("expected error for unknown kind")
	}
}

func TestAddRequiresManifest(t *testing.T) {
	root := t.TempDir()
	if _, err := Add(Options{Root: root, Kind: KindRedis}); err == nil {
		t.Fatalf("expected error when manifest missing")
	}
}

func TestGoGetDepsIncludeGoCommon(t *testing.T) {
	for _, kind := range []string{KindRedis, KindKafka, KindES, KindClickHouse, KindRegistryEtcd, KindObservabilityLog} {
		deps := goGetDeps[kind]
		found := false
		for _, d := range deps {
			if d == "github.com/byx-darwin/go-tools/go-common" {
				found = true
			}
			if d == "github.com/samber/oops" {
				t.Errorf("goGetDeps[%s] must not list samber/oops (indirect dep of go-common): %v", kind, deps)
			}
		}
		if !found {
			t.Errorf("goGetDeps[%s] missing go-common: %v", kind, deps)
		}
	}
}

func TestAddNextStepsContainGoGet(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindRedis})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	joined := strings.Join(res.NextSteps, "\n")
	for _, want := range []string{
		"go get github.com/byx-darwin/go-tools/go-middleware",
		"go get github.com/byx-darwin/go-tools/go-common",
		"go mod tidy",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("NextSteps missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "go get github.com/samber/oops") {
		t.Errorf("NextSteps should not hint samber/oops (oops is an indirect dep of go-common):\n%s", joined)
	}
}

func TestAddKitexOnlyRegistryEtcd(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindRegistryEtcd})
	if err != nil {
		t.Fatalf("Add registry_etcd: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "registry", "etcd.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read registry file: %v", err)
	}
	if !strings.Contains(string(body), "package registry") {
		t.Errorf("registry_etcd should write package registry")
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "go get github.com/kitex-contrib/registry-etcd") {
		t.Errorf("next steps missing registry-etcd dep:\n%s", joined)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindRegistryEtcd {
		t.Errorf("manifest.Infra = %v, want [registry_etcd]", m.Infra)
	}
}

func TestAddObservabilityOtelForKitex(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindObservabilityOtel})
	if err != nil {
		t.Fatalf("Add observability_otel: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "observability", "otel.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read observability file: %v", err)
	}
	if !strings.Contains(string(body), "package observability") {
		t.Errorf("observability_otel should write package observability")
	}
	if !strings.Contains(string(body), "type LoongSuiteConfig struct") {
		t.Errorf("observability_otel should expose LoongSuiteConfig")
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "loongsuite-go-agent") || !strings.Contains(joined, "otel go build ./...") {
		t.Errorf("next steps missing LoongSuite setup:\n%s", joined)
	}
}

func TestAddObservabilityOtelForHertz(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindObservabilityOtel})
	if err != nil {
		t.Fatalf("Add observability_otel: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "observability", "otel.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read observability file: %v", err)
	}
	if !strings.Contains(string(body), "type LoongSuiteConfig struct") || !strings.Contains(string(body), "func DefaultLoongSuiteConfig") {
		t.Errorf("observability_otel missing expected API")
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindObservabilityOtel {
		t.Errorf("manifest.Infra = %v, want [observability_otel]", m.Infra)
	}
	if _, err := Add(Options{Root: root, Kind: KindObservabilityOtel}); err == nil {
		t.Fatalf("expected existing file error without --force")
	}
	res, err = Add(Options{Root: root, Kind: KindObservabilityOtel, Force: true})
	if err != nil {
		t.Fatalf("Add observability_otel --force: %v", err)
	}
	if res.Updated {
		t.Errorf("Updated = true, want false after dedup")
	}
}

func TestAddOtelAliasRecordsCanonicalKind(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindOtelAlias})
	if err != nil {
		t.Fatalf("Add otel alias: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "observability", "otel.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindObservabilityOtel {
		t.Errorf("manifest.Infra = %v, want [observability_otel]", m.Infra)
	}
}

func TestAddObservabilityLoggingForHertz(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindObservabilityLog})
	if err != nil {
		t.Fatalf("Add observability_logging: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "logging", "logging.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	wantAdapterPath := filepath.Join(root, "internal", "base", "logging", "hertz.go")
	if strings.Join(res.WrittenPaths, "\n") != strings.Join([]string{wantPath, wantAdapterPath}, "\n") {
		t.Errorf("WrittenPaths = %v, want [%s %s]", res.WrittenPaths, wantPath, wantAdapterPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read logging file: %v", err)
	}
	for _, want := range []string{"package logging", "goclog", "CategoryAccess", "WithRequestID", "SinceMS"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("logging template missing %q", want)
		}
	}
	adapterBody, err := os.ReadFile(wantAdapterPath)
	if err != nil {
		t.Fatalf("read hertz logging adapter: %v", err)
	}
	for _, want := range []string{"func HertzRequestID", "func HertzAccessLog", "func HertzRecovery"} {
		if !strings.Contains(string(adapterBody), want) {
			t.Errorf("hertz logging adapter missing %q", want)
		}
	}
	joined := strings.Join(res.NextSteps, "\n")
	for _, want := range []string{"go get github.com/byx-darwin/go-tools/go-common", "go mod tidy"} {
		if !strings.Contains(joined, want) {
			t.Errorf("next steps missing %q in:\n%s", want, joined)
		}
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindObservabilityLog {
		t.Errorf("manifest.Infra = %v, want [observability_logging]", m.Infra)
	}
}

func TestAddLoggingAliasRecordsCanonicalKind(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindLoggingAlias})
	if err != nil {
		t.Fatalf("Add logging alias: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "logging", "logging.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	wantAdapterPath := filepath.Join(root, "internal", "base", "logging", "kitex.go")
	if strings.Join(res.WrittenPaths, "\n") != strings.Join([]string{wantPath, wantAdapterPath}, "\n") {
		t.Errorf("WrittenPaths = %v, want [%s %s]", res.WrittenPaths, wantPath, wantAdapterPath)
	}
	adapterBody, err := os.ReadFile(wantAdapterPath)
	if err != nil {
		t.Fatalf("read kitex logging adapter: %v", err)
	}
	for _, want := range []string{"func KitexRequestID", "func KitexAccessLog", "func KitexRecovery"} {
		if !strings.Contains(string(adapterBody), want) {
			t.Errorf("kitex logging adapter missing %q", want)
		}
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindObservabilityLog {
		t.Errorf("manifest.Infra = %v, want [observability_logging]", m.Infra)
	}
}

func TestAddReleaseCanaryForHertz(t *testing.T) {
	root := seedProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindReleaseCanary})
	if err != nil {
		t.Fatalf("Add release_canary: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "release", "canary.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	wantAdapterPath := filepath.Join(root, "internal", "base", "release", "hertz.go")
	if strings.Join(res.WrittenPaths, "\n") != strings.Join([]string{wantPath, wantAdapterPath}, "\n") {
		t.Errorf("WrittenPaths = %v, want [%s %s]", res.WrittenPaths, wantPath, wantAdapterPath)
	}
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read canary file: %v", err)
	}
	for _, want := range []string{"package release", "type ReleaseInfo struct", "type RuleSet struct", "type Selector struct", "func Select(", "func SplitInstances", "ProviderNacos", "ProviderPolaris", "type NacosDiscoverer struct", "type PolarisDiscoverer struct", "type NacosRuleProvider struct", "type PolarisRuleProvider struct", "func InstancesFromNacos", "func InstancesFromPolaris"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("release canary template missing %q", want)
		}
	}
	adapterBody, err := os.ReadFile(wantAdapterPath)
	if err != nil {
		t.Fatalf("read hertz canary adapter: %v", err)
	}
	for _, want := range []string{"func HertzTraffic", "func TrafficFromHertz", "func HertzDecision"} {
		if !strings.Contains(string(adapterBody), want) {
			t.Errorf("hertz canary adapter missing %q", want)
		}
	}
	joined := strings.Join(res.NextSteps, "\n")
	if !strings.Contains(joined, "go mod tidy") {
		t.Errorf("next steps missing go mod tidy in:\n%s", joined)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindReleaseCanary {
		t.Errorf("manifest.Infra = %v, want [release_canary]", m.Infra)
	}
}

func TestAddCanaryAliasRecordsCanonicalKind(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindCanaryAlias})
	if err != nil {
		t.Fatalf("Add canary alias: %v", err)
	}
	wantPath := filepath.Join(root, "internal", "base", "release", "canary.go")
	if res.WrittenPath != wantPath {
		t.Errorf("WrittenPath = %q, want %q", res.WrittenPath, wantPath)
	}
	wantAdapterPath := filepath.Join(root, "internal", "base", "release", "kitex.go")
	if strings.Join(res.WrittenPaths, "\n") != strings.Join([]string{wantPath, wantAdapterPath}, "\n") {
		t.Errorf("WrittenPaths = %v, want [%s %s]", res.WrittenPaths, wantPath, wantAdapterPath)
	}
	adapterBody, err := os.ReadFile(wantAdapterPath)
	if err != nil {
		t.Fatalf("read kitex canary adapter: %v", err)
	}
	for _, want := range []string{"func KitexTraffic", "func TrafficFromKitex", "func InjectKitexTraffic", "type KitexCanaryLoadBalancer struct", "func NewKitexCanaryLoadBalancer", "type KitexResultDiscoverer struct"} {
		if !strings.Contains(string(adapterBody), want) {
			t.Errorf("kitex canary adapter missing %q", want)
		}
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 1 || m.Infra[0] != KindReleaseCanary {
		t.Errorf("manifest.Infra = %v, want [release_canary]", m.Infra)
	}
}

func TestAddLoggingWireForHertz(t *testing.T) {
	root := seedProject(t, nil)
	writeHertzServer(t, root)
	res, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true})
	if err != nil {
		t.Fatalf("Add logging --wire: %v", err)
	}
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	if strings.Join(res.WiredPaths, "\n") != serverPath {
		t.Fatalf("WiredPaths = %v, want [%s]", res.WiredPaths, serverPath)
	}
	body := readFile(t, serverPath)
	for _, want := range []string{
		`"github.com/x/demo/internal/base/logging"`,
		"logCfg := logging.Config{",
		"Enabled:   cfg.Logging.Enabled,",
		"Track:       cfg.Release.Info.Track,",
		"GitSHA:      cfg.Release.Info.GitSHA,",
		"BuildTime:   cfg.Release.Info.BuildTime,",
		"h.Use(logging.HertzRecovery())",
		"h.Use(logging.HertzRequestID())",
		"h.Use(logging.HertzAccessLog())",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("hertz logging wiring missing %q\n---\n%s", want, body)
		}
	}
	if strings.Contains(body, "h.Use(middleware.AccessLog())") {
		t.Errorf("default access log should be replaced\n---\n%s", body)
	}
	res, err = Add(Options{Root: root, Kind: KindLoggingAlias, Force: true, Wire: true})
	if err != nil {
		t.Fatalf("Add logging --wire again: %v", err)
	}
	if len(res.WiredPaths) != 0 {
		t.Errorf("second --wire should be idempotent, got %v", res.WiredPaths)
	}
}

func TestAddLoggingWireDryRunForHertzDoesNotWrite(t *testing.T) {
	root := seedProject(t, nil)
	writeHertzServer(t, root)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	originalServer := readFile(t, serverPath)

	res, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true, DryRun: true})
	if err != nil {
		t.Fatalf("Add logging --wire --dry-run: %v", err)
	}
	if !res.DryRun {
		t.Fatalf("DryRun = false, want true")
	}
	wantWritten := []string{
		filepath.Join(root, "internal", "base", "logging", "logging.go"),
		filepath.Join(root, "internal", "base", "logging", "hertz.go"),
	}
	if strings.Join(res.WrittenPaths, "\n") != strings.Join(wantWritten, "\n") {
		t.Fatalf("WrittenPaths = %v, want %v", res.WrittenPaths, wantWritten)
	}
	if strings.Join(res.WiredPaths, "\n") != serverPath {
		t.Fatalf("WiredPaths = %v, want [%s]", res.WiredPaths, serverPath)
	}
	if !planContains(res.Plan, "wire", "update", serverPath, "") {
		t.Fatalf("Plan missing wire update for %s: %+v", serverPath, res.Plan)
	}
	for _, want := range []struct {
		action string
		detail string
	}{
		{action: "add_import", detail: "github.com/x/demo/internal/base/logging"},
		{action: "insert_logging_init", detail: "logging.Init"},
		{action: "replace_middleware", detail: "hertz recovery"},
		{action: "replace_middleware", detail: "hertz request id"},
		{action: "replace_middleware", detail: "hertz access log"},
	} {
		if !planContains(res.Plan, "wire", want.action, serverPath, want.detail) {
			t.Fatalf("Plan missing %s/%s for %s: %+v", want.action, want.detail, serverPath, res.Plan)
		}
	}
	if !planContainsAnchor(res.Plan, "wire", "insert_logging_init", serverPath, "logging.Init", anchorSourceLegacy, "do.ProvideValue(injector, cfg)") {
		t.Fatalf("Plan missing legacy anchor source for logging init: %+v", res.Plan)
	}
	for _, p := range wantWritten {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s: stat err = %v", p, err)
		}
	}
	if got := readFile(t, serverPath); got != originalServer {
		t.Fatalf("dry-run modified server.go\n--- got ---\n%s\n--- want ---\n%s", got, originalServer)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 0 {
		t.Fatalf("dry-run updated manifest infra = %v, want empty", m.Infra)
	}
}

func TestAddLoggingWirePreflightFailureDoesNotWrite(t *testing.T) {
	root := seedProject(t, nil)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	body := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/samber/do/v2"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	injector := do.New()
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	do.ProvideValue(injector, cfg)
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	writeTestFile(t, serverPath, body)

	_, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true})
	if err == nil || !strings.Contains(err.Error(), "h.Use(middleware.AccessLog())") {
		t.Fatalf("err = %v, want missing access log anchor", err)
	}
	for _, p := range []string{
		filepath.Join(root, "internal", "base", "logging", "logging.go"),
		filepath.Join(root, "internal", "base", "logging", "hertz.go"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("preflight failure wrote %s: stat err = %v", p, err)
		}
	}
	if got := readFile(t, serverPath); got != body {
		t.Fatalf("preflight failure modified server.go\n--- got ---\n%s\n--- want ---\n%s", got, body)
	}
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if len(m.Infra) != 0 {
		t.Fatalf("preflight failure updated manifest infra = %v, want empty", m.Infra)
	}
}

func TestAddCanaryWireForHertz(t *testing.T) {
	root := seedProject(t, nil)
	writeHertzServer(t, root)
	_, err := Add(Options{Root: root, Kind: KindCanaryAlias, Wire: true})
	if err != nil {
		t.Fatalf("Add canary --wire: %v", err)
	}
	body := readFile(t, filepath.Join(root, "internal", "base", "server", "server.go"))
	for _, want := range []string{`"github.com/x/demo/internal/base/release"`, "if cfg.Release.Enabled {", "h.Use(release.HertzTraffic())"} {
		if !strings.Contains(body, want) {
			t.Errorf("hertz canary wiring missing %q\n---\n%s", want, body)
		}
	}
}

func TestAddCanaryWireForHertzUsesMarkerAnchor(t *testing.T) {
	root := seedProject(t, nil)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	body := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	h := server.Default()
	// ncgo:wire:canary:server-traffic
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	writeTestFile(t, serverPath, body)

	res, err := Add(Options{Root: root, Kind: KindCanaryAlias, Wire: true})
	if err != nil {
		t.Fatalf("Add hertz canary --wire with marker: %v", err)
	}
	if !planContainsAnchor(res.Plan, "wire", "insert_traffic_middleware", serverPath, "release.HertzTraffic", anchorSourceMarker, markerCanaryServerTraffic) {
		t.Fatalf("Plan missing marker anchor source for canary traffic: %+v", res.Plan)
	}
	got := readFile(t, serverPath)
	if !strings.Contains(got, "h.Use(release.HertzTraffic())") {
		t.Fatalf("marker anchor did not insert canary traffic\n---\n%s", got)
	}
	if strings.Index(got, "h.Use(release.HertzTraffic())") > strings.Index(got, "h.Use(middleware.AccessLog())") {
		t.Fatalf("canary traffic should be inserted before access log\n---\n%s", got)
	}
}

func TestAddLoggingWireForKitexServerAndClient(t *testing.T) {
	root := seedKitexProject(t, nil)
	writeKitexServer(t, root)
	writeKitexClient(t, root)
	res, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true})
	if err != nil {
		t.Fatalf("Add kitex logging --wire: %v", err)
	}
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	clientPath := filepath.Join(root, "pkg", "client", "demo", "client.go")
	if strings.Join(res.WiredPaths, "\n") != strings.Join([]string{serverPath, clientPath}, "\n") {
		t.Fatalf("WiredPaths = %v, want [%s %s]", res.WiredPaths, serverPath, clientPath)
	}
	serverBody := readFile(t, serverPath)
	for _, want := range []string{
		`"github.com/x/demo/internal/base/logging"`,
		"logging.Init(logging.DefaultConfig()",
		"logging.KitexRequestID(),",
		"logging.KitexAccessLog(),",
		"logging.KitexRecovery(),",
	} {
		if !strings.Contains(serverBody, want) {
			t.Errorf("kitex server logging wiring missing %q\n---\n%s", want, serverBody)
		}
	}
	clientBody := readFile(t, clientPath)
	for _, want := range []string{`"github.com/x/demo/internal/base/logging"`, "logging.KitexRequestID()", "logging.KitexAccessLog()"} {
		if !strings.Contains(clientBody, want) {
			t.Errorf("kitex client logging wiring missing %q\n---\n%s", want, clientBody)
		}
	}
}

func TestAddLoggingWireDryRunForKitexServerAndClientDoesNotWrite(t *testing.T) {
	root := seedKitexProject(t, nil)
	writeKitexServer(t, root)
	writeKitexClient(t, root)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	clientPath := filepath.Join(root, "pkg", "client", "demo", "client.go")
	originalServer := readFile(t, serverPath)
	originalClient := readFile(t, clientPath)

	res, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true, DryRun: true})
	if err != nil {
		t.Fatalf("Add kitex logging --wire --dry-run: %v", err)
	}
	wantWritten := []string{
		filepath.Join(root, "internal", "base", "logging", "logging.go"),
		filepath.Join(root, "internal", "base", "logging", "kitex.go"),
	}
	if strings.Join(res.WrittenPaths, "\n") != strings.Join(wantWritten, "\n") {
		t.Fatalf("WrittenPaths = %v, want %v", res.WrittenPaths, wantWritten)
	}
	if strings.Join(res.WiredPaths, "\n") != strings.Join([]string{serverPath, clientPath}, "\n") {
		t.Fatalf("WiredPaths = %v, want [%s %s]", res.WiredPaths, serverPath, clientPath)
	}
	for _, p := range wantWritten {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s: stat err = %v", p, err)
		}
	}
	if got := readFile(t, serverPath); got != originalServer {
		t.Fatalf("dry-run modified kitex server.go\n--- got ---\n%s", got)
	}
	if got := readFile(t, clientPath); got != originalClient {
		t.Fatalf("dry-run modified kitex client.go\n--- got ---\n%s", got)
	}
	if !planContains(res.Plan, "file", "create", wantWritten[0], "") || !planContains(res.Plan, "manifest", "add", filepath.Join(".ncgo", "manifest.yaml"), "") {
		t.Fatalf("Plan missing file/manifest items: %+v", res.Plan)
	}
	for _, p := range []string{serverPath, clientPath} {
		if !planContains(res.Plan, "wire", "update", p, "") {
			t.Fatalf("Plan missing wire update for %s: %+v", p, res.Plan)
		}
	}
	if !planContains(res.Plan, "wire", "insert_client_middleware", clientPath, "logging.KitexAccessLog") {
		t.Fatalf("Plan missing kitex logging client middleware insert: %+v", res.Plan)
	}
	assertManifestInfra(t, root)
}

func TestAddCanaryWireForKitexServerAndClient(t *testing.T) {
	root := seedKitexProject(t, nil)
	writeKitexServer(t, root)
	writeKitexClient(t, root)
	_, err := Add(Options{Root: root, Kind: KindCanaryAlias, Wire: true})
	if err != nil {
		t.Fatalf("Add kitex canary --wire: %v", err)
	}
	serverBody := readFile(t, filepath.Join(root, "internal", "base", "server", "server.go"))
	clientBody := readFile(t, filepath.Join(root, "pkg", "client", "demo", "client.go"))
	for _, body := range []string{serverBody, clientBody} {
		for _, want := range []string{`"github.com/x/demo/internal/base/release"`, "release.KitexTraffic()"} {
			if !strings.Contains(body, want) {
				t.Errorf("kitex canary wiring missing %q\n---\n%s", want, body)
			}
		}
	}
}

func TestAddCanaryWireDryRunForKitexServerAndClientDoesNotWrite(t *testing.T) {
	root := seedKitexProject(t, nil)
	writeKitexServer(t, root)
	writeKitexClient(t, root)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	clientPath := filepath.Join(root, "pkg", "client", "demo", "client.go")
	originalServer := readFile(t, serverPath)
	originalClient := readFile(t, clientPath)

	res, err := Add(Options{Root: root, Kind: KindCanaryAlias, Wire: true, DryRun: true})
	if err != nil {
		t.Fatalf("Add kitex canary --wire --dry-run: %v", err)
	}
	wantWritten := []string{
		filepath.Join(root, "internal", "base", "release", "canary.go"),
		filepath.Join(root, "internal", "base", "release", "kitex.go"),
	}
	if strings.Join(res.WrittenPaths, "\n") != strings.Join(wantWritten, "\n") {
		t.Fatalf("WrittenPaths = %v, want %v", res.WrittenPaths, wantWritten)
	}
	if strings.Join(res.WiredPaths, "\n") != strings.Join([]string{serverPath, clientPath}, "\n") {
		t.Fatalf("WiredPaths = %v, want [%s %s]", res.WiredPaths, serverPath, clientPath)
	}
	for _, p := range wantWritten {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s: stat err = %v", p, err)
		}
	}
	if got := readFile(t, serverPath); got != originalServer {
		t.Fatalf("dry-run modified kitex server.go\n--- got ---\n%s", got)
	}
	if got := readFile(t, clientPath); got != originalClient {
		t.Fatalf("dry-run modified kitex client.go\n--- got ---\n%s", got)
	}
	for _, p := range []string{serverPath, clientPath} {
		if !planContains(res.Plan, "wire", "update", p, "") {
			t.Fatalf("Plan missing wire update for %s: %+v", p, res.Plan)
		}
	}
	if !planContains(res.Plan, "wire", "insert_traffic_middleware", serverPath, "release.KitexTraffic") {
		t.Fatalf("Plan missing kitex canary server traffic middleware insert: %+v", res.Plan)
	}
	if !planContains(res.Plan, "wire", "insert_client_middleware", clientPath, "release.KitexTraffic") {
		t.Fatalf("Plan missing kitex canary client middleware insert: %+v", res.Plan)
	}
	assertManifestInfra(t, root)
}

func TestAddKitexWireDryRunWithoutClientsReportsServerOnly(t *testing.T) {
	root := seedKitexProject(t, nil)
	writeKitexServer(t, root)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")

	res, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true, DryRun: true})
	if err != nil {
		t.Fatalf("Add kitex logging --wire --dry-run without clients: %v", err)
	}
	if strings.Join(res.WiredPaths, "\n") != serverPath {
		t.Fatalf("WiredPaths = %v, want [%s]", res.WiredPaths, serverPath)
	}
	if !planContains(res.Plan, "wire", "update", serverPath, "") {
		t.Fatalf("Plan missing server wire update: %+v", res.Plan)
	}
	assertManifestInfra(t, root)
}

func TestAddKitexWireClientPreflightFailureDoesNotWrite(t *testing.T) {
	root := seedKitexProject(t, nil)
	writeKitexServer(t, root)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	clientPath := filepath.Join(root, "pkg", "client", "demo", "client.go")
	badClient := `package democlient

import (
	"context"

	kitexclient "github.com/cloudwego/kitex/client"
)

type Config struct {
	EnableMetaInfo bool
}

func New(ctx context.Context, cfg Config, opts ...kitexclient.Option) {
	_ = ctx
	_ = cfg
	options := append([]kitexclient.Option{}, opts...)
	_ = options
}
`
	writeTestFile(t, clientPath, badClient)
	originalServer := readFile(t, serverPath)

	_, err := Add(Options{Root: root, Kind: KindLoggingAlias, Wire: true})
	if err == nil || !strings.Contains(err.Error(), "could not find insertion anchor") {
		t.Fatalf("err = %v, want missing client insertion anchor", err)
	}
	for _, p := range []string{
		filepath.Join(root, "internal", "base", "logging", "logging.go"),
		filepath.Join(root, "internal", "base", "logging", "kitex.go"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("preflight failure wrote %s: stat err = %v", p, err)
		}
	}
	if got := readFile(t, serverPath); got != originalServer {
		t.Fatalf("preflight failure modified server.go\n--- got ---\n%s", got)
	}
	if got := readFile(t, clientPath); got != badClient {
		t.Fatalf("preflight failure modified client.go\n--- got ---\n%s", got)
	}
	assertManifestInfra(t, root)
}

func TestAddWireRejectsUnsupportedKind(t *testing.T) {
	root := seedProject(t, nil)
	writeHertzServer(t, root)
	_, err := Add(Options{Root: root, Kind: KindRedis, Wire: true})
	if err == nil || !strings.Contains(err.Error(), "--wire is only supported") {
		t.Fatalf("err = %v, want unsupported --wire error", err)
	}
}

func TestAddKitexOnlyRejectedForHertz(t *testing.T) {
	root := seedProject(t, nil)
	_, err := Add(Options{Root: root, Kind: KindRegistryEtcd})
	if err == nil {
		t.Fatalf("expected registry_etcd to be rejected for hertz")
	}
	if !strings.Contains(err.Error(), "only supported for kitex") {
		t.Errorf("unexpected error: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func planContains(plan []PlanItem, kind, action, path, detail string) bool {
	for _, item := range plan {
		if item.Kind == kind && item.Action == action && item.Path == path && item.Detail == detail {
			return true
		}
	}
	return false
}

func planContainsAnchor(plan []PlanItem, kind, action, path, detail, anchorSource, anchorSnippet string) bool {
	for _, item := range plan {
		if item.Kind == kind && item.Action == action && item.Path == path && item.Detail == detail && item.AnchorSource == anchorSource && strings.Contains(item.Anchor, anchorSnippet) {
			return true
		}
	}
	return false
}

func sliceContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func assertManifestInfra(t *testing.T, root string, want ...string) {
	t.Helper()
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatalf("reload manifest: %v", err)
	}
	if strings.Join(m.Infra, "\n") != strings.Join(want, "\n") {
		t.Fatalf("manifest.Infra = %v, want %v", m.Infra, want)
	}
}

func writeHertzServer(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body := `package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/samber/do/v2"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/middleware"
)

func Run() {
	injector := do.New()
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	do.ProvideValue(injector, cfg)
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
`
	writeTestFile(t, path, body)
}

func writeKitexServer(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body := `package server

import (
	"context"
	"log"

	"github.com/cloudwego/kitex/pkg/endpoint"
	kitexserver "github.com/cloudwego/kitex/server"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/interceptor"
)

func Run() {
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	_ = log.Flags()
	opts := []kitexserver.Option{
		kitexserver.WithMiddleware(endpoint.Chain(
			interceptor.RequestID(),
			interceptor.AccessLog(),
			interceptor.Recovery(),
			interceptor.RequestTimeout(0),
		)),
		kitexserver.WithErrorHandler(func(ctx context.Context, err error) error { return err }),
	}
	_ = opts
}
`
	writeTestFile(t, path, body)
}

func writeKitexClient(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "pkg", "client", "demo", "client.go")
	body := `package democlient

import (
	"context"

	kitexclient "github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/transmeta"
)

type Config struct {
	EnableMetaInfo bool
}

func New(ctx context.Context, cfg Config, opts ...kitexclient.Option) {
	_ = ctx
	options := make([]kitexclient.Option, 0, len(opts)+6)
	if cfg.EnableMetaInfo {
		options = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))
	}
	options = append(options, opts...)
	_ = options
}

func callerServiceMiddleware(caller string) endpoint.Middleware {
	_ = caller
	return func(next endpoint.Endpoint) endpoint.Endpoint { return next }
}
`
	writeTestFile(t, path, body)
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestAddAllSupportedKindsCopySuccessfully(t *testing.T) {
	for _, kind := range commonKinds() {
		t.Run(kind, func(t *testing.T) {
			root := seedProject(t, nil)
			res, err := Add(Options{Root: root, Kind: kind})
			if err != nil {
				t.Fatalf("Add %s: %v", kind, err)
			}
			body, err := os.ReadFile(res.WrittenPath)
			if err != nil {
				t.Fatalf("read %s: %v", kind, err)
			}
			if !strings.Contains(string(body), "package ") {
				t.Errorf("%s missing package declaration", kind)
			}
		})
	}
	for _, kind := range kitexOnlyKinds() {
		t.Run(kind, func(t *testing.T) {
			root := seedKitexProject(t, nil)
			res, err := Add(Options{Root: root, Kind: kind})
			if err != nil {
				t.Fatalf("Add %s: %v", kind, err)
			}
			body, err := os.ReadFile(res.WrittenPath)
			if err != nil {
				t.Fatalf("read %s: %v", kind, err)
			}
			if !strings.Contains(string(body), "package ") {
				t.Errorf("%s missing package declaration", kind)
			}
		})
	}
}

func pathListed(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}
