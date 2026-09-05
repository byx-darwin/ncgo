# FrameworkAdapter (Issue #101) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce `internal/scaffold/framework.Adapter` (a segregated-interface, registry-based abstraction over Hertz/Kitex differences) and migrate the existing hardcoded `if`/`switch` branches in `mono/mono.go`, `mono/files.go`, `infra/infra.go`, `infra/wire.go`, and `shared/container.go` to call it, with **zero change to generated output** (verified via golden tests at every migration step).

**Architecture:** A new leaf-ish package `internal/scaffold/framework` defines five small interfaces (`AssetResolver`, `ConfigMerger`, `ContainerRenderer`, `Generator`, `Wirer`) composed into `Adapter`, plus a `Register`/`Get`/`MustGet` registry mirroring `internal/scaffold/infra`'s `Plugin` pattern (see `internal/scaffold/infra/infra.go:50-131` and `internal/scaffold/infra/plugin_redis.go`). Two concrete adapters — `hertzAdapter` (`adapter_hertz.go`) and `kitexAdapter` (`adapter_kitex.go`) — register themselves via `init()`. `framework` imports only leaf packages (`internal/manifest`, `internal/assets`, `internal/exec`) so it can be imported by `mono`, `infra`, and `shared` without any import cycle; it never imports `mono`, `infra`, `shared`, or `scaffoldtemplate` back.

**Tech Stack:** Go 1.25+, existing `internal/exec.Runner`/`internal/manifest.Manifest` types, `internal/testutil/golden` for golden-tree tests.

**Spec:** `docs/superpowers/specs/2026-09-04-framework-adapter-design.md`

## Global Constraints

- **Zero generated-output change.** This is a pure interface-convergence refactor (spec's own framing: "本次是纯接口收敛重构... 不改变生成产物内容"). Every migration task ends with a golden-test run that must show **zero diff**.
- **No third framework.** Do not add gin/grpc-go or make `manifest.Validate`'s Kind enum dynamically extensible — spec explicitly excludes this ("本次是纯接口收敛重构，不新增第三种框架").
- **`manifest.go:138` `Validate` switch stays untouched.** It is schema-layer enum validation, out of scope per spec.
- **`bff/bff.go`, `rpc/rpc.go` stay untouched.** Their Kind values are fixed literals (bff is always Hertz, rpc is always Kitex), not branches — not a divergence point per spec.
- **No import cycle:** `internal/scaffold/framework` MUST NOT import `internal/scaffold/mono`, `internal/scaffold/infra`, `internal/scaffold/shared`, or `internal/scaffold/template`. It may import `internal/manifest`, `internal/assets`, `internal/exec`, and stdlib only.
- **Template handoff ordering preserved:** Kitex scaffolds must still run `make sqlc` before `go mod tidy`; Hertz needs the same ordering only when `WithDatabase=true` (`requiresSQLCBeforeTidy`/`Generator.RequiresSQLCBeforeTidy` must preserve this exactly).
- Follow `.claude/rules/go.md` and `.claude/rules/agent-engineering.md`: minimal diffs, no opportunistic refactors, `gofmt`-clean, tests updated alongside behavior.

## Documented Scope Decisions (read before starting)

The design doc's pseudocode has two problems that would either not compile or force a much larger refactor than "pure interface convergence." This plan resolves them as follows — do not deviate without re-reading this section:

1. **No `mono.Options` or `infra.Plugin` types in the `framework` package.** The design doc's sketch (`Generator.IDLPath(opts mono.Options)`, `ConfigMerger.MergeServiceConfig(plugin Plugin, ...)`) would create `framework → mono → framework` and `framework → infra → framework` import cycles, since `mono` and `infra` both need to import `framework` to call it. This plan instead defines a small standalone `framework.GeneratorOptions{Name, Module string}` struct carrying only the fields `Generator` methods need, and expresses `ConfigMerger` methods in terms of raw `[]byte`/`string` (snippet, current file bytes, key strings) instead of the `infra.Plugin` interface. Callers in `infra.go` extract those fields from `Plugin`/`Options` before calling the adapter.
2. **`Wirer` is intentionally thin.** `infra/wire.go`'s `wireHertz`/`wireKitex` bodies are inseparable from `infra`-package-only optional capability interfaces (`hertzServerWirer`, `kitexServerWirer`, `kitexClientWirer` — see `internal/scaffold/infra/infra.go:78-88`), which `framework` cannot depend on without an `infra → framework → infra` cycle. Turning the full wiring logic into an `Adapter.Wire(ctx WireContext) error` method (as the design doc's pseudocode implies) would require a new plugin-callback abstraction — real scope creep beyond "纯接口收敛重构" for this issue. This plan's `Wirer` interface only extracts the one safely-portable value: `ServerFilePath() string` (replacing the hardcoded `"internal/base/server/server.go"` literal duplicated in both `wireHertz` and `wireKitex`), and the `wire()` dispatcher's local `manifestKindHertz`/`manifestKindKitex` constants are deleted in favor of `manifest.KindHertz`/`manifest.KindKitex` (the exact duplication the spec calls out at `infra/wire.go:83-84`). `wireHertz`/`wireKitex` themselves stay as-is.
3. **`mono/mono.go` Kitex-only go.mod pre-write / manifest-domain-scan / Hertz-only template-overlay-after-hz (lines 177, 190, 197 in the divergence catalog) stay as literal `defaultKind(opts.Kind) == manifest.KindKitex/KindHertz` checks.** The spec's own "迁移范围" migration table (the concrete build scope, as opposed to the broader "现状调研" catalog of every branch found) only names `defaultKind`/`runGenerator`/`defaultIDL`/`validate` for `mono/mono.go`. These three call sites drive multi-step procedures entangled with `scaffoldtemplate`/`manifest.Load`/`os.ReadDir` — not simple per-kind values — and forcing them into the `Generator` interface would balloon it or reintroduce the `mono.Options` cycle. Left unchanged.
4. **`mono/files.go`'s `writeTemplate` dispatch (`writeHertzTemplate`/`writeKitexTemplate` bodies) stays as a literal Kind check.** Those functions are hundreds of lines deep into `assets.FS()`, `shared.ReadSharedFragmentBody`, `opts.Preset`, `opts.SkipDefaultTemplates` — moving them would require `framework` to import `shared`, which already must be imported *by* `shared` (for `ContainerRenderer`), producing a cycle. Only the leaf-value helpers (`idlNameToken`, `requiresSQLCBeforeTidy`, `renderIDLPlaceholder`, the Hertz-only proto-support-file write inside `writeIDLPlaceholder`, `generatorCommand`) migrate to `framework.Get(kind).XXX(...)` calls.
5. **`shared/container.go`'s `loadWorkspaceComposeApps` host-port allocation (spec line `:251`) stays untouched.** It is a stateful incrementing counter (`hertzHostPort`/`kitexHostPort` variables bumped per app), not a per-kind constant; `ContainerRenderer.ContainerPort()` only covers the fixed **container** port (8080/8888), not the host-port allocation range. Documented, not silently dropped.
6. **`internal/scaffold/infra/plugin_observability_logging.go:26` and `plugin_release_canary.go:24`** construct `SourcePath: serviceKind + "/optional/" + kind + ".go"` manually instead of going through `frameworkAssetFiles`. They are unchanged — not in the spec's migration table, and changing them is unrelated cleanup.

---

## File Structure

New:
- `internal/scaffold/framework/framework.go` — interface definitions, `GeneratorOptions`, `ConfigWrite`, `ComposeFeatureFlags` types.
- `internal/scaffold/framework/registry.go` — `Register`/`Get`/`MustGet` + internal `registry` type.
- `internal/scaffold/framework/registry_test.go` — registry unit tests with a fake adapter (no hertz/kitex dependency).
- `internal/scaffold/framework/mergeutil.go` — `hasTopLevelConfigKey`, shared by both adapters' config-merge logic.
- `internal/scaffold/framework/adapter_hertz.go` — `hertzAdapter`, registered via `init()`.
- `internal/scaffold/framework/adapter_hertz_test.go` — unit tests asserting exact outputs vs. current hardcoded behavior.
- `internal/scaffold/framework/adapter_kitex.go` — `kitexAdapter`, registered via `init()`.
- `internal/scaffold/framework/adapter_kitex_test.go` — unit tests.

Modified (one task each): `internal/scaffold/mono/mono.go`, `internal/scaffold/mono/files.go`, `internal/scaffold/infra/infra.go`, `internal/scaffold/infra/wire.go`, `internal/scaffold/shared/container.go`.

---

## Task 1: `framework` package skeleton — interfaces + Registry (no Hertz/Kitex dependency)

**Files:**
- Create: `internal/scaffold/framework/framework.go`
- Create: `internal/scaffold/framework/registry.go`
- Create: `internal/scaffold/framework/registry_test.go`

**Interfaces:**
- Produces: `framework.Adapter` (and its five embedded sub-interfaces), `framework.GeneratorOptions{Name, Module string}`, `framework.ConfigWrite{Body []byte, Action string}`, `framework.ComposeFeatureFlags{Nacos, Polaris, Vegeta bool}`, `framework.Register(a Adapter)`, `framework.Get(kind string) (Adapter, bool)`, `framework.MustGet(kind string) Adapter`. These exact names/signatures are consumed by Task 2 (adapters) and Tasks 3-7 (migrations).

- [ ] **Step 1: Write `framework.go`**

```go
// Package framework abstracts the differences between the two scaffold
// frameworks ncgo supports today — Hertz (manifest.KindHertz) and Kitex
// (manifest.KindKitex) — behind a small set of segregated interfaces
// composed into Adapter. It exists to remove the repeated Kind-branching
// switch/if statements spread across internal/scaffold/{mono,infra,shared}.
//
// framework MUST NOT import internal/scaffold/mono, internal/scaffold/infra,
// internal/scaffold/shared, or internal/scaffold/template — all of those
// import framework, so a back-import would create a cycle. It may import
// internal/manifest, internal/assets, internal/exec, and the standard
// library.
package framework

import (
	"context"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// GeneratorOptions carries only the mono.Options fields that Generator
// methods need. It is a standalone type (not mono.Options) because
// internal/scaffold/mono imports this package, not the other way around.
type GeneratorOptions struct {
	Name   string // service name
	Module string // Go module path
}

// AssetResolver resolves embedded-asset source paths that differ by
// framework kind.
type AssetResolver interface {
	// OptionalAssetPath returns the embedded source path for infraKind's
	// `ncgo add infra` optional add-on Go file (e.g. "hertz/optional/redis.go"),
	// and whether this framework has one.
	OptionalAssetPath(infraKind string) (string, bool)
	// HertzConfigAssetPath returns the embedded hertz/optional-config/<kind>.yaml
	// path for infraKind, and whether this framework has a per-kind Hertz
	// conf.yaml snippet. The Kitex adapter always returns ("", false).
	HertzConfigAssetPath(infraKind string) (string, bool)
}

// ConfigWrite is a planned conf/dev/conf.yaml write.
type ConfigWrite struct {
	Body   []byte
	Action string // "create" | "update"
}

// ConfigMerger computes planned conf/dev/conf.yaml writes for `ncgo add
// infra`. Only Hertz owns per-plugin config-key merging; only Kitex owns
// rate_limit block merging. The adapter that doesn't own a given merge
// returns (nil, nil).
type ConfigMerger interface {
	// MergeHertzConfig merges a plugin's Hertz config snippet (already read
	// by the caller from the path returned by HertzConfigAssetPath) into the
	// existing conf/dev/conf.yaml content. current is nil when the file does
	// not exist yet. Returns (nil, nil) when no write is needed.
	MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error)
	// MergeRateLimitConfig merges the kitex rate_limit infra add-on's
	// conf/dev/conf.yaml block. current is nil when the file does not exist
	// yet. Returns (nil, nil) when no write is needed.
	MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error)
}

// ComposeFeatureFlags are the compose-feature flags a framework enables on
// top of the infra-list-derived ones (postgres/redis/kafka/es/clickhouse are
// common to both kinds and computed by the caller).
type ComposeFeatureFlags struct {
	Nacos   bool // config-center + service-discovery nacos profile
	Polaris bool // config-center + service-discovery polaris profile
	Vegeta  bool // rate-limit load-testing sidecar
}

// ContainerRenderer resolves docker/compose differences that vary by kind.
type ContainerRenderer interface {
	// DockerConfigBlocks returns the conf/docker/conf.yaml YAML blocks (after
	// the shared "env: docker" header) specific to this framework and the
	// manifest's enabled infra/database flags.
	DockerConfigBlocks(m *manifest.Manifest) []string
	// ContainerPort is the fixed in-container listen port (Hertz: 8080,
	// Kitex: 8888).
	ContainerPort() int
	// ComposeFeatures reports which compose profile flags this framework
	// enables unconditionally (or when withDatabase is true).
	ComposeFeatures(withDatabase bool) ComposeFeatureFlags
}

// Generator resolves IDL path/content and code-generator invocation
// differences that vary by kind.
type Generator interface {
	// IDLPath is the default IDL path when Options.IDL is empty.
	IDLPath(opts GeneratorOptions) string
	// IDLNameToken is the per-kind service-name token substituted for
	// `{{ToLower .ServiceName}}` in a template package's relative IDL paths.
	IDLNameToken(opts GeneratorOptions) string
	// GeneratorCommand is the literal shell command a user can paste to
	// invoke this framework's generator.
	GeneratorCommand(opts GeneratorOptions, idl string) string
	// RenderIDLPlaceholder is the starter IDL file content dropped into the
	// scaffold when no IDL exists yet.
	RenderIDLPlaceholder(opts GeneratorOptions) string
	// WriteIDLSupportFiles writes any framework-specific support files that
	// must exist next to the IDL before the generator runs (Hertz: api.proto
	// + openapi/*.proto + validate/validate.proto; Kitex: no-op).
	WriteIDLSupportFiles(dir string) error
	// RunGenerator invokes this framework's code generator (hz or kitex).
	RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error)
	// RequiresSQLCBeforeTidy reports whether `make sqlc` must run before the
	// first `go mod tidy` for a scaffold using this framework.
	RequiresSQLCBeforeTidy(withDatabase bool) bool
}

// Wirer resolves the one framework-specific value infra/wire.go's dispatch
// needs. The bulk of --wire logic stays in internal/scaffold/infra because it
// depends on infra-package-only Plugin capability interfaces that framework
// cannot import back (see plan Documented Scope Decisions).
type Wirer interface {
	// ServerFilePath is the project-relative (slash-separated) path of the
	// generated server bootstrap file --wire patches first.
	ServerFilePath() string
}

// Adapter is the full per-framework capability set. Concrete adapters
// (hertzAdapter, kitexAdapter) are defined in adapter_hertz.go/adapter_kitex.go
// and register themselves via init().
type Adapter interface {
	Kind() string
	AssetResolver
	ConfigMerger
	ContainerRenderer
	Generator
	Wirer
}
```

- [ ] **Step 2: Write `registry.go`**

```go
package framework

import "fmt"

// registry is a lookup table from kind to Adapter. The zero value is not
// usable; construct with newRegistry. Mirrors internal/scaffold/infra's
// pluginRegistry pattern (see internal/scaffold/infra/infra.go:90-131).
type registry struct {
	byKind map[string]Adapter
}

func newRegistry() *registry {
	return &registry{byKind: map[string]Adapter{}}
}

func (r *registry) register(a Adapter) {
	if _, exists := r.byKind[a.Kind()]; exists {
		panic(fmt.Sprintf("framework: duplicate adapter registration for kind %q", a.Kind()))
	}
	r.byKind[a.Kind()] = a
}

func (r *registry) get(kind string) (Adapter, bool) {
	a, ok := r.byKind[kind]
	return a, ok
}

var defaultRegistry = newRegistry()

// Register adds an adapter to the package-level registry. Called from each
// adapter file's init(). Panics on duplicate Kind() — the adapter set is
// closed and known at compile time, so a collision is a programming error.
func Register(a Adapter) { defaultRegistry.register(a) }

// Get looks up the adapter for kind (e.g. manifest.KindHertz).
func Get(kind string) (Adapter, bool) { return defaultRegistry.get(kind) }

// MustGet looks up the adapter for kind and panics if none is registered.
// Callers use this only where kind has already been validated (e.g. after
// Options.validate() or manifest.Manifest.Validate() succeeded).
func MustGet(kind string) Adapter {
	a, ok := Get(kind)
	if !ok {
		panic(fmt.Sprintf("framework: no adapter registered for kind %q", kind))
	}
	return a
}
```

- [ ] **Step 3: Write `registry_test.go` with a trivial fake adapter (no hertz/kitex dependency)**

```go
package framework

import (
	"context"
	"testing"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

// fakeAdapter implements Adapter with trivial stub values, exercising only
// the registry — it must NOT depend on hertz/kitex-specific behavior.
type fakeAdapter struct{ kind string }

func (f fakeAdapter) Kind() string { return f.kind }
func (f fakeAdapter) OptionalAssetPath(infraKind string) (string, bool) {
	return f.kind + "/optional/" + infraKind + ".go", true
}
func (f fakeAdapter) HertzConfigAssetPath(infraKind string) (string, bool) { return "", false }
func (f fakeAdapter) MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil
}
func (f fakeAdapter) MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil
}
func (f fakeAdapter) DockerConfigBlocks(m *manifest.Manifest) []string     { return nil }
func (f fakeAdapter) ContainerPort() int                                  { return 0 }
func (f fakeAdapter) ComposeFeatures(withDatabase bool) ComposeFeatureFlags { return ComposeFeatureFlags{} }
func (f fakeAdapter) IDLPath(opts GeneratorOptions) string                { return "idl/fake.proto" }
func (f fakeAdapter) IDLNameToken(opts GeneratorOptions) string           { return opts.Name }
func (f fakeAdapter) GeneratorCommand(opts GeneratorOptions, idl string) string { return "fake-gen " + idl }
func (f fakeAdapter) RenderIDLPlaceholder(opts GeneratorOptions) string   { return "" }
func (f fakeAdapter) WriteIDLSupportFiles(dir string) error               { return nil }
func (f fakeAdapter) RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error) {
	return exec.Result{}, nil
}
func (f fakeAdapter) RequiresSQLCBeforeTidy(withDatabase bool) bool { return withDatabase }
func (f fakeAdapter) ServerFilePath() string                       { return "internal/base/server/server.go" }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := newRegistry()
	r.register(fakeAdapter{kind: "widget"})

	a, ok := r.get("widget")
	if !ok || a.Kind() != "widget" {
		t.Fatalf("get(widget) = %v, %v; want widget adapter", a, ok)
	}
	if _, ok := r.get("missing"); ok {
		t.Fatalf("get(missing) = ok=true; want ok=false")
	}
}

func TestRegistryRegisterDuplicateKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Kind() registration")
		}
	}()
	r := newRegistry()
	r.register(fakeAdapter{kind: "widget"})
	r.register(fakeAdapter{kind: "widget"})
}

func TestMustGetPanicsWhenMissing(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from MustGet on unknown kind")
		}
	}()
	MustGet("does-not-exist")
}

func TestGetMissingReturnsFalse(t *testing.T) {
	if _, ok := Get("does-not-exist"); ok {
		t.Fatalf("Get(does-not-exist) = ok=true; want ok=false")
	}
}
```

- [ ] **Step 4: Build and run the package standalone**

Run: `go build ./internal/scaffold/framework/... && go test ./internal/scaffold/framework/... -count=1`
Expected: build succeeds (no hertz/kitex files exist yet, so `Get`/`MustGet` for real kinds fail — that's expected and only exercised in Task 2+), all four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/framework/framework.go internal/scaffold/framework/registry.go internal/scaffold/framework/registry_test.go
git commit -m "feat(framework): add Adapter interfaces and Registry skeleton"
```

---

## Task 2: `hertzAdapter` and `kitexAdapter` implementations

**Files:**
- Create: `internal/scaffold/framework/adapter_hertz.go`
- Create: `internal/scaffold/framework/adapter_hertz_test.go`
- Create: `internal/scaffold/framework/adapter_kitex.go`
- Create: `internal/scaffold/framework/adapter_kitex_test.go`
- Create: `internal/scaffold/framework/mergeutil.go`

**Interfaces:**
- Consumes: `framework.Adapter`, `framework.GeneratorOptions`, `framework.ConfigWrite`, `framework.ComposeFeatureFlags`, `framework.Register` from Task 1.
- Produces: `manifest.KindHertz`-registered and `manifest.KindKitex`-registered adapters that Tasks 3-7 call via `framework.Get(kind)`/`framework.MustGet(kind)`.

Exact current-behavior values these adapters must reproduce (pulled from the code read during design): Hertz container port `8080` (`shared/container.go:14`), Kitex `8888` (`shared/container.go:15`); Hertz IDL default `idl/app/<name>.proto` (`mono/mono.go:243`), Kitex `idl/<kitexIDLBase>.proto` (`mono/mono.go:241`); `requiresSQLCBeforeTidy` = Kitex always `true`, Hertz only when `withDatabase` (`mono/files.go:939`); Hertz `hzArgs`/Kitex `kitexArgs` exactly as in `mono/mono.go:337-363`; the embedded `hertzAPIProto` literal and `writeHertzProtoSupportFiles` bodies exactly as in `mono/files.go:20-87,700-734`; `renderIDLPlaceholder` bodies exactly as in `mono/files.go:747-826`; `mergeHertzConfig`/`wrapHertzConfigSnippet`/`hertzConfigMarkers`/`replaceMarkedHertzConfigBlock` exactly as in `infra/infra.go:409-465`; `mergeKitexRateLimitConfig`/`defaultRateLimitConfBlock` exactly as in `infra/infra.go:707-816`; Hertz `ComposeFeatures` = `{Nacos:true, Polaris:true, Vegeta:withDatabase}` and Kitex = all-false (`shared/container.go:594-601`); `renderHertzDockerConfigBlocks`/`renderKitexDockerConfigBlocks` exactly as in `shared/container.go:518-573`.

- [ ] **Step 1: Write `mergeutil.go` (pure helper shared by both adapters)**

```go
package framework

import "strings"

// hasTopLevelConfigKey reports whether src has a non-indented, non-comment
// line beginning with "<key>:". Moved verbatim from
// internal/scaffold/infra/infra.go so both adapters' config-merge logic can
// use it without infra depending back on framework for it.
func hasTopLevelConfigKey(src, key string) bool {
	needle := key + ":"
	for _, line := range strings.Split(src, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.HasPrefix(line, needle) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Write `adapter_hertz.go`**

```go
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
```

Notes for the implementer:
- `manifestHasInfra`, `postgresDSN`, `mkdirAllFor`, `writeFileFor` above are placed in `adapter_hertz.go` for concreteness, but are also called from `adapter_kitex.go`'s `DockerConfigBlocks` (Step 3 below) — since both files are in the same `framework` package, this single definition is visible to both. If you prefer, move them to a separate `internal_helpers.go` in the package instead; either placement compiles identically.
- `exportName` (used by `RenderIDLPlaceholder`) is defined once in `adapter_kitex.go` (Step 3 below) since Kitex's `kitexIDLBase` also needs it — both adapters live in the same `framework` package so a single definition is visible to both files.

- [ ] **Step 3: Write `adapter_kitex.go`**

```go
package framework

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/byx-darwin/ncgo/internal/exec"
	"github.com/byx-darwin/ncgo/internal/manifest"
)

func init() { Register(kitexAdapter{}) }

type kitexAdapter struct{}

func (kitexAdapter) Kind() string { return manifest.KindKitex }

func (kitexAdapter) OptionalAssetPath(infraKind string) (string, bool) {
	return "kitex/optional/" + infraKind + ".go", true
}

func (kitexAdapter) HertzConfigAssetPath(infraKind string) (string, bool) {
	return "", false
}

func (kitexAdapter) MergeHertzConfig(infraKind, hertzConfigKey string, snippet, current []byte, force bool) (*ConfigWrite, error) {
	return nil, nil // Hertz-only in current behavior.
}

func (kitexAdapter) MergeRateLimitConfig(current []byte, force bool) (*ConfigWrite, error) {
	if current == nil {
		return &ConfigWrite{Body: []byte(defaultRateLimitConfBlock()), Action: "create"}, nil
	}
	merged, changed := mergeKitexRateLimitConfig(string(current))
	if !changed {
		return nil, nil
	}
	return &ConfigWrite{Body: []byte(merged), Action: "update"}, nil
}

// defaultRateLimitConfBlock/mergeKitexRateLimitConfig are moved verbatim from
// internal/scaffold/infra/infra.go:724-816.
func defaultRateLimitConfBlock() string {
	return `rate_limit:
  enabled: true
  mode: shadow
  backend: memory
  fail_open: true
  source:
    type: config
    cache_ttl_seconds: 60s
    fallback_on_error: true
  static:
    max_qps: 0
    max_connections: 0
`
}

func mergeKitexRateLimitConfig(src string) (string, bool) {
	if !hasTopLevelConfigKey(src, "rate_limit") {
		trimmed := strings.TrimRight(src, "\n")
		if trimmed == "" {
			return defaultRateLimitConfBlock(), true
		}
		return trimmed + "\n\n" + defaultRateLimitConfBlock(), true
	}
	lines := strings.Split(src, "\n")
	startIdx := -1
	endIdx := len(lines)
	childIndent := -1
	for i, line := range lines {
		if startIdx == -1 {
			if strings.HasPrefix(line, "rate_limit:") {
				startIdx = i
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			endIdx = i
			break
		}
		if childIndent == -1 {
			childIndent = len(line) - len(strings.TrimLeft(line, " \t"))
		}
	}
	if startIdx == -1 {
		return src, false
	}
	changed := false
	for i := startIdx + 1; i < endIdx; i++ {
		line := lines[i]
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if childIndent >= 0 && indent != childIndent {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "enabled:") {
			if !strings.Contains(trimmed, "true") {
				indentStr := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indentStr + "enabled: true"
				changed = true
			}
		} else if strings.HasPrefix(trimmed, "mode:") {
			if !strings.Contains(trimmed, "shadow") {
				indentStr := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				lines[i] = indentStr + "mode: shadow"
				changed = true
			}
		}
	}
	if !changed {
		return src, false
	}
	return strings.Join(lines, "\n"), true
}

func (kitexAdapter) DockerConfigBlocks(m *manifest.Manifest) []string {
	if !m.Service.WithDatabase {
		return nil
	}
	return []string{fmt.Sprintf("database:\n  enabled: true\n  dsn: %q", postgresDSN(m.Service.Name))}
}

func (kitexAdapter) ContainerPort() int { return 8888 }

func (kitexAdapter) ComposeFeatures(withDatabase bool) ComposeFeatureFlags {
	return ComposeFeatureFlags{}
}

func (kitexAdapter) IDLPath(opts GeneratorOptions) string {
	return filepath.ToSlash(filepath.Join("idl", kitexIDLBase(opts)+".proto"))
}

// kitexIDLBase matches the generated Makefile's
// `{{.ServiceInfo.ServiceName | ToLower}}` convention, and avoids invalid
// proto/Go identifiers for CLI names like "user-api".
func kitexIDLBase(opts GeneratorOptions) string {
	return strings.ToLower(exportName(opts.Name))
}

// exportName converts "user-api" to "UserApi" for use as a proto service
// name. Shared by both adapters' generated-name logic.
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

func (kitexAdapter) IDLNameToken(opts GeneratorOptions) string {
	return kitexIDLBase(opts)
}

func (kitexAdapter) GeneratorCommand(opts GeneratorOptions, idl string) string {
	return fmt.Sprintf("kitex -module %s -template-dir template/kitex-template -type protobuf %s", opts.Module, idl)
}

func (kitexAdapter) RenderIDLPlaceholder(opts GeneratorOptions) string {
	base := kitexIDLBase(opts)
	service := exportName(opts.Name)
	return fmt.Sprintf(`syntax = "proto3";

package %s;

option go_package = "%s/kitex_gen/%s;%s";

service %s {
}
`, base, opts.Module, base, base, service)
}

func (kitexAdapter) WriteIDLSupportFiles(dir string) error { return nil }

func (kitexAdapter) RunGenerator(ctx context.Context, r exec.Runner, dir string, opts GeneratorOptions, idl string) (exec.Result, error) {
	return exec.Kitex(ctx, r, dir, kitexArgs(opts, idl)...)
}

// kitexArgs mirrors the invocation documented in
// internal/assets/_data/kitex/kitex-template/makefile.yaml so `make update`
// later produces the same files.
func kitexArgs(opts GeneratorOptions, idl string) []string {
	return []string{
		"-module", opts.Module,
		"-template-dir", "template/kitex-template",
		"-type", "protobuf",
		idl,
	}
}

func (kitexAdapter) RequiresSQLCBeforeTidy(withDatabase bool) bool { return true }

func (kitexAdapter) ServerFilePath() string {
	return "internal/base/server/server.go"
}
```

- [ ] **Step 4: Write `adapter_hertz_test.go`**

```go
package framework

import (
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestHertzAdapterRegistered(t *testing.T) {
	a, ok := Get(manifest.KindHertz)
	if !ok || a.Kind() != manifest.KindHertz {
		t.Fatalf("Get(hertz) = %v, %v; want hertz adapter", a, ok)
	}
}

func TestHertzAdapterContainerPort(t *testing.T) {
	if got := (hertzAdapter{}).ContainerPort(); got != 8080 {
		t.Fatalf("ContainerPort() = %d, want 8080", got)
	}
}

func TestHertzAdapterIDLPath(t *testing.T) {
	got := (hertzAdapter{}).IDLPath(GeneratorOptions{Name: "demo", Module: "github.com/x/demo"})
	want := "idl/app/demo.proto"
	if got != want {
		t.Fatalf("IDLPath = %q, want %q", got, want)
	}
}

func TestHertzAdapterIDLNameToken(t *testing.T) {
	got := (hertzAdapter{}).IDLNameToken(GeneratorOptions{Name: "Demo"})
	if got != "demo" {
		t.Fatalf("IDLNameToken = %q, want %q", got, "demo")
	}
}

func TestHertzAdapterRequiresSQLCBeforeTidy(t *testing.T) {
	a := hertzAdapter{}
	if a.RequiresSQLCBeforeTidy(false) {
		t.Fatal("RequiresSQLCBeforeTidy(false) = true, want false")
	}
	if !a.RequiresSQLCBeforeTidy(true) {
		t.Fatal("RequiresSQLCBeforeTidy(true) = false, want true")
	}
}

func TestHertzAdapterOptionalAssetPath(t *testing.T) {
	path, ok := (hertzAdapter{}).OptionalAssetPath("redis")
	if !ok || path != "hertz/optional/redis.go" {
		t.Fatalf("OptionalAssetPath(redis) = %q, %v; want hertz/optional/redis.go, true", path, ok)
	}
}

func TestHertzAdapterHertzConfigAssetPath(t *testing.T) {
	path, ok := (hertzAdapter{}).HertzConfigAssetPath("redis")
	if !ok || path != "hertz/optional-config/redis.yaml" {
		t.Fatalf("HertzConfigAssetPath(redis) = %q, %v; want hertz/optional-config/redis.yaml, true", path, ok)
	}
}

func TestHertzAdapterComposeFeatures(t *testing.T) {
	a := hertzAdapter{}
	got := a.ComposeFeatures(false)
	want := ComposeFeatureFlags{Nacos: true, Polaris: true, Vegeta: false}
	if got != want {
		t.Fatalf("ComposeFeatures(false) = %+v, want %+v", got, want)
	}
	got = a.ComposeFeatures(true)
	want = ComposeFeatureFlags{Nacos: true, Polaris: true, Vegeta: true}
	if got != want {
		t.Fatalf("ComposeFeatures(true) = %+v, want %+v", got, want)
	}
}

func TestHertzAdapterGeneratorCommand(t *testing.T) {
	got := (hertzAdapter{}).GeneratorCommand(GeneratorOptions{Module: "github.com/x/demo"}, "idl/app/demo.proto")
	want := "hz new --mod=github.com/x/demo --idl=idl/app/demo.proto -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml"
	if got != want {
		t.Fatalf("GeneratorCommand = %q, want %q", got, want)
	}
}

func TestHertzAdapterMergeHertzConfigCreatesWhenAbsent(t *testing.T) {
	write, err := (hertzAdapter{}).MergeHertzConfig("redis", "redis", []byte("addrs:\n  - localhost:6379\n"), nil, false)
	if err != nil {
		t.Fatalf("MergeHertzConfig: %v", err)
	}
	if write == nil || write.Action != "create" {
		t.Fatalf("MergeHertzConfig = %+v, want Action=create", write)
	}
}

func TestHertzAdapterMergeHertzConfigSkipsExistingKey(t *testing.T) {
	current := []byte("redis:\n  addrs:\n    - localhost:6379\n")
	write, err := (hertzAdapter{}).MergeHertzConfig("redis", "redis", []byte("addrs:\n  - localhost:6379\n"), current, false)
	if err != nil {
		t.Fatalf("MergeHertzConfig: %v", err)
	}
	if write != nil {
		t.Fatalf("MergeHertzConfig = %+v, want nil (key already present)", write)
	}
}

func TestHertzAdapterMergeRateLimitConfigIsNoop(t *testing.T) {
	write, err := (hertzAdapter{}).MergeRateLimitConfig(nil, false)
	if err != nil || write != nil {
		t.Fatalf("MergeRateLimitConfig = %+v, %v; want nil, nil", write, err)
	}
}
```

- [ ] **Step 5: Write `adapter_kitex_test.go`**

```go
package framework

import (
	"testing"

	"github.com/byx-darwin/ncgo/internal/manifest"
)

func TestKitexAdapterRegistered(t *testing.T) {
	a, ok := Get(manifest.KindKitex)
	if !ok || a.Kind() != manifest.KindKitex {
		t.Fatalf("Get(kitex) = %v, %v; want kitex adapter", a, ok)
	}
}

func TestKitexAdapterContainerPort(t *testing.T) {
	if got := (kitexAdapter{}).ContainerPort(); got != 8888 {
		t.Fatalf("ContainerPort() = %d, want 8888", got)
	}
}

func TestKitexAdapterIDLPath(t *testing.T) {
	got := (kitexAdapter{}).IDLPath(GeneratorOptions{Name: "user-api", Module: "github.com/x/user-api"})
	want := "idl/userapi.proto"
	if got != want {
		t.Fatalf("IDLPath = %q, want %q", got, want)
	}
}

func TestKitexAdapterIDLNameToken(t *testing.T) {
	got := (kitexAdapter{}).IDLNameToken(GeneratorOptions{Name: "user-api"})
	if got != "userapi" {
		t.Fatalf("IDLNameToken = %q, want %q", got, "userapi")
	}
}

func TestKitexAdapterRequiresSQLCBeforeTidyAlwaysTrue(t *testing.T) {
	a := kitexAdapter{}
	if !a.RequiresSQLCBeforeTidy(false) {
		t.Fatal("RequiresSQLCBeforeTidy(false) = false, want true (kitex always requires sqlc-before-tidy)")
	}
	if !a.RequiresSQLCBeforeTidy(true) {
		t.Fatal("RequiresSQLCBeforeTidy(true) = false, want true")
	}
}

func TestKitexAdapterHertzConfigAssetPathAlwaysFalse(t *testing.T) {
	path, ok := (kitexAdapter{}).HertzConfigAssetPath("redis")
	if ok || path != "" {
		t.Fatalf("HertzConfigAssetPath(redis) = %q, %v; want \"\", false", path, ok)
	}
}

func TestKitexAdapterComposeFeaturesAllFalse(t *testing.T) {
	got := (kitexAdapter{}).ComposeFeatures(true)
	want := ComposeFeatureFlags{}
	if got != want {
		t.Fatalf("ComposeFeatures(true) = %+v, want %+v", got, want)
	}
}

func TestKitexAdapterGeneratorCommand(t *testing.T) {
	got := (kitexAdapter{}).GeneratorCommand(GeneratorOptions{Module: "github.com/x/demo"}, "idl/demo.proto")
	want := "kitex -module github.com/x/demo -template-dir template/kitex-template -type protobuf idl/demo.proto"
	if got != want {
		t.Fatalf("GeneratorCommand = %q, want %q", got, want)
	}
}

func TestKitexAdapterMergeRateLimitConfigCreatesWhenAbsent(t *testing.T) {
	write, err := (kitexAdapter{}).MergeRateLimitConfig(nil, false)
	if err != nil {
		t.Fatalf("MergeRateLimitConfig: %v", err)
	}
	if write == nil || write.Action != "create" {
		t.Fatalf("MergeRateLimitConfig = %+v, want Action=create", write)
	}
}

func TestKitexAdapterMergeRateLimitConfigFlipsExistingBlock(t *testing.T) {
	current := []byte("rate_limit:\n  enabled: false\n  mode: enforce\n")
	write, err := (kitexAdapter{}).MergeRateLimitConfig(current, false)
	if err != nil {
		t.Fatalf("MergeRateLimitConfig: %v", err)
	}
	if write == nil || write.Action != "update" {
		t.Fatalf("MergeRateLimitConfig = %+v, want Action=update", write)
	}
}

func TestKitexAdapterMergeHertzConfigIsNoop(t *testing.T) {
	write, err := (kitexAdapter{}).MergeHertzConfig("redis", "redis", nil, nil, false)
	if err != nil || write != nil {
		t.Fatalf("MergeHertzConfig = %+v, %v; want nil, nil", write, err)
	}
}
```

- [ ] **Step 6: Build and test**

Run: `gofmt -l internal/scaffold/framework && go vet ./internal/scaffold/framework/... && go test ./internal/scaffold/framework/... -count=1 -v`
Expected: `gofmt -l` prints nothing; all tests PASS (including the four from Task 1, now that both adapters are registered).

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/framework/
git commit -m "feat(framework): add hertzAdapter and kitexAdapter implementations"
```

---

## Task 3: Migrate `internal/scaffold/mono/mono.go`

**Files:**
- Modify: `internal/scaffold/mono/mono.go:239-244` (`defaultIDL`), `:253-261` (`runGenerator`), `:279-284` (`validate`'s Kind switch)
- Test: `internal/scaffold/mono/golden_test.go` (run existing tests, no new test file needed — behavior is covered by golden trees plus `internal/scaffold/mono` unit tests already asserting `Options.validate()` errors)

**Interfaces:**
- Consumes: `framework.MustGet(kind string) framework.Adapter`, `framework.Get(kind string) (framework.Adapter, bool)`, `framework.GeneratorOptions{Name, Module string}` from Task 2.
- Produces: no new exported symbols; `defaultIDL`/`runGenerator`/`validate` keep their existing signatures and call sites unchanged.

- [ ] **Step 1: Add the import**

In `internal/scaffold/mono/mono.go`, add `"github.com/byx-darwin/ncgo/internal/scaffold/framework"` to the import block (after `"github.com/byx-darwin/ncgo/internal/manifest"`).

- [ ] **Step 2: Replace `defaultIDL`**

Before (`mono.go:239-244`):
```go
func defaultIDL(opts Options) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return filepath.ToSlash(filepath.Join("idl", kitexIDLBase(opts)+".proto"))
	}
	return filepath.ToSlash(filepath.Join("idl", "app", opts.Name+".proto"))
}
```

After:
```go
func defaultIDL(opts Options) string {
	return framework.MustGet(defaultKind(opts.Kind)).IDLPath(framework.GeneratorOptions{Name: opts.Name, Module: opts.Module})
}
```

- [ ] **Step 3: Replace `runGenerator`**

Before (`mono.go:254-261`):
```go
func runGenerator(ctx context.Context, r exec.Runner, dir string, opts Options, idl string) (exec.Result, error) {
	switch defaultKind(opts.Kind) {
	case manifest.KindKitex:
		return exec.Kitex(ctx, r, dir, kitexArgs(opts, idl)...)
	default:
		return exec.HZ(ctx, r, dir, hzArgs(opts, idl)...)
	}
}
```

After:
```go
func runGenerator(ctx context.Context, r exec.Runner, dir string, opts Options, idl string) (exec.Result, error) {
	adapter := framework.MustGet(defaultKind(opts.Kind))
	return adapter.RunGenerator(ctx, r, dir, framework.GeneratorOptions{Name: opts.Name, Module: opts.Module}, idl)
}
```

Note: `hzArgs`/`kitexArgs` in `mono.go:337-363` become dead code after this step — do NOT delete them yet (Task 8 does a final dead-code sweep after all migrations land, so a partial deletion here doesn't leave other still-in-progress call sites broken). `go vet`/`unused` won't flag unused package-level funcs, so leaving them temporarily is safe.

- [ ] **Step 4: Replace the Kind validity switch in `validate`**

Before (`mono.go:279-284`):
```go
	switch defaultKind(o.Kind) {
	case manifest.KindHertz, manifest.KindKitex:
	default:
		return fmt.Errorf("scaffold: kind %q is invalid (hertz|kitex)", o.Kind)
	}
	return nil
}
```

After:
```go
	if _, ok := framework.Get(defaultKind(o.Kind)); !ok {
		return fmt.Errorf("scaffold: kind %q is invalid (hertz|kitex)", o.Kind)
	}
	return nil
}
```

This preserves the exact error message (contract-sensitive per `.claude/rules/go.md` §4 — CLI-adjacent error text).

- [ ] **Step 5: Build**

Run: `go build ./internal/scaffold/mono/...`
Expected: succeeds. (`manifest` import in `mono.go` stays in use — `manifest.KindKitex` is still referenced at `mono.go:146,177,190,197` per the Documented Scope Decisions above.)

- [ ] **Step 6: Run the golden tests — must show zero diff**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenWithDatabase -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexDefault -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenWithRuleCenter -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenTemplateRuleCenter -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexWithDatabase -count=1 -v`

Expected: all six PASS with no diff output. If any fails, do **not** run `-update-golden` — that would bless a behavior change this refactor must not introduce. Debug the migrated function instead.

- [ ] **Step 7: Run the full mono package test suite**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS (covers `Options.validate()` error-message assertions and other non-golden unit tests in the package).

- [ ] **Step 8: Commit**

```bash
git add internal/scaffold/mono/mono.go
git commit -m "refactor(mono): route defaultIDL/runGenerator/validate through framework.Adapter"
```

---

## Task 4: Migrate `internal/scaffold/mono/files.go`

**Files:**
- Modify: `internal/scaffold/mono/files.go:629-638` (`idlNameToken`), `:640-680` (`writeIDLPlaceholder`'s Hertz-only branch), `:747-826` (`renderIDLPlaceholder` — becomes a thin wrapper), `:896-903` (`generatorCommand`), `:934-940` (`requiresSQLCBeforeTidy`)
- Test: `internal/scaffold/mono/golden_test.go` (same six golden tests as Task 3)

**Interfaces:**
- Consumes: `framework.MustGet(kind string) framework.Adapter`, `framework.GeneratorOptions{Name, Module string}` from Task 2.
- Produces: `idlNameToken`, `renderIDLPlaceholder`, `generatorCommand`, `requiresSQLCBeforeTidy` keep their existing signatures — callers elsewhere in `mono` (`mono.go:120`, `files.go:599,675,880-902,922-924`) are unaffected.

- [ ] **Step 1: Add the import**

Add `"github.com/byx-darwin/ncgo/internal/scaffold/framework"` to `files.go`'s import block.

- [ ] **Step 2: Replace `idlNameToken`**

Before (`files.go:633-638`):
```go
func idlNameToken(opts Options) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return kitexIDLBase(opts)
	}
	return strings.ToLower(opts.Name)
}
```

After:
```go
func idlNameToken(opts Options) string {
	return framework.MustGet(defaultKind(opts.Kind)).IDLNameToken(framework.GeneratorOptions{Name: opts.Name, Module: opts.Module})
}
```

Note: `kitexIDLBase(opts Options) string` at `mono.go:249-251` becomes dead code after this and Task 3's `defaultIDL` change — leave it for Task 8's sweep (still referenced nowhere else after this step; verify with `grep -rn kitexIDLBase internal/scaffold/mono/` before deleting in Task 8).

- [ ] **Step 3: Replace the Hertz-only branch in `writeIDLPlaceholder`**

Before (`files.go:650-655`):
```go
func writeIDLPlaceholder(dir, idl string, opts Options) error {
	if defaultKind(opts.Kind) == manifest.KindHertz {
		if err := writeHertzProtoSupportFiles(dir); err != nil {
			return err
		}
	}
	full := filepath.Join(dir, filepath.FromSlash(idl))
```

After:
```go
func writeIDLPlaceholder(dir, idl string, opts Options) error {
	if err := framework.MustGet(defaultKind(opts.Kind)).WriteIDLSupportFiles(dir); err != nil {
		return err
	}
	full := filepath.Join(dir, filepath.FromSlash(idl))
```

`writeHertzProtoSupportFiles`, `writeHertzAPIProto`, and the `hertzAPIProto` const (`files.go:20-87,700-734`) become dead code — leave for Task 8 (their logic now lives in `framework/adapter_hertz.go`'s `WriteIDLSupportFiles`).

- [ ] **Step 4: Replace `renderIDLPlaceholder`**

Before (`files.go:747-826`, ~80 lines with per-kind bodies inline):
```go
func renderIDLPlaceholder(opts Options) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		base := kitexIDLBase(opts)
		service := exportName(opts.Name)
		return fmt.Sprintf(`syntax = "proto3";
...
`, base, opts.Module, base, base, service)
	}
	service := exportName(opts.Name)
	return strings.Join([]string{
		...
	}, "\n")
}
```

After:
```go
func renderIDLPlaceholder(opts Options) string {
	return framework.MustGet(defaultKind(opts.Kind)).RenderIDLPlaceholder(framework.GeneratorOptions{Name: opts.Name, Module: opts.Module})
}
```

Delete the ~78 lines of inline body content between the old `func renderIDLPlaceholder(opts Options) string {` and its closing `}` — that content was moved verbatim into `framework/adapter_hertz.go` and `framework/adapter_kitex.go` in Task 2, Steps 2-3.

- [ ] **Step 5: Replace `generatorCommand`**

Before (`files.go:898-903`):
```go
func generatorCommand(opts Options, idl string) string {
	if defaultKind(opts.Kind) == manifest.KindKitex {
		return fmt.Sprintf("kitex -module %s -template-dir template/kitex-template -type protobuf %s", opts.Module, idl)
	}
	return fmt.Sprintf("hz new --mod=%s --idl=%s -I idl --handler_dir=internal/handler --model_dir=internal/pb --router_dir=internal/router --customize_layout=template/layout.yaml --customize_layout_data_path=template/data.json --customize_package=template/package.yaml", opts.Module, idl)
}
```

After:
```go
func generatorCommand(opts Options, idl string) string {
	return framework.MustGet(defaultKind(opts.Kind)).GeneratorCommand(framework.GeneratorOptions{Name: opts.Name, Module: opts.Module}, idl)
}
```

- [ ] **Step 6: Replace `requiresSQLCBeforeTidy`**

Before (`files.go:938-940`):
```go
func requiresSQLCBeforeTidy(opts Options) bool {
	return defaultKind(opts.Kind) == manifest.KindKitex || opts.WithDatabase
}
```

After:
```go
func requiresSQLCBeforeTidy(opts Options) bool {
	return framework.MustGet(defaultKind(opts.Kind)).RequiresSQLCBeforeTidy(opts.WithDatabase)
}
```

Verify: `kitexAdapter.RequiresSQLCBeforeTidy` always returns `true` regardless of its `withDatabase` argument (Task 2, adapter_kitex.go), and `hertzAdapter.RequiresSQLCBeforeTidy` returns `withDatabase` — together reproducing `defaultKind(opts.Kind) == manifest.KindKitex || opts.WithDatabase` exactly for both kinds.

- [ ] **Step 7: Build**

Run: `go build ./internal/scaffold/mono/...`
Expected: succeeds. `writeTemplate` (`files.go:93-98`) is untouched per Documented Scope Decision #4 — it still does `if defaultKind(opts.Kind) == manifest.KindKitex { ... }`, so `manifest` import stays in use.

- [ ] **Step 8: Run the golden tests — must show zero diff**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenDefault -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenWithDatabase -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexDefault -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenWithRuleCenter -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenTemplateRuleCenter -count=1 -v`
Run: `go test ./internal/scaffold/mono/... -run TestGenerateGoldenKitexWithDatabase -count=1 -v`

Expected: all six PASS with zero diff. This is the highest-risk task in the plan (it touches the actual IDL placeholder content and proto support files byte-for-byte) — if any golden test fails, diff the generated tree against `internal/scaffold/mono/testdata/` manually before considering `-update-golden`.

- [ ] **Step 9: Run the full mono package test suite**

Run: `go test ./internal/scaffold/mono/... -count=1`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/scaffold/mono/files.go
git commit -m "refactor(mono): route idlNameToken/renderIDLPlaceholder/generatorCommand/requiresSQLCBeforeTidy through framework.Adapter"
```

---

## Task 5: Migrate `internal/scaffold/infra/infra.go`

**Files:**
- Modify: `internal/scaffold/infra/infra.go:375-407` (`planHertzConfigWrite`), `:676-683` (`planKitexRateLimitConfigWrite`), `:818-830` (`frameworkAssetFiles`)
- Test: `internal/scaffold/infra/*_test.go` (existing plugin tests — run the whole package)

**Interfaces:**
- Consumes: `framework.Get(kind string) (framework.Adapter, bool)` from Task 2.
- Produces: `planHertzConfigWrite`, `planKitexRateLimitConfigWrite`, `frameworkAssetFiles` keep their existing signatures; `Add` (`infra.go:216-341`) calls them unchanged.

- [ ] **Step 1: Add the import**

Add `"github.com/byx-darwin/ncgo/internal/scaffold/framework"` to `infra.go`'s import block.

- [ ] **Step 2: Replace `frameworkAssetFiles`**

Before (`infra.go:818-830`):
```go
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

After:
```go
func frameworkAssetFiles(infraKind, outputRelPath string) func(serviceKind string) ([]addOnFile, error) {
	return func(serviceKind string) ([]addOnFile, error) {
		adapter, ok := framework.Get(serviceKind)
		if !ok {
			return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
		}
		path, ok := adapter.OptionalAssetPath(infraKind)
		if !ok {
			return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
		}
		return []addOnFile{{SourcePath: path, OutputRelPath: outputRelPath}}, nil
	}
}
```

- [ ] **Step 3: Replace `planHertzConfigWrite`**

Before (`infra.go:375-407`):
```go
func planHertzConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	if serviceKind != manifest.KindHertz {
		return nil, nil
	}
	p, ok := pluginByKind(infraKind)
	if !ok || p.HertzConfigKey() == "" {
		return nil, nil
	}
	snippet, err := fs.ReadFile(assets.FS(), filepath.ToSlash(filepath.Join("hertz", "optional-config", infraKind+".yaml")))
	if err != nil {
		return nil, fmt.Errorf("infra: read embedded hertz/optional-config/%s.yaml: %w", infraKind, err)
	}
	path := filepath.Join(root, filepath.FromSlash(hertzConfigRelPath))
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &plannedWrite{
				Path:   path,
				Body:   []byte(wrapHertzConfigSnippet(string(snippet), infraKind) + "\n"),
				Action: "create",
			}, nil
		}
		return nil, fmt.Errorf("infra: read %s: %w", path, err)
	}
	merged, changed, err := mergeHertzConfig(current, string(snippet), infraKind, p.HertzConfigKey(), force)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, nil
	}
	return &plannedWrite{Path: path, Body: []byte(merged), Action: "update"}, nil
}
```

After:
```go
func planHertzConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	adapter, ok := framework.Get(serviceKind)
	if !ok {
		return nil, nil
	}
	assetPath, ok := adapter.HertzConfigAssetPath(infraKind)
	if !ok {
		return nil, nil
	}
	p, ok := pluginByKind(infraKind)
	if !ok || p.HertzConfigKey() == "" {
		return nil, nil
	}
	snippet, err := fs.ReadFile(assets.FS(), assetPath)
	if err != nil {
		return nil, fmt.Errorf("infra: read embedded %s: %w", assetPath, err)
	}
	path := filepath.Join(root, filepath.FromSlash(hertzConfigRelPath))
	current, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("infra: read %s: %w", path, err)
		}
		current = nil
	}
	write, err := adapter.MergeHertzConfig(infraKind, p.HertzConfigKey(), snippet, current, force)
	if err != nil || write == nil {
		return nil, err
	}
	return &plannedWrite{Path: path, Body: write.Body, Action: write.Action}, nil
}
```

This is behavior-preserving: `kitexAdapter.HertzConfigAssetPath` always returns `("", false)`, so for `serviceKind == manifest.KindKitex` this returns `(nil, nil)` at the second check — identical to the original's `if serviceKind != manifest.KindHertz { return nil, nil }` short-circuit.

`mergeHertzConfig`, `wrapHertzConfigSnippet`, `hertzConfigMarkers`, `replaceMarkedHertzConfigBlock`, `hasTopLevelConfigKey` (`infra.go:409-481`, minus the `mergeKitexRateLimitConfig` usage of `hasTopLevelConfigKey` handled in Step 4 below) become dead code in this file — leave for Task 8.

- [ ] **Step 4: Replace `planKitexRateLimitConfigWrite`**

Before (`infra.go:676-683`, plus its callee `planKitexRateLimitConfig` at `:703-722`):
```go
func planKitexRateLimitConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	if serviceKind != manifest.KindKitex || infraKind != KindRateLimit {
		return nil, nil
	}
	return planKitexRateLimitConfig(root, force)
}
```

After:
```go
func planKitexRateLimitConfigWrite(root, serviceKind, infraKind string, force bool) (*plannedWrite, error) {
	if infraKind != KindRateLimit {
		return nil, nil
	}
	adapter, ok := framework.Get(serviceKind)
	if !ok {
		return nil, nil
	}
	path := kitexRateLimitConfPath(root)
	current, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("infra: read %s: %w", path, err)
		}
		current = nil
	}
	write, err := adapter.MergeRateLimitConfig(current, force)
	if err != nil || write == nil {
		return nil, err
	}
	return &plannedWrite{Path: path, Body: write.Body, Action: write.Action}, nil
}
```

This is behavior-preserving: for `serviceKind == manifest.KindHertz`, `hertzAdapter.MergeRateLimitConfig` always returns `(nil, nil)` — identical to the original's `if serviceKind != manifest.KindKitex { return nil, nil }` short-circuit. `force` was already an unused parameter in the original `planKitexRateLimitConfig`/`mergeKitexRateLimitConfig` — this migration preserves that (the parameter is still threaded through for interface symmetry with `MergeHertzConfig`, but neither adapter's `MergeRateLimitConfig` implementation reads it).

`planKitexRateLimitConfig`, `defaultRateLimitConfBlock`, `mergeKitexRateLimitConfig` (`infra.go:703-816`) become dead code — leave for Task 8. `kitexRateLimitConfPath` (`infra.go:672-674`) stays — still called above.

- [ ] **Step 5: Build**

Run: `go build ./internal/scaffold/infra/...`
Expected: succeeds. `manifest` import stays in use elsewhere in `infra.go` (e.g. `manifest.Load`, `manifest.Manifest`).

- [ ] **Step 6: Run the infra package test suite**

Run: `go test ./internal/scaffold/infra/... -count=1 -v`
Expected: all tests PASS, including `internal/scaffold/infra/registry_test.go` and every `plugin_*_test.go` (these exercise `Add()` end-to-end for each infra kind against both service kinds, so they cover `planHertzConfigWrite`/`planKitexRateLimitConfigWrite`/`frameworkAssetFiles` transitively).

- [ ] **Step 7: Run mono golden tests as a cross-package smoke check**

Run: `go test ./internal/scaffold/mono/... -run TestGenerateGolden -count=1`
Expected: PASS (mono's `Generate` calls `addSelectedInfra` → `scaffoldinfra.Add`, which now routes through `framework`; this confirms the wiring holds end-to-end).

- [ ] **Step 8: Commit**

```bash
git add internal/scaffold/infra/infra.go
git commit -m "refactor(infra): route planHertzConfigWrite/planKitexRateLimitConfigWrite/frameworkAssetFiles through framework.Adapter"
```

---

## Task 6: Migrate `internal/scaffold/infra/wire.go`

**Files:**
- Modify: `internal/scaffold/infra/wire.go:47-59` (`wire` dispatcher), `:82-85` (delete local Kind consts), `:99-101` (`wireHertz` path literal), `:128-129` (`wireKitex` path literal)
- Test: `internal/scaffold/infra/*_test.go` (existing `--wire` tests, run the whole package)

**Interfaces:**
- Consumes: `framework.MustGet(kind string) framework.Adapter` (for `.Kind()`/`.ServerFilePath()`) from Task 2; `manifest.KindHertz`/`manifest.KindKitex` from `internal/manifest` (new import in this file).
- Produces: `wire`, `wireHertz`, `wireKitex` keep their existing signatures.

- [ ] **Step 1: Add imports, delete local Kind consts**

Before (`wire.go:82-97`):
```go
const (
	manifestKindHertz = "hertz"
	manifestKindKitex = "kitex"

	anchorSourceMarker = "marker"
	anchorSourceLegacy = "legacy"

	markerLoggingInit               = "// ncgo:wire:logging:init"
	markerLoggingServerMiddleware   = "// ncgo:wire:logging:server-middleware"
	markerCanaryServerTraffic       = "// ncgo:wire:canary:server-traffic"
	markerKitexClientMiddleware     = "// ncgo:wire:kitex-client:middleware"
	markerRegistryServer            = "// ncgo:wire:registry:server"
	markerRegistryClient            = "// ncgo:wire:registry:client"
	markerRateLimitServerMiddleware = "// ncgo:wire:ratelimit:server-middleware"
	markerRateLimitStaticLimit      = "// ncgo:wire:ratelimit:static-limit"
)
```

After:
```go
const (
	anchorSourceMarker = "marker"
	anchorSourceLegacy = "legacy"

	markerLoggingInit               = "// ncgo:wire:logging:init"
	markerLoggingServerMiddleware   = "// ncgo:wire:logging:server-middleware"
	markerCanaryServerTraffic       = "// ncgo:wire:canary:server-traffic"
	markerKitexClientMiddleware     = "// ncgo:wire:kitex-client:middleware"
	markerRegistryServer            = "// ncgo:wire:registry:server"
	markerRegistryClient            = "// ncgo:wire:registry:client"
	markerRateLimitServerMiddleware = "// ncgo:wire:ratelimit:server-middleware"
	markerRateLimitStaticLimit      = "// ncgo:wire:ratelimit:static-limit"
)
```

Add to `wire.go`'s import block (currently `"fmt"`, `"go/format"`, `"os"`, `"path/filepath"`, `"strings"`):
```go
"github.com/byx-darwin/ncgo/internal/manifest"
"github.com/byx-darwin/ncgo/internal/scaffold/framework"
```

- [ ] **Step 2: Replace the two dispatcher cases in `wire`**

Before (`wire.go:47-59`):
```go
func wire(root, module, serviceKind, kind string, dryRun bool) (*wireResult, error) {
	if !wireSupportedKind(kind) {
		return nil, unsupportedWireError()
	}
	switch serviceKind {
	case manifestKindHertz:
		return wireHertz(root, module, kind, dryRun)
	case manifestKindKitex:
		return wireKitex(root, module, kind, dryRun)
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
}
```

After:
```go
func wire(root, module, serviceKind, kind string, dryRun bool) (*wireResult, error) {
	if !wireSupportedKind(kind) {
		return nil, unsupportedWireError()
	}
	switch serviceKind {
	case manifest.KindHertz:
		return wireHertz(root, module, kind, dryRun)
	case manifest.KindKitex:
		return wireKitex(root, module, kind, dryRun)
	default:
		return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
	}
}
```

- [ ] **Step 3: Replace the hardcoded server-file path literal in `wireHertz`**

Before (`wire.go:99-101`):
```go
func wireHertz(root, module, kind string, dryRun bool) (*wireResult, error) {
	path := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(path)
```

After:
```go
func wireHertz(root, module, kind string, dryRun bool) (*wireResult, error) {
	path := filepath.Join(root, filepath.FromSlash(framework.MustGet(manifest.KindHertz).ServerFilePath()))
	body, err := readSource(path)
```

- [ ] **Step 4: Replace the hardcoded server-file path literal in `wireKitex`**

Before (`wire.go:125-129`):
```go
func wireKitex(root, module, kind string, dryRun bool) (*wireResult, error) {
	paths := []string{}
	plan := []PlanItem(nil)
	serverPath := filepath.Join(root, "internal", "base", "server", "server.go")
	body, err := readSource(serverPath)
```

After:
```go
func wireKitex(root, module, kind string, dryRun bool) (*wireResult, error) {
	paths := []string{}
	plan := []PlanItem(nil)
	serverPath := filepath.Join(root, filepath.FromSlash(framework.MustGet(manifest.KindKitex).ServerFilePath()))
	body, err := readSource(serverPath)
```

- [ ] **Step 5: Build**

Run: `go build ./internal/scaffold/infra/...`
Expected: succeeds.

- [ ] **Step 6: Run the infra package test suite**

Run: `go test ./internal/scaffold/infra/... -count=1 -v`
Expected: all tests PASS, including any `TestWire*`/`--wire`-flagged tests in `plugin_*_test.go` and `internal/cli/add_test.go` if it exercises `--wire` (run `go test ./internal/cli/... -run TestAdd -count=1` as well if that test name exists — check with `grep -rn "func TestAdd" internal/cli/add_test.go` first).

- [ ] **Step 7: Commit**

```bash
git add internal/scaffold/infra/wire.go
git commit -m "refactor(infra): resolve --wire server file path via framework.Adapter, drop duplicate Kind consts"
```

---

## Task 7: Migrate `internal/scaffold/shared/container.go`

**Files:**
- Modify: `internal/scaffold/shared/container.go:501-516` (`renderServiceDockerConfig`), `:518-566` (delete `renderHertzDockerConfigBlocks`), `:568-573` (delete `renderKitexDockerConfigBlocks`), `:575-603` (`composeFeaturesForApp`), `:705-714` (`servicePort`)
- Test: `internal/scaffold/shared/*_test.go` (existing container/compose tests, run the whole package) plus mono golden tests (compose.yaml/conf/docker/conf.yaml are part of the golden tree)

**Interfaces:**
- Consumes: `framework.Get(kind string) (framework.Adapter, bool)`, `framework.ComposeFeatureFlags{Nacos, Polaris, Vegeta bool}` from Task 2.
- Produces: `servicePort`, `renderServiceDockerConfig`, `composeFeaturesForApp` keep their existing signatures.

- [ ] **Step 1: Add the import**

Add `"github.com/byx-darwin/ncgo/internal/scaffold/framework"` to `container.go`'s import block.

- [ ] **Step 2: Replace `servicePort`**

Before (`container.go:705-714`):
```go
func servicePort(kind string) (int, error) {
	switch kind {
	case "", manifest.KindHertz:
		return hertzContainerPort, nil
	case manifest.KindKitex:
		return kitexContainerPort, nil
	default:
		return 0, fmt.Errorf("scaffold: unsupported service kind %q for container files", kind)
	}
}
```

After:
```go
func servicePort(kind string) (int, error) {
	if kind == "" {
		kind = manifest.KindHertz
	}
	adapter, ok := framework.Get(kind)
	if !ok {
		return 0, fmt.Errorf("scaffold: unsupported service kind %q for container files", kind)
	}
	return adapter.ContainerPort(), nil
}
```

`hertzContainerPort`/`kitexContainerPort` consts (`container.go:14-15`) become dead code — leave for Task 8.

- [ ] **Step 3: Replace `renderServiceDockerConfig` and delete the two `render*DockerConfigBlocks` functions**

Before (`container.go:501-573`):
```go
func renderServiceDockerConfig(m *manifest.Manifest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("scaffold: render docker config: nil manifest")
	}
	var blocks []string
	blocks = append(blocks, "# Generated by ncgo for docker-compose based local development.\nenv: docker")
	switch m.Service.Kind {
	case "", manifest.KindHertz:
		blocks = append(blocks, renderHertzDockerConfigBlocks(m)...)
	case manifest.KindKitex:
		blocks = append(blocks, renderKitexDockerConfigBlocks(m)...)
	default:
		return "", fmt.Errorf("scaffold: unsupported service kind %q for docker config", m.Service.Kind)
	}
	return strings.Join(blocks, "\n\n") + "\n", nil
}

func renderHertzDockerConfigBlocks(m *manifest.Manifest) []string {
	... (48 lines) ...
}

func renderKitexDockerConfigBlocks(m *manifest.Manifest) []string {
	if !m.Service.WithDatabase {
		return nil
	}
	return []string{fmt.Sprintf("database:\n  enabled: true\n  dsn: %q", postgresDSN(m.Service.Name))}
}
```

After:
```go
func renderServiceDockerConfig(m *manifest.Manifest) (string, error) {
	if m == nil {
		return "", fmt.Errorf("scaffold: render docker config: nil manifest")
	}
	kind := m.Service.Kind
	if kind == "" {
		kind = manifest.KindHertz
	}
	adapter, ok := framework.Get(kind)
	if !ok {
		return "", fmt.Errorf("scaffold: unsupported service kind %q for docker config", m.Service.Kind)
	}
	blocks := []string{"# Generated by ncgo for docker-compose based local development.\nenv: docker"}
	blocks = append(blocks, adapter.DockerConfigBlocks(m)...)
	return strings.Join(blocks, "\n\n") + "\n", nil
}
```

Delete `renderHertzDockerConfigBlocks` and `renderKitexDockerConfigBlocks` entirely — their bodies were moved verbatim into `framework/adapter_hertz.go` and `framework/adapter_kitex.go` in Task 2. (Unlike other tasks' dead-code, delete these now rather than in Task 8: they reference `infraRedis`/`infraKafka`/`infraES`/`infraClickHouse` package-level consts that stay in use elsewhere in this file via `composeFeaturesForApp`, so leaving unreferenced duplicate logic around here is more confusing than helpful — but this is optional tidiness, not required for correctness. If in doubt, leave them and let Task 8's sweep catch it.)

- [ ] **Step 4: Replace the Kind-dependent part of `composeFeaturesForApp`**

Before (`container.go:575-603`):
```go
func composeFeaturesForApp(app composeApp) composeFeatures {
	features := composeFeatures{postgres: app.WithDatabase}
	for _, kind := range app.Infra {
		switch kind {
		case infraRedis:
			features.redis = true
		case infraKafka:
			features.kafka = true
		case infraES:
			features.es = true
		case infraClickHouse:
			features.ch = true
		case infraRegistryPolaris:
			features.polaris = true
		case infraReleaseCanary:
			features.nacos = true
			features.polaris = true
		}
	}
	if app.Kind == manifest.KindHertz {
		features.nacos = true
		features.polaris = true
	}
	// Vegeta is available for Hertz services with postgres (rate-limit E2E testing).
	if app.Kind == manifest.KindHertz && app.WithDatabase {
		features.vegeta = true
	}
	return features
}
```

After:
```go
func composeFeaturesForApp(app composeApp) composeFeatures {
	features := composeFeatures{postgres: app.WithDatabase}
	for _, kind := range app.Infra {
		switch kind {
		case infraRedis:
			features.redis = true
		case infraKafka:
			features.kafka = true
		case infraES:
			features.es = true
		case infraClickHouse:
			features.ch = true
		case infraRegistryPolaris:
			features.polaris = true
		case infraReleaseCanary:
			features.nacos = true
			features.polaris = true
		}
	}
	appKind := app.Kind
	if appKind == "" {
		appKind = manifest.KindHertz
	}
	if adapter, ok := framework.Get(appKind); ok {
		flags := adapter.ComposeFeatures(app.WithDatabase)
		features.nacos = features.nacos || flags.Nacos
		features.polaris = features.polaris || flags.Polaris
		features.vegeta = features.vegeta || flags.Vegeta
	}
	return features
}
```

Note: the original code checked `app.Kind == manifest.KindHertz` (exact match, no empty-string fallback) — `composeApp.Kind` is always populated from `manifest.Service.Kind` or `WorkspaceService.Kind` by its callers (`WriteMonoCompose`, `loadWorkspaceComposeApps`), which are themselves normalized via `defaultKind`/manifest defaults upstream, so `app.Kind == ""` should not occur in practice; the `appKind == ""` fallback added here is defensive parity with `servicePort`'s existing `""` handling and does not change observed behavior for any existing test fixture.

- [ ] **Step 5: Build**

Run: `go build ./internal/scaffold/shared/...`
Expected: succeeds. `manifest` stays imported (used by `composeFeaturesForApp`, `servicePort`, and elsewhere in the file).

- [ ] **Step 6: Run the shared package test suite**

Run: `go test ./internal/scaffold/shared/... -count=1 -v`
Expected: all tests PASS.

- [ ] **Step 7: Run the full golden suite — must show zero diff**

Run: `go test ./internal/scaffold/mono/... -count=1 -v`
Expected: all six `TestGenerateGolden*` tests PASS (compose.yaml and conf/docker/conf.yaml are both part of the golden tree, so this is the definitive zero-diff check for this task).

- [ ] **Step 8: Commit**

```bash
git add internal/scaffold/shared/container.go
git commit -m "refactor(shared): route servicePort/renderServiceDockerConfig/composeFeaturesForApp through framework.Adapter"
```

---

## Task 8: Final validation, dead-code sweep, final commit

**Files:**
- Modify (dead-code removal only, no behavior change): `internal/scaffold/mono/mono.go` (`hzArgs`, `kitexArgs`, `kitexIDLBase` if fully unreferenced), `internal/scaffold/mono/files.go` (`hertzAPIProto` const, `writeHertzProtoSupportFiles`, `writeHertzAPIProto`), `internal/scaffold/infra/infra.go` (`mergeHertzConfig`, `wrapHertzConfigSnippet`, `hertzConfigMarkers`, `replaceMarkedHertzConfigBlock`, `hasTopLevelConfigKey`, `planKitexRateLimitConfig`, `defaultRateLimitConfBlock`, `mergeKitexRateLimitConfig`), `internal/scaffold/shared/container.go` (`hertzContainerPort`, `kitexContainerPort` consts, `renderHertzDockerConfigBlocks`/`renderKitexDockerConfigBlocks` if not already deleted in Task 7)

**Interfaces:**
- Consumes: nothing new — this task only deletes now-unreferenced code, it adds no calls.
- Produces: nothing new.

- [ ] **Step 1: Find dead code left behind by Tasks 3-7**

Run, for each candidate symbol, `grep -rn '<symbol>' internal/` to confirm zero remaining references before deleting:

```bash
for sym in hzArgs kitexArgs kitexIDLBase hertzAPIProto writeHertzProtoSupportFiles writeHertzAPIProto \
           mergeHertzConfig wrapHertzConfigSnippet hertzConfigMarkers replaceMarkedHertzConfigBlock \
           hasTopLevelConfigKey planKitexRateLimitConfig defaultRateLimitConfBlock mergeKitexRateLimitConfig \
           hertzContainerPort kitexContainerPort renderHertzDockerConfigBlocks renderKitexDockerConfigBlocks; do
  echo "=== $sym ==="
  grep -rn "\b$sym\b" internal/scaffold/mono internal/scaffold/infra internal/scaffold/shared --include='*.go' | grep -v '_test.go'
done
```

Expected: each symbol shows exactly one match — its own `func`/`const` declaration line — confirming no remaining callers outside `internal/scaffold/framework` (where the moved copies live under different exact names in some cases, e.g. `mergeHertzConfig` also exists standalone inside `adapter_hertz.go`; that's fine, they're different packages).

- [ ] **Step 2: Delete confirmed-dead code**

For each symbol confirmed dead in Step 1, delete its full declaration (function body or const block) from the file it lives in. Do not delete anything still showing a caller — re-check the design doc's Documented Scope Decisions section above if a symbol you expected to be dead (e.g. `writeHertzTemplate`, `writeKitexTemplate`, `writeTemplate`) still has callers; those are intentionally NOT migrated (Decision #4) and must stay.

- [ ] **Step 3: gofmt and vet**

Run: `gofmt -l $(find . -name '*.go' -not -path './.git/*')`
Expected: no output (all files clean).

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 4: Full test suite**

Run: `go test ./... -count=1`
Expected: all packages PASS, including `internal/scaffold/framework`, `internal/scaffold/mono`, `internal/scaffold/infra`, `internal/scaffold/shared`, and every other package in the module (CLI, MCP, doctor, etc. — none of these should be affected by this refactor, but the full suite catches any missed transitive breakage).

- [ ] **Step 5: Full build**

Run: `go build ./... && go build .`
Expected: both succeed with no output.

- [ ] **Step 6: Smoke test**

Run: `./scripts/smoke.sh`
Expected: exits 0. This exercises `ncgo new` end-to-end for both Hertz and Kitex through the CLI binary, the final confirmation that the refactor produces byte-identical scaffolds via the actual CLI entrypoint, not just via the Go test harness.

- [ ] **Step 7: Full golden re-check (belt and suspenders)**

Run: `go test ./internal/scaffold/mono/... -count=1 -v`
Expected: all `TestGenerateGolden*` tests PASS — final confirmation no golden diff exists anywhere after the dead-code sweep.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "refactor(scaffold): remove dead Kind-branch code superseded by framework.Adapter"
```

---

## Self-Review Notes (completed during plan authoring)

**Spec coverage:**
- (a) 资产/模板路径选择: `writeTemplate`'s dispatch (unmigrated, Decision #4), `overlayTemplatePackage` (unmigrated — Hertz-only branch is about template-package overlay semantics tied to `scaffoldtemplate`, not IDL/asset path selection; out of the spec's own migration table), `idlNameToken` (Task 4), `writeIDLPlaceholder`'s Hertz-only branch (Task 4), `writeHertzProtoSupportFiles` (Task 2/4), `renderIDLPlaceholder` (Task 4), `planHertzConfigWrite` (Task 5), `frameworkAssetFiles` (Task 5) — all covered or explicitly scoped out with rationale.
- (b) config 合并: `planHertzConfigWrite` (Task 5), `nextSteps`'s Hertz+HertzConfigKey step (unmigrated — this reads `p.HertzConfigKey()` from the `infra.Plugin`, a per-add-on value, not a per-framework one; not in the spec's migration table), `planKitexRateLimitConfigWrite` (Task 5) — covered.
- (c) wire/DI: `wire`'s dispatch + duplicate Kind consts (Task 6) — covered per Decision #2's documented narrower scope.
- (d) docker/dockerfile: `loadWorkspaceComposeApps` host-port allocation (explicitly out of scope, Decision #5), `renderServiceDockerConfig` (Task 7), `composeFeaturesForApp` (Task 7), `servicePort` (Task 7) — covered or explicitly scoped out.
- (f) IDL/生成器: `defaultIDL` (Task 3), `runGenerator` (Task 3), `idlNameToken`/`renderIDLPlaceholder`/`generatorCommand`/`requiresSQLCBeforeTidy` (Task 4), `Generate`'s lines 177/190/197 (explicitly out of scope, Decision #3) — covered or explicitly scoped out.
- "不在本次收敛范围内": `manifest.go:138` untouched (no task touches it), `bff/bff.go`/`rpc/rpc.go` untouched (no task touches them) — confirmed.

**Placeholder scan:** No "TBD"/"similar to Task N without code" patterns found — every task has concrete before/after code. The one place that says "leave for Task 8" is not a placeholder; it's an explicit, tracked deferral to the dead-code-sweep task with the exact symbol names listed in Task 8 Step 1's `grep` loop.

**Type consistency:** `framework.GeneratorOptions{Name, Module string}` is used identically across Task 1 (definition), Task 2 (both adapters), Task 3 (`mono.go`), and Task 4 (`files.go`) — checked. `framework.ConfigWrite{Body []byte, Action string}` used identically in Task 1, Task 2, Task 5. `framework.ComposeFeatureFlags{Nacos, Polaris, Vegeta bool}` used identically in Task 1, Task 2, Task 7. `framework.Register`/`Get`/`MustGet` signatures match between Task 1's definition and every call site in Tasks 3-7. Adapter method names (`IDLPath`, `IDLNameToken`, `GeneratorCommand`, `RenderIDLPlaceholder`, `WriteIDLSupportFiles`, `RunGenerator`, `RequiresSQLCBeforeTidy`, `OptionalAssetPath`, `HertzConfigAssetPath`, `MergeHertzConfig`, `MergeRateLimitConfig`, `DockerConfigBlocks`, `ContainerPort`, `ComposeFeatures`, `ServerFilePath`, `Kind`) are used with the exact same names and argument order in Task 1 (interface), Task 2 (implementations), and Tasks 3-7 (call sites) — checked by cross-reference while writing each task.
