# infra Plugin Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the switch/map-driven organization of `internal/scaffold/infra/infra.go` and `internal/scaffold/infra/wire.go` with an interface-based `Plugin` registry, with zero change to generated output, golden tests, or exported API.

**Architecture:** Each infra kind becomes a concrete type implementing `Plugin` (required methods) plus optional capability interfaces (`extraFilesPlugin`, `hertzServerWirer`, `kitexServerWirer`, `kitexClientWirer`) detected via type assertion. Plugins self-register in `init()`. Tasks 1-10 are purely additive (new files, new plugin types, isolated characterization tests) — the existing `Add()`/`Wire()` dispatch is untouched and all existing tests keep passing throughout. Task 11 is the single atomic cutover that rewires dispatch to the registry and deletes the now-dead legacy maps/switches. Task 12 is final full-repo validation.

**Tech Stack:** Go 1.25, no new dependencies.

**Spec:** `docs/superpowers/specs/2026-09-04-infra-plugin-registry-design.md`

## Global Constraints

- Pure refactor: `internal/scaffold/infra/testdata/**` golden fixtures must be byte-identical before and after (verified via `go test ./internal/scaffold/infra/... -count=1`).
- Exported symbols must not change: `infra.KindRedis` / `KindKafka` / `KindES` / `KindClickHouse` / `KindRegistryPolaris` / `KindObservabilityLog` / `KindLoggingAlias` / `KindReleaseCanary` / `KindCanaryAlias` / `KindRateLimit` / `KindRateLimitAlias` / `KindPolarisAdapter` / `KindPolarisAdapterAlias`, `SupportedKinds()`, `Add()`, `Wire()`, `PreviewWire()`, `PreviewWirePlan()`, `Options`, `Result`, `PlanItem` (alias of `planpkg.Item`).
- `SupportedKinds()` output order must stay exactly: `[redis, kafka, es, clickhouse, observability_logging, logging, release_canary, canary, registry_polaris, rate_limit, rate-limit, polaris_adapter, polaris-adapter]`.
- `unsupportedWireError()` message text must stay exactly: `"infra: --wire is only supported for observability_logging/release_canary/registry_polaris/rate_limit"`.
- No changes to `internal/assets/_data/**` template content.

---

### Task 1: Plugin interfaces + registry framework

**Files:**
- Modify: `internal/scaffold/infra/infra.go` (append new section after the existing `const` block, before `SupportedKinds()`)
- Test: `internal/scaffold/infra/registry_test.go` (new)

**Interfaces:**
- Produces: `type Plugin interface { Kind() string; Aliases() []string; ServiceScope() string; GoGetDeps() []string; SetupSteps() []string; HertzConfigKey() string; AssetFiles(serviceKind string) ([]addOnFile, error) }`, optional `extraFilesPlugin { ExtraFiles(root, serviceKind string) ([]addOnFile, error) }`, `hertzServerWirer { WireHertzServer(src, module string, plan *[]PlanItem) (string, error) }`, `kitexServerWirer { WireKitexServer(src, module string, plan *[]PlanItem) (string, error) }`, `kitexClientWirer { WireKitexClient(src, module string, plan *[]PlanItem) (string, error) }`, `func Register(p Plugin)`, `func pluginByKind(kind string) (Plugin, bool)`.
- Consumes: `addOnFile` (already defined in `infra.go`), `PlanItem` (already defined).

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/registry_test.go
package infra

import "testing"

type fakePlugin struct {
	kind    string
	aliases []string
}

func (f fakePlugin) Kind() string                                            { return f.kind }
func (f fakePlugin) Aliases() []string                                       { return f.aliases }
func (f fakePlugin) ServiceScope() string                                    { return "common" }
func (f fakePlugin) GoGetDeps() []string                                     { return nil }
func (f fakePlugin) SetupSteps() []string                                    { return nil }
func (f fakePlugin) HertzConfigKey() string                                  { return "" }
func (f fakePlugin) AssetFiles(serviceKind string) ([]addOnFile, error)      { return nil, nil }

func TestRegisterAndLookupByKindAndAlias(t *testing.T) {
	reg := newRegistry()
	reg.register(fakePlugin{kind: "widget", aliases: []string{"widget-alias"}})

	p, ok := reg.byKind("widget")
	if !ok || p.Kind() != "widget" {
		t.Fatalf("byKind(widget) = %v, %v; want widget plugin", p, ok)
	}
	p, ok = reg.byKind("widget-alias")
	if !ok || p.Kind() != "widget" {
		t.Fatalf("byKind(widget-alias) = %v, %v; want widget plugin via alias", p, ok)
	}
	if _, ok := reg.byKind("missing"); ok {
		t.Fatalf("byKind(missing) = ok=true; want ok=false")
	}
}

func TestRegisterDuplicateKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Kind() registration")
		}
	}()
	reg := newRegistry()
	reg.register(fakePlugin{kind: "widget"})
	reg.register(fakePlugin{kind: "widget"})
}

func TestRegisterDuplicateAliasPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate alias registration")
		}
	}()
	reg := newRegistry()
	reg.register(fakePlugin{kind: "a", aliases: []string{"shared"}})
	reg.register(fakePlugin{kind: "b", aliases: []string{"shared"}})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run 'TestRegisterAndLookupByKindAndAlias|TestRegisterDuplicateKindPanics|TestRegisterDuplicateAliasPanics' -v -count=1`
Expected: FAIL — `newRegistry`/`reg.register`/`reg.byKind` undefined.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/scaffold/infra/infra.go`, immediately after the existing `const ( KindRedis = ... )` block (do not touch anything else yet):

```go
// Plugin describes one `ncgo add infra <kind>` add-on. Concrete plugin types
// live in plugin_<kind>.go and self-register via Register() in an init().
type Plugin interface {
	Kind() string
	Aliases() []string
	// ServiceScope reports which manifest.Kind values this plugin supports:
	// "common" (both hertz and kitex), "hertz", or "kitex".
	ServiceScope() string
	GoGetDeps() []string
	// SetupSteps returns an explicit next-steps override, or nil to derive
	// the default (go get GoGetDeps + hertz config note + "go mod tidy").
	SetupSteps() []string
	// HertzConfigKey returns the conf/dev/conf.yaml top-level key this
	// plugin's Hertz config snippet writes under, or "" if it has none.
	HertzConfigKey() string
	AssetFiles(serviceKind string) ([]addOnFile, error)
}

// extraFilesPlugin is an optional capability: plugins that need to append
// files conditioned on project state (e.g. redis's Hertz shared helper)
// implement it.
type extraFilesPlugin interface {
	ExtraFiles(root, serviceKind string) ([]addOnFile, error)
}

// hertzServerWirer, kitexServerWirer, kitexClientWirer are optional
// capabilities for --wire support. A plugin implements only the hooks
// relevant to it; the absence of an interface means that wire target is a
// no-op for that kind, matching the current switch statements having no
// case for it.
type hertzServerWirer interface {
	WireHertzServer(src, module string, plan *[]PlanItem) (string, error)
}

type kitexServerWirer interface {
	WireKitexServer(src, module string, plan *[]PlanItem) (string, error)
}

type kitexClientWirer interface {
	WireKitexClient(src, module string, plan *[]PlanItem) (string, error)
}

// pluginRegistry is a lookup table from kind or alias to Plugin. The zero
// value is not usable; construct with newRegistry.
type pluginRegistry struct {
	byName map[string]Plugin // canonical Kind() -> Plugin
	alias  map[string]string // alias -> canonical Kind()
}

func newRegistry() *pluginRegistry {
	return &pluginRegistry{byName: map[string]Plugin{}, alias: map[string]string{}}
}

func (r *pluginRegistry) register(p Plugin) {
	if _, exists := r.byName[p.Kind()]; exists {
		panic(fmt.Sprintf("infra: duplicate plugin registration for kind %q", p.Kind()))
	}
	r.byName[p.Kind()] = p
	for _, a := range p.Aliases() {
		if _, exists := r.alias[a]; exists {
			panic(fmt.Sprintf("infra: duplicate plugin alias registration for %q", a))
		}
		r.alias[a] = p.Kind()
	}
}

func (r *pluginRegistry) byKind(kindOrAlias string) (Plugin, bool) {
	if p, ok := r.byName[kindOrAlias]; ok {
		return p, true
	}
	if canonical, ok := r.alias[kindOrAlias]; ok {
		p, ok := r.byName[canonical]
		return p, ok
	}
	return nil, false
}

var pluginRegistryInstance = newRegistry()

// Register adds a plugin to the package-level registry. Called from each
// plugin file's init(). Panics on duplicate Kind()/alias — the plugin set
// is closed and known at compile time, so a collision is a programming
// error, not a runtime condition to recover from.
func Register(p Plugin) { pluginRegistryInstance.register(p) }

func pluginByKind(kind string) (Plugin, bool) { return pluginRegistryInstance.byKind(kind) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run 'TestRegisterAndLookupByKindAndAlias|TestRegisterDuplicateKindPanics|TestRegisterDuplicateAliasPanics' -v -count=1`
Expected: PASS (3/3).

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS — this task is purely additive, nothing existing changed behavior.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/registry_test.go
git commit -m "refactor(scaffold): add Plugin interface and registry framework for infra add-ons"
```

---

### Task 2: `redis` plugin

**Files:**
- Create: `internal/scaffold/infra/plugin_redis.go`
- Test: `internal/scaffold/infra/plugin_redis_test.go`

**Interfaces:**
- Consumes: `Plugin`, `extraFilesPlugin`, `Register`, `addOnFile`, `manifest.KindHertz`/`manifest.KindKitex` (from Task 1 and existing `infra.go`).
- Produces: `redisPlugin` type registered under `Register()` for `KindRedis`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_redis_test.go
package infra

import (
	"reflect"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestRedisPluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindRedis)
	if !ok {
		t.Fatal("redis plugin not registered")
	}
	if p.Kind() != KindRedis {
		t.Errorf("Kind() = %q, want %q", p.Kind(), KindRedis)
	}
	if len(p.Aliases()) != 0 {
		t.Errorf("Aliases() = %v, want empty", p.Aliases())
	}
	if p.ServiceScope() != "common" {
		t.Errorf("ServiceScope() = %q, want common", p.ServiceScope())
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.SetupSteps() != nil {
		t.Errorf("SetupSteps() = %v, want nil (default derivation)", p.SetupSteps())
	}
	if p.HertzConfigKey() != "redis" {
		t.Errorf("HertzConfigKey() = %q, want redis", p.HertzConfigKey())
	}
}

func TestRedisPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindRedis)
	hertzFiles, err := p.AssetFiles(manifest.KindHertz)
	if err != nil {
		t.Fatalf("AssetFiles(hertz): %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/redis.go", OutputRelPath: "internal/base/data/redis.go"}}
	if !reflect.DeepEqual(hertzFiles, want) {
		t.Errorf("AssetFiles(hertz) = %+v, want %+v", hertzFiles, want)
	}
	kitexFiles, err := p.AssetFiles(manifest.KindKitex)
	if err != nil {
		t.Fatalf("AssetFiles(kitex): %v", err)
	}
	want = []addOnFile{{SourcePath: "kitex/optional/redis.go", OutputRelPath: "internal/base/data/redis.go"}}
	if !reflect.DeepEqual(kitexFiles, want) {
		t.Errorf("AssetFiles(kitex) = %+v, want %+v", kitexFiles, want)
	}
}

func TestRedisPluginExtraFilesAddsHertzSharedHelperOnce(t *testing.T) {
	p, _ := pluginByKind(KindRedis)
	ep, ok := p.(extraFilesPlugin)
	if !ok {
		t.Fatal("redis plugin must implement extraFilesPlugin")
	}
	root := seedProject(t, nil)
	files, err := ep.ExtraFiles(root, manifest.KindHertz)
	if err != nil {
		t.Fatalf("ExtraFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/redis_shared.go", OutputRelPath: "internal/base/data/redis_shared.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("ExtraFiles = %+v, want %+v", files, want)
	}
	files, err = ep.ExtraFiles(root, manifest.KindKitex)
	if err != nil {
		t.Fatalf("ExtraFiles(kitex): %v", err)
	}
	if len(files) != 0 {
		t.Errorf("ExtraFiles(kitex) = %+v, want empty (kitex has no shared redis helper)", files)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestRedisPlugin -v -count=1`
Expected: FAIL — `pluginByKind(KindRedis)` returns `ok=false` (no plugin registered yet).

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_redis.go
package infra

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(redisPlugin{}) }

type redisPlugin struct{}

func (redisPlugin) Kind() string        { return KindRedis }
func (redisPlugin) Aliases() []string   { return nil }
func (redisPlugin) ServiceScope() string { return "common" }
func (redisPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common"}
}
func (redisPlugin) SetupSteps() []string   { return nil }
func (redisPlugin) HertzConfigKey() string { return "redis" }

func (redisPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindRedis, "internal/base/data/redis.go")(serviceKind)
}

// ExtraFiles appends the Hertz redis shared helper if it is not already
// present in the project (kitex has no equivalent shared helper).
func (redisPlugin) ExtraFiles(root, serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindHertz {
		return nil, nil
	}
	helperPath := filepath.Join(root, filepath.FromSlash(hertzRedisSharedHelperRelPath))
	if _, err := os.Stat(helperPath); err == nil {
		return nil, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return []addOnFile{{SourcePath: "hertz/optional/redis_shared.go", OutputRelPath: filepath.FromSlash(hertzRedisSharedHelperRelPath)}}, nil
}
```

Also append this shared helper to `internal/scaffold/infra/infra.go` (used by all four simple plugins in Tasks 2-5; write it once here):

```go
// frameworkAssetFiles builds an AssetFiles implementation for plugins whose
// asset is a single file living under hertz/optional/ or kitex/optional/
// depending on serviceKind, written to a fixed outputRelPath.
func frameworkAssetFiles(infraKind, outputRelPath string) func(serviceKind string) ([]addOnFile, error) {
	return func(serviceKind string) ([]addOnFile, error) {
		switch serviceKind {
		case manifest.KindHertz, manifest.KindKitex:
			return []addOnFile{{SourcePath: serviceKind + "/optional/" + infraKind + ".go", OutputRelPath: outputRelPath}}, nil
		default:
			return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestRedisPlugin -v -count=1`
Expected: PASS (3/3).

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/plugin_redis.go internal/scaffold/infra/plugin_redis_test.go
git commit -m "refactor(scaffold): extract redis infra add-on to Plugin type"
```

---

### Task 3: `kafka` plugin

**Files:**
- Create: `internal/scaffold/infra/plugin_kafka.go`
- Test: `internal/scaffold/infra/plugin_kafka_test.go`

**Interfaces:**
- Consumes: `frameworkAssetFiles` (Task 2), `Register`, `Plugin`.
- Produces: `kafkaPlugin` registered under `KindKafka`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_kafka_test.go
package infra

import (
	"reflect"
	"testing"
)

func TestKafkaPluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindKafka)
	if !ok {
		t.Fatal("kafka plugin not registered")
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "kafka" {
		t.Errorf("HertzConfigKey() = %q, want kafka", p.HertzConfigKey())
	}
	if p.ServiceScope() != "common" {
		t.Errorf("ServiceScope() = %q, want common", p.ServiceScope())
	}
}

func TestKafkaPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindKafka)
	files, err := p.AssetFiles("hertz")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/kafka.go", OutputRelPath: "internal/base/data/kafka.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestKafkaPlugin -v -count=1`
Expected: FAIL — kafka plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_kafka.go
package infra

func init() { Register(kafkaPlugin{}) }

type kafkaPlugin struct{}

func (kafkaPlugin) Kind() string        { return KindKafka }
func (kafkaPlugin) Aliases() []string   { return nil }
func (kafkaPlugin) ServiceScope() string { return "common" }
func (kafkaPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
}
func (kafkaPlugin) SetupSteps() []string   { return nil }
func (kafkaPlugin) HertzConfigKey() string { return "kafka" }
func (kafkaPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindKafka, "internal/base/data/kafka.go")(serviceKind)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestKafkaPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_kafka.go internal/scaffold/infra/plugin_kafka_test.go
git commit -m "refactor(scaffold): extract kafka infra add-on to Plugin type"
```

---

### Task 4: `es` plugin

**Files:**
- Create: `internal/scaffold/infra/plugin_es.go`
- Test: `internal/scaffold/infra/plugin_es_test.go`

**Interfaces:**
- Consumes: `frameworkAssetFiles` (Task 2), `Register`, `Plugin`.
- Produces: `esPlugin` registered under `KindES`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_es_test.go
package infra

import (
	"reflect"
	"testing"
)

func TestESPluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindES)
	if !ok {
		t.Fatal("es plugin not registered")
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "es" {
		t.Errorf("HertzConfigKey() = %q, want es", p.HertzConfigKey())
	}
}

func TestESPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindES)
	files, err := p.AssetFiles("kitex")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "kitex/optional/es.go", OutputRelPath: "internal/base/data/es.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestESPlugin -v -count=1`
Expected: FAIL — es plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_es.go
package infra

func init() { Register(esPlugin{}) }

type esPlugin struct{}

func (esPlugin) Kind() string        { return KindES }
func (esPlugin) Aliases() []string   { return nil }
func (esPlugin) ServiceScope() string { return "common" }
func (esPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
}
func (esPlugin) SetupSteps() []string   { return nil }
func (esPlugin) HertzConfigKey() string { return "es" }
func (esPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindES, "internal/base/data/es.go")(serviceKind)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestESPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_es.go internal/scaffold/infra/plugin_es_test.go
git commit -m "refactor(scaffold): extract es infra add-on to Plugin type"
```

---

### Task 5: `clickhouse` plugin

**Files:**
- Create: `internal/scaffold/infra/plugin_clickhouse.go`
- Test: `internal/scaffold/infra/plugin_clickhouse_test.go`

**Interfaces:**
- Consumes: `frameworkAssetFiles` (Task 2), `Register`, `Plugin`.
- Produces: `clickhousePlugin` registered under `KindClickHouse`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_clickhouse_test.go
package infra

import (
	"reflect"
	"testing"
)

func TestClickHousePluginMetadataMatchesLegacy(t *testing.T) {
	p, ok := pluginByKind(KindClickHouse)
	if !ok {
		t.Fatal("clickhouse plugin not registered")
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "clickhouse" {
		t.Errorf("HertzConfigKey() = %q, want clickhouse", p.HertzConfigKey())
	}
}

func TestClickHousePluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindClickHouse)
	files, err := p.AssetFiles("hertz")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{{SourcePath: "hertz/optional/clickhouse.go", OutputRelPath: "internal/base/data/clickhouse.go"}}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestClickHousePlugin -v -count=1`
Expected: FAIL — clickhouse plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_clickhouse.go
package infra

func init() { Register(clickhousePlugin{}) }

type clickhousePlugin struct{}

func (clickhousePlugin) Kind() string        { return KindClickHouse }
func (clickhousePlugin) Aliases() []string   { return nil }
func (clickhousePlugin) ServiceScope() string { return "common" }
func (clickhousePlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-middleware", "github.com/byx-darwin/go-tools/go-common", "github.com/byx-darwin/go-tools/go-framework"}
}
func (clickhousePlugin) SetupSteps() []string   { return nil }
func (clickhousePlugin) HertzConfigKey() string { return "clickhouse" }
func (clickhousePlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	return frameworkAssetFiles(KindClickHouse, "internal/base/data/clickhouse.go")(serviceKind)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestClickHousePlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_clickhouse.go internal/scaffold/infra/plugin_clickhouse_test.go
git commit -m "refactor(scaffold): extract clickhouse infra add-on to Plugin type"
```

---

### Task 6: `observability_logging` plugin (+ wire hooks)

**Files:**
- Create: `internal/scaffold/infra/plugin_observability_logging.go`
- Test: `internal/scaffold/infra/plugin_observability_logging_test.go`

**Interfaces:**
- Consumes: `frameworkAdapterName` (existing helper in `infra.go`, unchanged), `hertzServerWirer`/`kitexServerWirer`/`kitexClientWirer`, the existing wire primitives `addGoImportWithPlan`/`insertOnceMarkerOrAnchorWithPlan`/`replaceOnceStrictWithPlan`/`hertzLoggingInit`/`kitexLoggingInit`/marker constants (`markerLoggingInit`, `markerKitexClientMiddleware`) — all unchanged in `wire.go`.
- Produces: `observabilityLoggingPlugin` registered under `KindObservabilityLog` with alias `KindLoggingAlias`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_observability_logging_test.go
package infra

import (
	"reflect"
	"strings"
	"testing"
)

func TestObservabilityLoggingPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindObservabilityLog)
	if !ok {
		t.Fatal("observability_logging plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindLoggingAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindLoggingAlias)
	}
	byAlias, ok := pluginByKind(KindLoggingAlias)
	if !ok || byAlias.Kind() != KindObservabilityLog {
		t.Errorf("pluginByKind(%s) = %v, %v; want observability_logging plugin", KindLoggingAlias, byAlias, ok)
	}
	wantDeps := []string{"github.com/byx-darwin/go-tools/go-common"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
	if p.HertzConfigKey() != "" {
		t.Errorf("HertzConfigKey() = %q, want empty", p.HertzConfigKey())
	}
}

func TestObservabilityLoggingPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindObservabilityLog)
	files, err := p.AssetFiles("hertz")
	if err != nil {
		t.Fatalf("AssetFiles(hertz): %v", err)
	}
	want := []addOnFile{
		{SourcePath: "optional/observability_logging.go", OutputRelPath: "internal/base/logging/logging.go"},
		{SourcePath: "hertz/optional/observability_logging.go", OutputRelPath: "internal/base/logging/hertz.go"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles(hertz) = %+v, want %+v", files, want)
	}
}

func TestObservabilityLoggingPluginWiresHertzServer(t *testing.T) {
	p, _ := pluginByKind(KindObservabilityLog)
	w, ok := p.(hertzServerWirer)
	if !ok {
		t.Fatal("observability_logging plugin must implement hertzServerWirer")
	}
	src := "package server\n\nfunc New() {\n\tdo.ProvideValue(injector, cfg)\n\th.Use(middleware.Recovery())\n\th.Use(middleware.RequestID())\n\th.Use(middleware.AccessLog())\n}\n"
	var plan []PlanItem
	out, err := w.WireHertzServer(src, "example.com/mod", &plan)
	if err != nil {
		t.Fatalf("WireHertzServer: %v", err)
	}
	if !containsAll(out, []string{"logging.Init(", "logging.HertzRecovery()", "logging.HertzRequestID()", "logging.HertzAccessLog()"}) {
		t.Errorf("WireHertzServer output missing expected logging wiring: %s", out)
	}
	if len(plan) == 0 {
		t.Error("WireHertzServer should record plan items")
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestObservabilityLoggingPlugin -v -count=1`
Expected: FAIL — plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_observability_logging.go
package infra

import "github.com/byx-darwin/ncgo/internal/manifest"

func init() { Register(observabilityLoggingPlugin{}) }

type observabilityLoggingPlugin struct{}

func (observabilityLoggingPlugin) Kind() string        { return KindObservabilityLog }
func (observabilityLoggingPlugin) Aliases() []string   { return []string{KindLoggingAlias} }
func (observabilityLoggingPlugin) ServiceScope() string { return "common" }
func (observabilityLoggingPlugin) GoGetDeps() []string {
	return []string{"github.com/byx-darwin/go-tools/go-common"}
}
func (observabilityLoggingPlugin) SetupSteps() []string   { return nil }
func (observabilityLoggingPlugin) HertzConfigKey() string { return "" }

func (observabilityLoggingPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	files := []addOnFile{{SourcePath: "optional/" + KindObservabilityLog + ".go", OutputRelPath: "internal/base/logging/logging.go"}}
	switch serviceKind {
	case manifest.KindHertz, manifest.KindKitex:
		files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + KindObservabilityLog + ".go", OutputRelPath: frameworkAdapterName(KindObservabilityLog, serviceKind)})
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
	return files, nil
}

func (observabilityLoggingPlugin) WireHertzServer(s, module string, plan *[]PlanItem) (string, error) {
	var err error
	s, err = addGoImportWithPlan(s, module+"/internal/base/logging", "", plan)
	if err != nil {
		return "", err
	}
	s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.Init(", markerLoggingInit, "\tdo.ProvideValue(injector, cfg)\n", hertzLoggingInit(), "", plan, "insert_logging_init", "logging.Init")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzRecovery())", "h.Use(middleware.Recovery())", "h.Use(logging.HertzRecovery())", "", plan, "hertz recovery")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "h.Use(logging.HertzRequestID())", "h.Use(middleware.RequestID())", "h.Use(logging.HertzRequestID())", "", plan, "hertz request id")
	if err != nil {
		return "", err
	}
	return replaceOnceStrictWithPlan(s, "h.Use(logging.HertzAccessLog())", "h.Use(middleware.AccessLog())", "h.Use(logging.HertzAccessLog())", "", plan, "hertz access log")
}

func (observabilityLoggingPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	var err error
	s, err = addGoImportWithPlan(s, module+"/internal/base/logging", "", plan)
	if err != nil {
		return "", err
	}
	s, err = insertOnceMarkerOrAnchorWithPlan(s, "logging.Init(", markerLoggingInit, "\tif cfg == nil {\n\t\tcfg = conf.Default()\n\t}\n", kitexLoggingInit(), "", plan, "insert_logging_init", "logging.Init")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "logging.KitexRequestID(),", "interceptor.RequestID(),", "logging.KitexRequestID(),", "", plan, "kitex request id")
	if err != nil {
		return "", err
	}
	s, err = replaceOnceStrictWithPlan(s, "logging.KitexAccessLog(),", "interceptor.AccessLog(),", "logging.KitexAccessLog(),", "", plan, "kitex access log")
	if err != nil {
		return "", err
	}
	return replaceOnceStrictWithPlan(s, "logging.KitexRecovery(),", "interceptor.Recovery(),", "logging.KitexRecovery(),", "", plan, "kitex recovery")
}

func (observabilityLoggingPlugin) WireKitexClient(s, module string, plan *[]PlanItem) (string, error) {
	var err error
	s, err = addGoImportWithPlan(s, module+"/internal/base/logging", "", plan)
	if err != nil {
		return "", err
	}
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	loggingBlock := "\toptions = append(options, kitexclient.WithMiddleware(endpoint.Chain(\n\t\tlogging.KitexRequestID(),\n\t\tlogging.KitexAccessLog(),\n\t)))\n"
	return insertOnceMarkerOrAnchorWithPlan(s, "logging.KitexAccessLog()", markerKitexClientMiddleware, anchor, loggingBlock, "", plan, "insert_client_middleware", "logging.KitexAccessLog")
}
```

> Note: the existing `*WithPlan` helpers in `wire.go` take a `path string` parameter used only to build the `PlanItem.Path` field. Task 11 (cutover) is responsible for passing the real `path` through from the caller — in this task's isolated unit test, `""` is acceptable since the test only checks output content and that a plan entry was recorded, not its `Path` field. Do not change the signatures of the existing `*WithPlan` helpers in this task.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestObservabilityLoggingPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_observability_logging.go internal/scaffold/infra/plugin_observability_logging_test.go
git commit -m "refactor(scaffold): extract observability_logging infra add-on to Plugin type"
```

---

### Task 7: `release_canary` plugin (+ wire hooks)

**Files:**
- Create: `internal/scaffold/infra/plugin_release_canary.go`
- Test: `internal/scaffold/infra/plugin_release_canary_test.go`

**Interfaces:**
- Consumes: `frameworkAdapterName`, `hertzCanaryTraffic`, `markerCanaryServerTraffic`, `markerKitexClientMiddleware`, `insertAfterMarkerOrAnyWithPlan`, `addGoImportWithPlan` (all existing, unchanged in `wire.go`).
- Produces: `releaseCanaryPlugin` registered under `KindReleaseCanary` with alias `KindCanaryAlias`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_release_canary_test.go
package infra

import (
	"reflect"
	"strings"
	"testing"
)

func TestReleaseCanaryPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindReleaseCanary)
	if !ok {
		t.Fatal("release_canary plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindCanaryAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindCanaryAlias)
	}
}

func TestReleaseCanaryPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindReleaseCanary)
	files, err := p.AssetFiles("kitex")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{
		{SourcePath: "optional/release_canary.go", OutputRelPath: "internal/base/release/canary.go"},
		{SourcePath: "kitex/optional/release_canary.go", OutputRelPath: "internal/base/release/kitex.go"},
		{SourcePath: "optional/release_ops.go", OutputRelPath: "internal/base/release/ops.go"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}

func TestReleaseCanaryPluginWiresKitexClient(t *testing.T) {
	p, _ := pluginByKind(KindReleaseCanary)
	w, ok := p.(kitexClientWirer)
	if !ok {
		t.Fatal("release_canary plugin must implement kitexClientWirer")
	}
	src := "package client\n\nfunc New() {\n\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n}\n"
	var plan []PlanItem
	out, err := w.WireKitexClient(src, "example.com/mod", &plan)
	if err != nil {
		t.Fatalf("WireKitexClient: %v", err)
	}
	if !strings.Contains(out, "release.KitexTraffic()") {
		t.Errorf("WireKitexClient output missing release.KitexTraffic(): %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestReleaseCanaryPlugin -v -count=1`
Expected: FAIL — plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_release_canary.go
package infra

import "github.com/byx-darwin/ncgo/internal/manifest"

func init() { Register(releaseCanaryPlugin{}) }

type releaseCanaryPlugin struct{}

func (releaseCanaryPlugin) Kind() string        { return KindReleaseCanary }
func (releaseCanaryPlugin) Aliases() []string   { return []string{KindCanaryAlias} }
func (releaseCanaryPlugin) ServiceScope() string { return "common" }
func (releaseCanaryPlugin) GoGetDeps() []string  { return nil }
func (releaseCanaryPlugin) SetupSteps() []string { return nil }
func (releaseCanaryPlugin) HertzConfigKey() string { return "" }

func (releaseCanaryPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	files := []addOnFile{{SourcePath: "optional/" + KindReleaseCanary + ".go", OutputRelPath: "internal/base/release/canary.go"}}
	switch serviceKind {
	case manifest.KindHertz, manifest.KindKitex:
		files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + KindReleaseCanary + ".go", OutputRelPath: frameworkAdapterName(KindReleaseCanary, serviceKind)})
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
	files = append(files, addOnFile{SourcePath: "optional/release_ops.go", OutputRelPath: "internal/base/release/ops.go"})
	return files, nil
}

func (releaseCanaryPlugin) WireHertzServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/release", "", plan)
	if err != nil {
		return "", err
	}
	return insertAfterMarkerOrAnyWithPlan(s, "release.HertzTraffic()", markerCanaryServerTraffic, []string{
		"\th.Use(logging.HertzRequestID())\n",
		"\th.Use(middleware.RequestID())\n",
	}, hertzCanaryTraffic(), "", plan, "insert_traffic_middleware", "release.HertzTraffic")
}

func (releaseCanaryPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/release", "", plan)
	if err != nil {
		return "", err
	}
	return insertAfterMarkerOrAnyWithPlan(s, "release.KitexTraffic()", markerCanaryServerTraffic, []string{
		"\t\t\tlogging.KitexRequestID(),\n",
		"\t\t\tinterceptor.RequestID(),\n",
	}, "\t\t\trelease.KitexTraffic(),\n", "", plan, "insert_traffic_middleware", "release.KitexTraffic")
}

func (releaseCanaryPlugin) WireKitexClient(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/release", "", plan)
	if err != nil {
		return "", err
	}
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	return insertOnceMarkerOrAnchorWithPlan(s, "release.KitexTraffic()", markerKitexClientMiddleware, anchor, "\toptions = append(options, kitexclient.WithMiddleware(release.KitexTraffic()))\n", "", plan, "insert_client_middleware", "release.KitexTraffic")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestReleaseCanaryPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_release_canary.go internal/scaffold/infra/plugin_release_canary_test.go
git commit -m "refactor(scaffold): extract release_canary infra add-on to Plugin type"
```

---

### Task 8: `registry_polaris` plugin (+ wire hooks, kitex-only)

**Files:**
- Create: `internal/scaffold/infra/plugin_registry_polaris.go`
- Test: `internal/scaffold/infra/plugin_registry_polaris_test.go`

**Interfaces:**
- Consumes: `kitexRegistryServer`, `kitexRegistryClient`, `markerRegistryServer`, `markerRegistryClient` (existing, unchanged in `wire.go`).
- Produces: `registryPolarisPlugin` registered under `KindRegistryPolaris`, `ServiceScope() == "kitex"`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_registry_polaris_test.go
package infra

import (
	"reflect"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestRegistryPolarisPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindRegistryPolaris)
	if !ok {
		t.Fatal("registry_polaris plugin not registered")
	}
	if p.ServiceScope() != "kitex" {
		t.Errorf("ServiceScope() = %q, want kitex", p.ServiceScope())
	}
	wantDeps := []string{"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"}
	if !reflect.DeepEqual(p.GoGetDeps(), wantDeps) {
		t.Errorf("GoGetDeps() = %v, want %v", p.GoGetDeps(), wantDeps)
	}
}

func TestRegistryPolarisPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindRegistryPolaris)
	files, err := p.AssetFiles(manifest.KindKitex)
	if err != nil {
		t.Fatalf("AssetFiles(kitex): %v", err)
	}
	want := []addOnFile{
		{SourcePath: "kitex/optional/registry_polaris.go", OutputRelPath: "internal/base/registry/polaris.go"},
		{SourcePath: "kitex/optional/registry_polaris.yaml", OutputRelPath: "polaris.yaml"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles(kitex) = %+v, want %+v", files, want)
	}
	if _, err := p.AssetFiles(manifest.KindHertz); err == nil {
		t.Error("AssetFiles(hertz) should error: registry_polaris is kitex-only")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestRegistryPolarisPlugin -v -count=1`
Expected: FAIL — plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_registry_polaris.go
package infra

import "github.com/byx-darwin/ncgo/internal/manifest"

func init() { Register(registryPolarisPlugin{}) }

type registryPolarisPlugin struct{}

func (registryPolarisPlugin) Kind() string        { return KindRegistryPolaris }
func (registryPolarisPlugin) Aliases() []string   { return nil }
func (registryPolarisPlugin) ServiceScope() string { return "kitex" }
func (registryPolarisPlugin) GoGetDeps() []string {
	return []string{"github.com/kitex-contrib/polaris", "github.com/byx-darwin/go-tools/go-common"}
}
func (registryPolarisPlugin) SetupSteps() []string   { return nil }
func (registryPolarisPlugin) HertzConfigKey() string { return "" }

func (registryPolarisPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindKitex {
		return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", KindRegistryPolaris)
	}
	return []addOnFile{
		{SourcePath: "kitex/optional/registry_polaris.go", OutputRelPath: "internal/base/registry/polaris.go"},
		{SourcePath: "kitex/optional/registry_polaris.yaml", OutputRelPath: "polaris.yaml"},
	}, nil
}

func (registryPolarisPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/registry", "", plan)
	if err != nil {
		return "", err
	}
	return insertOnceMarkerOrAnchorWithPlan(s, "kitexserver.WithRegistry(", markerRegistryServer, "\topts = append(opts, extraOptions...)\n", kitexRegistryServer(), "", plan, "insert_registry_server", "registry.NewRegistry")
}

func (registryPolarisPlugin) WireKitexClient(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/registry", "", plan)
	if err != nil {
		return "", err
	}
	anchor := "\tif cfg.EnableMetaInfo {\n\t\toptions = append(options, kitexclient.WithMetaHandler(transmeta.ClientTTHeaderHandler))\n\t}\n"
	return insertOnceMarkerOrAnchorWithPlan(s, "kitexclient.WithResolver(", markerRegistryClient, anchor, kitexRegistryClient(), "", plan, "insert_registry_client", "registry.NewResolver")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestRegistryPolarisPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_registry_polaris.go internal/scaffold/infra/plugin_registry_polaris_test.go
git commit -m "refactor(scaffold): extract registry_polaris infra add-on to Plugin type"
```

---

### Task 9: `rate_limit` plugin (+ wire hook, kitex-only, custom AssetFiles)

**Files:**
- Create: `internal/scaffold/infra/plugin_rate_limit.go`
- Test: `internal/scaffold/infra/plugin_rate_limit_test.go`

**Interfaces:**
- Consumes: `rateLimitAssetFiles()` (existing function in `infra.go`, unchanged — reused verbatim as the plugin's `AssetFiles` body), `markerRateLimitServerMiddleware`, `markerRateLimitStaticLimit`, `insertAfterMarkerOrAnyWithPlan`, `insertOnceMarkerOrAnchorWithPlan`, `addGoImportWithPlan` (existing, unchanged).
- Produces: `rateLimitPlugin` registered under `KindRateLimit` with alias `KindRateLimitAlias`, `ServiceScope() == "kitex"`. `SetupSteps()` returns the explicit 4-step override (not nil).

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_rate_limit_test.go
package infra

import (
	"reflect"
	"testing"
)

func TestRateLimitPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindRateLimit)
	if !ok {
		t.Fatal("rate_limit plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindRateLimitAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindRateLimitAlias)
	}
	if p.ServiceScope() != "kitex" {
		t.Errorf("ServiceScope() = %q, want kitex", p.ServiceScope())
	}
	want := []string{
		"review conf/dev/conf.yaml rate_limit block (source.type: config|database|rule_center)",
		"observe shadow logs (grep 'ratelimit shadow denied'), then set mode: enforce",
		"optional: set static.max_qps / static.max_connections for a global safety net",
		"go mod tidy",
	}
	if !reflect.DeepEqual(p.SetupSteps(), want) {
		t.Errorf("SetupSteps() = %v, want %v", p.SetupSteps(), want)
	}
}

func TestRateLimitPluginAssetFilesReturnsSharedFragmentsAndMiddleware(t *testing.T) {
	p, _ := pluginByKind(KindRateLimit)
	files, err := p.AssetFiles("kitex")
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("AssetFiles returned %d files, want 5 (4 shared fragments + 1 middleware)", len(files))
	}
	last := files[len(files)-1]
	if last.OutputRelPath == "" {
		t.Error("middleware template file missing OutputRelPath")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestRateLimitPlugin -v -count=1`
Expected: FAIL — plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_rate_limit.go
package infra

func init() { Register(rateLimitPlugin{}) }

type rateLimitPlugin struct{}

func (rateLimitPlugin) Kind() string        { return KindRateLimit }
func (rateLimitPlugin) Aliases() []string   { return []string{KindRateLimitAlias} }
func (rateLimitPlugin) ServiceScope() string { return "kitex" }
func (rateLimitPlugin) GoGetDeps() []string  { return nil }
func (rateLimitPlugin) SetupSteps() []string {
	return []string{
		"review conf/dev/conf.yaml rate_limit block (source.type: config|database|rule_center)",
		"observe shadow logs (grep 'ratelimit shadow denied'), then set mode: enforce",
		"optional: set static.max_qps / static.max_connections for a global safety net",
		"go mod tidy",
	}
}
func (rateLimitPlugin) HertzConfigKey() string { return "" }

func (rateLimitPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifestKindKitex {
		return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", KindRateLimit)
	}
	return rateLimitAssetFiles()
}

func (rateLimitPlugin) WireKitexServer(s, module string, plan *[]PlanItem) (string, error) {
	s, err := addGoImportWithPlan(s, module+"/internal/base/middleware", "", plan)
	if err != nil {
		return "", err
	}
	s, err = insertAfterMarkerOrAnyWithPlan(s, "\t\t\tmiddleware.RateLimit(cfg.RateLimit),\n", markerRateLimitServerMiddleware, []string{
		"\t\t\tinterceptor.RequestID(),\n",
	}, "\t\t\tmiddleware.RateLimit(cfg.RateLimit),\n", "", plan, "insert_ratelimit_middleware", "middleware.RateLimit")
	if err != nil {
		return "", err
	}
	return insertOnceMarkerOrAnchorWithPlan(s, "middleware.StaticLimitOption(", markerRateLimitStaticLimit, "\topts = append(opts, extraOptions...)\n", "\tif opt := middleware.StaticLimitOption(cfg.RateLimit.Static); opt != nil {\n\t\topts = append(opts, opt)\n\t}\n", "", plan, "insert_ratelimit_static", "middleware.StaticLimitOption")
}
```

`rateLimitAssetFiles()` already exists unchanged in `infra.go` (returns the 4 shared fragments + 1 middleware template — see spec section "数据流"). `manifestKindKitex` is the existing unexported constant already declared in `wire.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestRateLimitPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_rate_limit.go internal/scaffold/infra/plugin_rate_limit_test.go
git commit -m "refactor(scaffold): extract rate_limit infra add-on to Plugin type"
```

---

### Task 10: `polaris_adapter` plugin (kitex-only, no wire)

**Files:**
- Create: `internal/scaffold/infra/plugin_polaris_adapter.go`
- Test: `internal/scaffold/infra/plugin_polaris_adapter_test.go`

**Interfaces:**
- Consumes: none beyond `Plugin`/`Register` (this kind has no `--wire` support today — `wireSupportedKind` excludes `KindPolarisAdapter`).
- Produces: `polarisAdapterPlugin` registered under `KindPolarisAdapter` with alias `KindPolarisAdapterAlias`, `ServiceScope() == "kitex"`.

- [ ] **Step 1: Write the failing test**

```go
// internal/scaffold/infra/plugin_polaris_adapter_test.go
package infra

import (
	"reflect"
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestPolarisAdapterPluginMetadata(t *testing.T) {
	p, ok := pluginByKind(KindPolarisAdapter)
	if !ok {
		t.Fatal("polaris_adapter plugin not registered")
	}
	if !reflect.DeepEqual(p.Aliases(), []string{KindPolarisAdapterAlias}) {
		t.Errorf("Aliases() = %v, want [%s]", p.Aliases(), KindPolarisAdapterAlias)
	}
	want := []string{
		"go get github.com/polarismesh/polaris-go",
		"go get gopkg.in/yaml.v3",
		"go get github.com/byx-darwin/go-tools/go-common",
		"go get go.opentelemetry.io/otel/metric",
		"set POLARIS_TOKEN / POLARIS_NAMESPACE env vars (never hardcode credentials)",
		"wire release.NewPolarisSelector(...) into KitexCanaryLoadBalancer.RuleProvider",
		"verify kitex resolver returns full stable+canary instance set (see troubleshooting)",
		"go mod tidy",
	}
	if !reflect.DeepEqual(p.SetupSteps(), want) {
		t.Errorf("SetupSteps() = %v, want %v", p.SetupSteps(), want)
	}
	if _, ok := p.(hertzServerWirer); ok {
		t.Error("polaris_adapter must not implement hertzServerWirer")
	}
	if _, ok := p.(kitexServerWirer); ok {
		t.Error("polaris_adapter must not implement kitexServerWirer (no --wire support today)")
	}
}

func TestPolarisAdapterPluginAssetFiles(t *testing.T) {
	p, _ := pluginByKind(KindPolarisAdapter)
	files, err := p.AssetFiles(manifest.KindKitex)
	if err != nil {
		t.Fatalf("AssetFiles: %v", err)
	}
	want := []addOnFile{
		{SourcePath: "kitex/optional/polaris_canary_adapter.go", OutputRelPath: "internal/base/release/polaris_adapter.go"},
		{SourcePath: "kitex/optional/polaris_canary_observer_otel.go", OutputRelPath: "internal/base/release/polaris_observer_otel.go"},
	}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("AssetFiles = %+v, want %+v", files, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/infra/... -run TestPolarisAdapterPlugin -v -count=1`
Expected: FAIL — plugin not registered.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/scaffold/infra/plugin_polaris_adapter.go
package infra

import "github.com/byx-darwin/ncgo/internal/manifest"

func init() { Register(polarisAdapterPlugin{}) }

type polarisAdapterPlugin struct{}

func (polarisAdapterPlugin) Kind() string        { return KindPolarisAdapter }
func (polarisAdapterPlugin) Aliases() []string   { return []string{KindPolarisAdapterAlias} }
func (polarisAdapterPlugin) ServiceScope() string { return "kitex" }
func (polarisAdapterPlugin) GoGetDeps() []string {
	return []string{"github.com/polarismesh/polaris-go", "gopkg.in/yaml.v3", "github.com/byx-darwin/go-tools/go-common", "go.opentelemetry.io/otel", "go.opentelemetry.io/otel/metric"}
}
func (polarisAdapterPlugin) SetupSteps() []string {
	return []string{
		"go get github.com/polarismesh/polaris-go",
		"go get gopkg.in/yaml.v3",
		"go get github.com/byx-darwin/go-tools/go-common",
		"go get go.opentelemetry.io/otel/metric",
		"set POLARIS_TOKEN / POLARIS_NAMESPACE env vars (never hardcode credentials)",
		"wire release.NewPolarisSelector(...) into KitexCanaryLoadBalancer.RuleProvider",
		"verify kitex resolver returns full stable+canary instance set (see troubleshooting)",
		"go mod tidy",
	}
}
func (polarisAdapterPlugin) HertzConfigKey() string { return "" }

func (polarisAdapterPlugin) AssetFiles(serviceKind string) ([]addOnFile, error) {
	if serviceKind != manifest.KindKitex {
		return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", KindPolarisAdapter)
	}
	return []addOnFile{
		{SourcePath: "kitex/optional/polaris_canary_adapter.go", OutputRelPath: "internal/base/release/polaris_adapter.go"},
		{SourcePath: "kitex/optional/polaris_canary_observer_otel.go", OutputRelPath: "internal/base/release/polaris_observer_otel.go"},
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scaffold/infra/... -run TestPolarisAdapterPlugin -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full existing infra package suite to confirm no regression**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/plugin_polaris_adapter.go internal/scaffold/infra/plugin_polaris_adapter_test.go
git commit -m "refactor(scaffold): extract polaris_adapter infra add-on to Plugin type"
```

---

### Task 11: Cutover — rewire `Add()`/`Wire()` dispatch to the registry, delete dead code

This is the single atomic step where dispatch switches from the legacy maps/switches to the registry. All 9 plugins from Tasks 2-10 are already registered and characterization-tested; this task's test is the **existing** `infra_test.go` + `golden_test.go` suite, unchanged — a full pass proves behavioral equivalence.

**Files:**
- Modify: `internal/scaffold/infra/infra.go`
- Modify: `internal/scaffold/infra/wire.go`

**Interfaces:**
- Consumes: `pluginByKind` (Task 1), all 9 plugin types (Tasks 2-10).
- Produces: no new exported symbols; `SupportedKinds()`/`Add()`/`Wire()`/etc. keep their existing signatures and behavior.

- [ ] **Step 1: Confirm the pre-cutover baseline passes**

Run: `go test ./internal/scaffold/infra/... -count=1`
Expected: PASS (this is the baseline the cutover must not break).

- [ ] **Step 2: Rewrite `infra.go` dispatch functions**

In `internal/scaffold/infra/infra.go`:

1. Replace `SupportedKinds()`'s body — keep the exact same literal return (already correct; no change needed, since the const-listing form already matches Global Constraints).
2. Delete `commonKinds()` and replace its one caller in `infra_test.go` is out of scope for this task (tests are not modified) — instead **redefine** `commonKinds()` to compute from the registry so the existing test caller keeps working unmodified:

```go
func commonKinds() []string {
	out := make([]string, 0, 6)
	for _, kind := range kindOrder {
		p, ok := pluginByKind(kind)
		if !ok || p.Kind() != kind { // skip aliases, only canonical kinds
			continue
		}
		if p.ServiceScope() == "common" {
			out = append(out, kind)
		}
	}
	return out
}
```

Add the `kindOrder` slice (module-level var, exact order from Global Constraints):

```go
var kindOrder = []string{
	KindRedis, KindKafka, KindES, KindClickHouse,
	KindObservabilityLog, KindLoggingAlias,
	KindReleaseCanary, KindCanaryAlias,
	KindRegistryPolaris,
	KindRateLimit, KindRateLimitAlias,
	KindPolarisAdapter, KindPolarisAdapterAlias,
}
```

3. Delete `kitexOnlyKinds()` and `isKitexOnly()` — no longer needed; kitex-only validation now lives inside each plugin's own `AssetFiles` (Tasks 8, 9, 10 already return the "only supported for kitex services" error themselves).
4. Delete the package-level maps `goGetDeps`, `setupSteps`, `commonAssetKinds`, `outputRelPaths`, `hertzConfigSnippetKeys` — fully superseded by plugin methods.
5. Replace `assetFiles(serviceKind, infraKind string) ([]addOnFile, error)` body:

```go
func assetFiles(serviceKind, infraKind string) ([]addOnFile, error) {
	p, ok := pluginByKind(infraKind)
	if !ok {
		return nil, fmt.Errorf("infra: kind %q is invalid; want one of %v", infraKind, SupportedKinds())
	}
	return p.AssetFiles(serviceKind)
}
```

6. Delete `assetPath()` — fully superseded (its logic now lives in `frameworkAssetFiles` and each complex plugin's own `AssetFiles`).
7. Replace `appendHertzRedisHelperIfMissing` call site in `Add()`: change

```go
files, err = appendHertzRedisHelperIfMissing(files, root, m.Service.Kind, kind)
```

to

```go
if pl, ok := pluginByKind(kind); ok {
	if p, ok := pl.(extraFilesPlugin); ok {
		extra, err := p.ExtraFiles(root, m.Service.Kind)
		if err != nil {
			return nil, err
		}
		files = append(files, extra...)
	}
}
```

(`kind` here is already normalized by an earlier `normalizeKind` call, so the outer `ok` is always true; the check is kept for safety rather than assumed.) Delete the now-unused `appendHertzRedisHelperIfMissing` function.

8. Replace `normalizeKind`:

```go
func normalizeKind(kind string) (string, error) {
	if p, ok := pluginByKind(kind); ok {
		return p.Kind(), nil
	}
	return "", fmt.Errorf("infra: kind %q is invalid; want one of %v", kind, SupportedKinds())
}
```

9. Replace `planHertzConfigWrite`'s lookup of `hertzConfigSnippetKeys[infraKind]`:

```go
func planHertzConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	if serviceKind != manifest.KindHertz {
		return nil, nil
	}
	p, ok := pluginByKind(infraKind)
	if !ok || p.HertzConfigKey() == "" {
		return nil, nil
	}
	...
```

and replace every remaining `hertzConfigSnippetKeys[infraKind]` reference in `mergeHertzConfig`/`nextSteps` with `p.HertzConfigKey()` (pass the resolved `Plugin` or its key string through as a parameter where needed — `mergeHertzConfig` gains a `hertzConfigKey string` parameter in place of doing its own map lookup).

10. Replace `frameworkAdapterName`'s callers — keep the function itself (still used inside `plugin_observability_logging.go`/`plugin_release_canary.go` from Tasks 6-7), no change needed here.

11. Replace `nextSteps`:

```go
func nextSteps(kind, serviceKind, serviceName string) []string {
	p, ok := pluginByKind(kind)
	if !ok {
		return []string{"go mod tidy"}
	}
	if steps := p.SetupSteps(); steps != nil {
		out := append([]string(nil), steps...)
		if serviceName != "" {
			for i, step := range out {
				if step == "OTEL_SERVICE_NAME=<service> OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>" {
					out[i] = "OTEL_SERVICE_NAME=" + serviceName + " OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./<your-binary>"
				}
			}
		}
		return out
	}
	steps := make([]string, 0, len(p.GoGetDeps())+1)
	for _, dep := range p.GoGetDeps() {
		steps = append(steps, "go get "+dep)
	}
	if serviceKind == manifest.KindHertz && p.HertzConfigKey() != "" {
		steps = append(steps, "review "+filepath.FromSlash(hertzConfigRelPath)+" and complete the `"+p.HertzConfigKey()+"` section for local config or your config-center payload")
	}
	steps = append(steps, "go mod tidy")
	return steps
}
```

- [ ] **Step 3: Rewrite `wire.go` dispatch functions**

In `internal/scaffold/infra/wire.go`:

1. Replace `wireSupportedKind`:

```go
func wireSupportedKind(kind string) bool {
	p, ok := pluginByKind(kind)
	if !ok {
		return false
	}
	if _, ok := p.(hertzServerWirer); ok {
		return true
	}
	if _, ok := p.(kitexServerWirer); ok {
		return true
	}
	if _, ok := p.(kitexClientWirer); ok {
		return true
	}
	return false
}
```

2. Replace `unsupportedWireError` — keep the literal message text (Global Constraints require byte-identical text; do not derive it dynamically, since the order/format is a stable contract, not something worth risking on iteration order):

```go
func unsupportedWireError() error {
	return fmt.Errorf("infra: --wire is only supported for %s/%s/%s/%s", KindObservabilityLog, KindReleaseCanary, KindRegistryPolaris, KindRateLimit)
}
```

3. Replace `wireHertz` body:

```go
func wireHertz(root, module, kind string, dryRun bool) (*wireResult, error) {
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(path)
	if err != nil {
		return nil, err
	}
	s := string(body)
	plan := []PlanItem(nil)
	if p, ok := pluginByKind(kind); ok {
		if w, ok := p.(hertzServerWirer); ok {
			s, err = w.WireHertzServer(s, module, &plan)
			if err != nil {
				return nil, err
			}
			// re-home plan entries onto the real path (WireHertzServer's
			// helpers were exercised with path="" in plugin unit tests)
			for i := range plan {
				plan[i].Path = path
			}
		}
	}
	written, err := writeFormatted(path, []byte(s), dryRun)
	if err != nil {
		return nil, err
	}
	return wireResultFor(written, plan), nil
}
```

4. Replace `wireKitex` body's inner `switch kind { ... }` block (lines computing `serverPlan`) with:

```go
	serverPlan := []PlanItem(nil)
	if p, ok := pluginByKind(kind); ok {
		if w, ok := p.(kitexServerWirer); ok {
			s, err = w.WireKitexServer(s, module, &serverPlan)
			if err != nil {
				return nil, err
			}
			for i := range serverPlan {
				serverPlan[i].Path = serverPath
			}
		}
	}
```

(keep the rest of `wireKitex` — `writeFormatted`, the `filepath.Glob` client loop calling `wireKitexClient` — unchanged).

5. Replace `wireKitexClient` body's inner `switch kind { ... }` block with:

```go
	plan := []PlanItem(nil)
	if p, ok := pluginByKind(kind); ok {
		if w, ok := p.(kitexClientWirer); ok {
			s, err = w.WireKitexClient(s, module, &plan)
			if err != nil {
				return "", err
			}
			for i := range plan {
				plan[i].Path = path
			}
		}
	}
```

(keep `readSource`/`writeFormatted` calls surrounding it unchanged).

> The `path`/`""`-then-backfill pattern above is required because the existing `*WithPlan` helpers (`addGoImportWithPlan` etc.) bake `path` into each `PlanItem` at call time, and the plugin methods (Task 6-9) call them with `path=""` since a plugin method has no path parameter of its own. Backfilling `plan[i].Path` after the call restores the exact `PlanItem.Path` values the legacy switch-based code produced. Verify this specifically: `internal/scaffold/infra/infra_test.go` has existing assertions on `Result.Plan` / `PreviewWirePlan` path fields — the full run in Step 4 below is the check.

- [ ] **Step 4: Run the full existing infra package suite**

Run: `go test ./internal/scaffold/infra/... -count=1 -v 2>&1 | tail -100`
Expected: PASS — every existing test (`infra_test.go`, `golden_test.go`, `render_test.go`) and every new plugin test from Tasks 1-10 passes with no changes to assertions.

If any `PlanItem.Path` assertion fails, the backfill loop in Step 3 was missed for that dispatch function — fix and rerun.

- [ ] **Step 5: Run golden test explicitly to confirm zero diff**

Run: `go test ./internal/scaffold/infra/... -run TestGenerateGolden -v -count=1`
Expected: PASS for every `TestGenerateGoldenInfra*` test — confirms `internal/scaffold/infra/testdata/**` fixtures are unchanged.

- [ ] **Step 6: Build and vet**

Run: `go build ./... && go vet ./internal/scaffold/infra/...`
Expected: no errors, no unused-symbol warnings (confirms all dead legacy code was actually deleted, not just unreferenced).

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/wire.go
git commit -m "refactor(scaffold): rewire infra Add/Wire dispatch through the Plugin registry"
```

---

### Task 12: Repository-wide final validation

**Files:** none (validation only).

- [ ] **Step 1: Full build**

Run: `go build ./... && go build .`
Expected: success.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 3: Full test suite**

Run: `go test ./... -count=1`
Expected: all packages PASS.

- [ ] **Step 4: gofmt check**

Run: `gofmt -l $(find internal/scaffold/infra -name '*.go')`
Expected: empty output (no unformatted files).

- [ ] **Step 5: Smoke test**

Run: `./scripts/smoke.sh`
Expected: success (exercises `ncgo add infra <kind>` end-to-end via the CLI).

- [ ] **Step 6: Confirm no leftover references to deleted symbols**

Run: `grep -rn "kitexOnlyKinds\|commonAssetKinds\|hertzConfigSnippetKeys\|outputRelPaths\b\|appendHertzRedisHelperIfMissing\b" --include="*.go" internal/scaffold/infra/`
Expected: no matches (confirms full removal of legacy maps/helpers).

- [ ] **Step 7: Final commit (if Steps 1-6 required any fixups)**

```bash
git add -A
git commit -m "chore(scaffold): final validation pass for infra Plugin registry refactor"
```

(Skip this commit if Steps 1-6 required no changes — Task 11's commit is then the final state.)
