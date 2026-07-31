# Polaris Canary Adapter GA Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the opt-in `ncgo add infra polaris_adapter` (Polaris-first, kitex-only) for production: client cache+TTL, explicit discovery-error fail semantics, decision-level observability, dry-run/shadow, and a runtime canary harness — without changing the opt-in / SDK-neutral-core / kitex-only boundaries.

**Architecture:** GA concerns are SDK-neutral *decorators* in a new embedded asset `optional/release_ops.go` (`package release`, stdlib only), wrapping the existing `Discoverer`/`RuleProvider`/`Selector` seams. The canonical `optional/release_canary.go` stays byte-identical. An opt-in OTel `Observer` lives in the kitex adapter surface. The Kitex canary LB is extended to source the **full** instance set from the adapter `Discoverer` (GetAllInstances-backed), decoupled from the kitex resolver's routing-filtered result.

**Tech Stack:** Go 1.25+ (stdlib only in the SDK-neutral core: `log/slog`, `sync`, `time`, `math/rand`); OTel metric API (`go.opentelemetry.io/otel`) in the opt-in observer; polaris-go v1.7.1 (adapter, unchanged); golden tests + a compile-and-test verification module.

## Global Constraints

(Copied verbatim from the design doc §9 / Issue #34 constraints — every task inherits these.)

- ncgo 本体不引入 polaris-go；`release_ops.go` 仅 stdlib；OTel 仅在 opt-in adapter。
- SDK-neutral 核心 `internal/assets/_data/optional/release_canary.go` 保持 canonical、**字节级不变**（PR#33 不变式）。验证命令：`git diff <base>..HEAD -- internal/assets/_data/optional/release_canary.go` 必须为空。
- opt-in、kitex-only（`polaris_adapter` 与 OTel observer 仅 kitex）。
- 凭证仅环境变量（`POLARIS_TOKEN`/`POLARIS_NAMESPACE`）；错误/日志绝不泄露 token（observer 日志禁止输出凭证）。
- CI 仍编译级 + 单测；运行时 harness 用假实例，不依赖活 Polaris。
- 默认 fail 语义为 **FailOpen**（可配置 FailFast）。
- 契约敏感面（CLI/MCP/scaffold 模板/golden/文档）改动须同步更新测试与 EN+ZH 文档。

## Design Refinement Notice (vs design §7)

Design §7 proposed the runtime harness at `internal/scaffold/test/canary/`. That location **cannot compile** because the `release` package only exists under `_data/` (not built by `go build ./...`) and in the verify module. **Refinement:** the harness + all SDK-neutral ops unit tests live as **committed Go tests in the verify module** `tools/verifyexamples/polaris-adapter/release/`, and `scripts/verify-polaris-adapter.sh` is extended from compile-only to **compile + `go test`**. The verify module `.gitignore` is narrowed to ignore only the copied asset files (not `release/`), so test files are tracked. Task 10 updates design doc §7 to match.

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `internal/assets/_data/optional/release_ops.go` | Create | SDK-neutral decorators: `FailPolicy`, `CacheOptions`, `CachingDiscoverer`, `CachingRuleProvider`, `Observer`/`NopObserver`/`SlogObserver`, `Engine` (+dry-run). stdlib only. |
| `internal/assets/_data/kitex/optional/release_canary.go` | Modify | `KitexCanaryLoadBalancer` += optional `Discoverer` + `Observer`; full-pool selection when `Discoverer != nil`; backward compatible. |
| `internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go` | Create | OTel-backed `Observer` (opt-in), reuses global meter from go-framework OTLP. |
| `internal/scaffold/infra/infra.go` | Modify | Emit `release_ops.go` with `release_canary`; emit OTel observer with `polaris_adapter`; `goGetDeps[polaris_adapter]` += OTel. |
| `scripts/verify-polaris-adapter.sh` | Modify | Copy `release_ops.go` + observer; run `go build ./... && go test ./...`. |
| `tools/verifyexamples/polaris-adapter/.gitignore` | Modify | Ignore only copied asset `.go` files, not `release/`. |
| `tools/verifyexamples/polaris-adapter/release/ops_test.go` | Create | TDD unit tests: cache/SWR/jitter/single-flight, fail policy, observers, engine, dry-run. |
| `tools/verifyexamples/polaris-adapter/release/harness_test.go` | Create | AC5 runtime harness: split ratio, fail paths, dry-run, cache (fake instances). |
| `internal/scaffold/infra/testdata/infra-canary/ops.go` | Create (golden) | Golden for new `release_ops.go` emitted with canary. |
| `internal/scaffold/infra/testdata/infra-polaris-adapter/polaris_observer_otel.go` | Create (golden) | Golden for OTel observer emitted with polaris_adapter. |
| `internal/assets/assets_test.go` (or new `release_ops_asset_test.go`) | Modify/Create | Asset parse sanity for `release_ops.go` + observer (no creds, parses, symbols). |
| `internal/scaffold/infra/infra_test.go` | Modify | Assert canary emits `ops.go`; polaris_adapter emits observer. |
| `internal/scaffold/infra/golden_test.go` | (unchanged logic) | Re-record goldens via `-update-golden`. |
| `internal/assets/_data/docs/kitex/design-doc.en.md` / `.zh-CN.md` | Modify | Canary GA section + troubleshooting (cache/TTL, fail policy, dry-run, resolver-visibility). |
| `docs/superpowers/specs/2026-07-31-polaris-adapter-ga-design.md` | Modify | §7 refinement note (harness location). |

### Verified emission facts (from `internal/scaffold/infra/infra.go`)

- `KindReleaseCanary` emits `optional/release_canary.go` → `internal/base/release/canary.go` **and** `{kitex,hertz}/optional/release_canary.go` → `internal/base/release/{kitex,hertz}.go` (via `frameworkAdapterName`).
- `KindPolarisAdapter` emits only `kitex/optional/polaris_canary_adapter.go` → `internal/base/release/polaris_adapter.go`; it assumes canary was added first (same `release` package).
- `//go:embed all:_data` auto-embeds new files — no `assets.go` change needed.
- Golden tests iterate `res.WrittenPaths` and write `golden.File(t, filepath.Join("<dir>", filepath.Base(p)), ...)`.

---

## Task 0: Formal resolver-visibility verification note

**Files:**
- Modify: `docs/superpowers/specs/2026-07-31-polaris-adapter-ga-design.md` (§3 — append verification evidence)

This closes the Issue's 「前置/关联验证」 checkbox with reproducible evidence. The conclusion (resolver uses `GetInstances` = routing-filtered) was established in brainstorming; this task pins the exact commands so Phase 4 review can re-run them.

- [ ] **Step 1: Re-run the source verification commands and record output**

Run:
```bash
MOD="$(find "$(go env GOMODCACHE)" -maxdepth 3 -type d -name 'polaris-go@*' | head -1)"
grep -n -A1 "GetInstances(req \*GetInstancesRequest)\|GetAllInstances(req \*GetAllInstancesRequest)" "$MOD/api/consumer.go"
grep -n "GetInstances(getInstances)" "$(find "$(go env GOMODCACHE)" -type d -path '*kitex-contrib/polaris*')/resolver.go"
```
Expected: `GetInstances` doc = 「获取可用的服务列表（会执行路由链…）」; `GetAllInstances` doc = 「获取完整的服务列表…」; resolver `Resolve()` calls `GetInstances`.

- [ ] **Step 2: Append the evidence block to design §3**

Add to design doc §3 a fenced block with the two grep outputs and the date, confirming: "canary LB 必须基于 adapter 全量 `Discoverer`（GetAllInstances），与 resolver 过滤结果解耦（见 §6）".

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-07-31-polaris-adapter-ga-design.md
git commit -m "docs(spec): pin resolver-visibility verification evidence (#34)"
```

---

## Task 1: SDK-neutral ops — cache + fail policy (`release_ops.go`)

**Files:**
- Create: `internal/assets/_data/optional/release_ops.go`
- Create: `tools/verifyexamples/polaris-adapter/release/ops_test.go`
- Modify: `tools/verifyexamples/polaris-adapter/.gitignore`
- Modify: `scripts/verify-polaris-adapter.sh` (copy ops asset; enable `go test`)

**Interfaces:**
- Consumes (from `release_canary.go`, same package): `Discoverer`, `RuleProvider`, `Instance`, `RuleSet`.
- Produces (used by Tasks 2–8): `FailPolicy` (`FailOpen`/`FailFast`), `CacheOptions{TTL,StaleTTL,Jitter,Now,Rand}`, `Observer` interface, `NopObserver`, `NewSlogObserver(*slog.Logger)`, `NewCachingDiscoverer(Discoverer, CacheOptions, FailPolicy, Observer)`, `NewCachingRuleProvider(RuleProvider, CacheOptions, FailPolicy, Observer)`, `Engine` + `NewEngine(...)`, `Decision`/`Pools` (existing).

### Background: TDD command pattern

The `release` package compiles only in the verify module. After editing the asset, sync + test with:
```bash
cp internal/assets/_data/optional/release_canary.go internal/assets/_data/optional/release_ops.go tools/verifyexamples/polaris-adapter/release/ && \
  cd tools/verifyexamples/polaris-adapter && go test ./release/ -count=1
```
(First run needs `go mod tidy` once; ops is stdlib-only so subsequent runs are offline.)

- [ ] **Step 1: Narrow the verify module `.gitignore`**

Replace the file content with:
```
# Asset .go files are copied in by scripts/verify-polaris-adapter.sh from
# internal/assets/_data. Test files (*_test.go) ARE committed.
release/release_canary.go
release/polaris_canary_adapter.go
release/release_ops.go
release/polaris_canary_observer_otel.go
```

- [ ] **Step 2: Write the failing tests** (`tools/verifyexamples/polaris-adapter/release/ops_test.go`)

```go
package release

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakeDiscoverer counts calls and returns canned instances or an error.
type fakeDiscoverer struct {
	calls int32
	ins   []Instance
	err   error
}

func (f *fakeDiscoverer) Discover(ctx context.Context, svc string) ([]Instance, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return nil, f.err
	}
	return f.ins, nil
}

func (f *fakeDiscoverer) n() int { return int(atomic.LoadInt32(&f.calls)) }

func staticIns(tracks ...string) []Instance {
	var out []Instance
	for i, t := range tracks {
		out = append(out, Instance{
			ID: string(rune('a' + i)), Address: "h" + string(rune('a'+i)),
			Weight: 1, Healthy: true, Enabled: true,
			Metadata: map[string]string{MetadataReleaseTrack: t},
		})
	}
	return out
}

func clock(start time.Time) (func() time.Time, func(d time.Duration)) {
	now := start
	return func() time.Time { return now }, func(d time.Duration) { now = now.Add(d) }
}

func TestCachingDiscoverer_FreshServesFromCache(t *testing.T) {
	now, _ := clock(time.Unix(1000, 0))
	up := &fakeDiscoverer{ins: staticIns("stable", "canary")}
	c := NewCachingDiscoverer(up, CacheOptions{TTL: 30 * time.Second, Now: now}, FailOpen, NopObserver{})
	for i := 0; i < 5; i++ {
		got, err := c.Discover(context.Background(), "svc")
		if err != nil || len(got) != 2 {
			t.Fatalf("iter %d: got %d ins, err %v", i, len(got), err)
		}
	}
	if up.n() != 1 {
		t.Fatalf("upstream calls = %d, want 1 (fresh cache)", up.n())
	}
}

func TestCachingDiscoverer_RefetchAfterTTL(t *testing.T) {
	now, adv := clock(time.Unix(1000, 0))
	up := &fakeDiscoverer{ins: staticIns("stable")}
	c := NewCachingDiscoverer(up, CacheOptions{TTL: 30 * time.Second, Jitter: 0, Now: now}, FailOpen, NopObserver{})
	_, _ = c.Discover(context.Background(), "svc")
	adv(31 * time.Second)
	_, _ = c.Discover(context.Background(), "svc")
	if up.n() != 2 {
		t.Fatalf("upstream calls = %d, want 2 (refetch after TTL)", up.n())
	}
}

func TestCachingDiscoverer_StaleWhileRevalidateOnError(t *testing.T) {
	now, adv := clock(time.Unix(1000, 0))
	up := &fakeDiscoverer{ins: staticIns("stable")}
	c := NewCachingDiscoverer(up, CacheOptions{TTL: 30 * time.Second, StaleTTL: 5 * time.Minute, Jitter: 0, Now: now}, FailOpen, NopObserver{})
	if _, err := c.Discover(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	up.err = errors.New("registry down") // subsequent fetches fail
	adv(31 * time.Second)                // past TTL, within StaleTTL
	got, err := c.Discover(context.Background(), "svc")
	if err != nil {
		t.Fatalf("FailOpen within stale window must not error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected stale value (1 ins), got %d", len(got))
	}
}

func TestCachingDiscoverer_FailFastPropagatesError(t *testing.T) {
	up := &fakeDiscoverer{err: errors.New("registry down")}
	c := NewCachingDiscoverer(up, CacheOptions{Now: time.Now}, FailFast, NopObserver{})
	if _, err := c.Discover(context.Background(), "svc"); err == nil {
		t.Fatal("FailFast with no cache must propagate error")
	}
}

func TestCachingDiscoverer_FailOpenEmptyWhenNoCache(t *testing.T) {
	up := &fakeDiscoverer{err: errors.New("registry down")}
	c := NewCachingDiscoverer(up, CacheOptions{Now: time.Now}, FailOpen, NopObserver{})
	got, err := c.Discover(context.Background(), "svc")
	if err != nil {
		t.Fatalf("FailOpen must not error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("FailOpen with no cache must return empty pool, got %d", len(got))
	}
}

func TestCachingDiscoverer_JitterVariesTTL(t *testing.T) {
	// With Jitter=0.5 and a fixed Rand returning 1.0, effective TTL = TTL*(1+0.5).
	now, adv := clock(time.Unix(1000, 0))
	up := &fakeDiscoverer{ins: staticIns("stable")}
	opts := CacheOptions{TTL: 10 * time.Second, Jitter: 0.5, Now: now, Rand: func() float64 { return 1.0 }}
	c := NewCachingDiscoverer(up, opts, FailOpen, NopObserver{})
	_, _ = c.Discover(context.Background(), "svc")
	adv(11 * time.Second) // past base TTL (10s) but within jittered (15s)
	_, _ = c.Discover(context.Background(), "svc")
	if up.n() != 1 {
		t.Fatalf("upstream calls = %d, want 1 (still fresh under jittered TTL)", up.n())
	}
}
```

Also add rule-provider tests (mirror the discoverer tests) using a `fakeRuleProvider` returning `RuleSet{Version:1, Enabled:true}`:
```go
type fakeRuleProvider struct {
	calls int32
	rs    RuleSet
	err   error
}

func (f *fakeRuleProvider) Rules(ctx context.Context, svc string) (RuleSet, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return RuleSet{}, f.err
	}
	return f.rs, nil
}

func TestCachingRuleProvider_FreshAndFailOpen(t *testing.T) {
	now, adv := clock(time.Unix(1000, 0))
	up := &fakeRuleProvider{rs: RuleSet{Version: 1, Enabled: true, Service: "svc"}}
	c := NewCachingRuleProvider(up, CacheOptions{TTL: 30 * time.Second, StaleTTL: time.Minute, Jitter: 0, Now: now}, FailOpen, NopObserver{})
	if _, err := c.Rules(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	up.err = errors.New("config down")
	adv(31 * time.Second)
	rs, err := c.Rules(context.Background(), "svc")
	if err != nil || rs.Version != 1 {
		t.Fatalf("FailOpen stale rules: version=%d err=%v", rs.Version, err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run (sync + test pattern above). Expected: FAIL — `undefined: NewCachingDiscoverer`, `undefined: CacheOptions`, etc.

- [ ] **Step 4: Write the implementation** (`internal/assets/_data/optional/release_ops.go`)

```go
// Optional SDK-neutral operations add-on for the release canary seams.
//
// Same package as release_canary.go. This file is intentionally dependency-free
// (standard library only): it provides production-hardening decorators — a TTL
// cache with stale-while-revalidate and refresh jitter, explicit discovery/rule
// failure semantics (fail-open / fail-fast), decision-level observation hooks,
// and a dry-run/shadow engine. Wire concrete SDK clients elsewhere (adapters).

package release

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// FailPolicy controls behavior when discovery or rule loading fails.
type FailPolicy int

const (
	// FailOpen serves the last-known-good value (or an empty pool when nothing
	// is cached) so callers stay available. This is the default.
	FailOpen FailPolicy = iota
	// FailFast propagates the error, refusing to route on stale/unknown state.
	FailFast
)

func (p FailPolicy) String() string {
	if p == FailFast {
		return "fail_fast"
	}
	return "fail_open"
}

// CacheOptions configures the TTL cache decorators. Zero values fall back to
// production defaults via withDefaults.
type CacheOptions struct {
	TTL      time.Duration // fresh window; default 30s
	StaleTTL time.Duration // serve-stale window after TTL; default 5m
	Jitter   float64       // ±fraction applied to TTL to desynchronize refreshes; default 0.2
	Now      func() time.Time
	Rand     func() float64 // injectable for deterministic tests; default rand.Float64
}

const (
	defaultCacheTTL      = 30 * time.Second
	defaultCacheStaleTTL = 5 * time.Minute
	defaultCacheJitter   = 0.2
)

func (o CacheOptions) withDefaults() CacheOptions {
	if o.TTL <= 0 {
		o.TTL = defaultCacheTTL
	}
	if o.StaleTTL <= 0 {
		o.StaleTTL = defaultCacheStaleTTL
	}
	if o.Jitter == 0 {
		o.Jitter = defaultCacheJitter
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Rand == nil {
		o.Rand = rand.Float64
	}
	return o
}

// jitteredTTL returns TTL scaled by a random factor in [1-Jitter, 1+Jitter].
func (o CacheOptions) jitteredTTL() time.Duration {
	if o.Jitter <= 0 {
		return o.TTL
	}
	factor := 1 + (o.Rand()*2-1)*o.Jitter
	return time.Duration(float64(o.TTL) * factor)
}

// cacheEntry is a single cached value with its fetch time and fetch error.
type cacheEntry[T any] struct {
	value   T
	fetched time.Time
	ttl     time.Duration
	err     error
}

func (e cacheEntry[T]) fresh(now time.Time) bool { return now.Sub(e.fetched) < e.ttl }
func (e cacheEntry[T]) stale(now time.Time, staleTTL time.Duration) bool {
	return now.Sub(e.fetched) < e.ttl+staleTTL
}

// ttlCache is a small per-key TTL cache with single-flight refresh.
type ttlCache[T any] struct {
	opts CacheOptions
	mu   sync.Mutex
	ents map[string]cacheEntry[T]
	calls map[string]*sync.WaitGroup
}

func newTTLCache[T any](opts CacheOptions) *ttlCache[T] {
	opts = opts.withDefaults()
	return &ttlCache[T]{opts: opts, ents: map[string]cacheEntry[T]{}, calls: map[string]*sync.WaitGroup{}}
}

// get returns the cached entry (if any) and whether a refresh is needed.
func (c *ttlCache[T]) get(key string) (cacheEntry[T], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.ents[key]
	now := c.opts.Now()
	if !ok || !e.fresh(now) {
		return e, true
	}
	return e, false
}

// joinOrStart ensures only one goroutine refreshes a key at a time. It returns
// true when the caller owns the refresh (and must call done), false when another
// goroutine already owns it (caller should wait then re-read).
func (c *ttlCache[T]) joinOrStart(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wg, ok := c.calls[key]; ok {
		c.mu.Unlock()
		wg.Wait()
		c.mu.Lock()
		return false
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c.calls[key] = wg
	return true
}

func (c *ttlCache[T]) done(key string) {
	c.mu.Lock()
	wg := c.calls[key]
	delete(c.calls, key)
	c.mu.Unlock()
	if wg != nil {
		wg.Done()
	}
}

func (c *ttlCache[T]) put(key string, value T, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ents[key] = cacheEntry[T]{value: value, fetched: c.opts.Now(), ttl: c.opts.jitteredTTL(), err: err}
}

// CachingDiscoverer wraps a Discoverer with a TTL cache + stale-while-revalidate
// + single-flight + FailPolicy. It implements Discoverer.
type CachingDiscoverer struct {
	Upstream Discoverer
	Policy   FailPolicy
	Observer Observer
	cache    *ttlCache[[]Instance]
}

func NewCachingDiscoverer(upstream Discoverer, opts CacheOptions, policy FailPolicy, obs Observer) *CachingDiscoverer {
	if obs == nil {
		obs = NopObserver{}
	}
	return &CachingDiscoverer{Upstream: upstream, Policy: policy, Observer: obs, cache: newTTLCache[[]Instance](opts)}
}

func (c *CachingDiscoverer) Discover(ctx context.Context, serviceName string) ([]Instance, error) {
	entry, needRefresh := c.cache.get(serviceName)
	now := c.cache.opts.Now()
	if !needRefresh {
		return entry.value, nil
	}
	// Within the stale window with a good cached value: serve stale, refresh async.
	if entry.err == nil && len(entry.value) >= 0 && c.cache.opts.Now().Sub(entry.fetched) < entry.ttl+c.cache.opts.StaleTTL && entry.fetched != (time.Time{}) {
		if c.cache.joinOrStart(serviceName) {
			go c.refresh(context.Background(), serviceName)
		}
		if entry.err == nil {
			return entry.value, nil
		}
	}
	// No usable cache: blocking refresh (single-flight).
	if c.cache.joinOrStart(serviceName) {
		c.refresh(ctx, serviceName)
	} else {
		entry, _ = c.cache.get(serviceName)
		return c.applyPolicy(serviceName, entry)
	}
	entry, _ = c.cache.get(serviceName)
	return c.applyPolicy(serviceName, entry)
}

func (c *CachingDiscoverer) refresh(ctx context.Context, serviceName string) {
	defer c.cache.done(serviceName)
	value, err := c.Upstream.Discover(ctx, serviceName)
	c.Observer.ObserveDiscovery(serviceName, len(value), err)
	if err != nil {
		// Keep the previous good value in the cache; record the error on a
		// side entry only if nothing good exists.
		c.cache.mu.Lock()
		prev, ok := c.cache.ents[serviceName]
		c.cache.mu.Unlock()
		if ok && prev.err == nil {
			c.cache.mu.Lock()
			c.cache.ents[serviceName] = cacheEntry[[]Instance]{value: prev.value, fetched: prev.fetched, ttl: prev.ttl, err: err}
			c.cache.mu.Unlock()
			return
		}
		c.cache.put(serviceName, nil, err)
		return
	}
	c.cache.put(serviceName, value, nil)
}

func (c *CachingDiscoverer) applyPolicy(serviceName string, entry cacheEntry[[]Instance]) ([]Instance, error) {
	if entry.err == nil {
		return entry.value, nil
	}
	if c.Policy == FailFast {
		return nil, entry.err
	}
	if entry.value != nil {
		return entry.value, nil
	}
	return []Instance{}, nil
}

// CachingRuleProvider wraps a RuleProvider with the same caching + fail policy.
type CachingRuleProvider struct {
	Upstream RuleProvider
	Policy   FailPolicy
	Observer Observer
	cache    *ttlCache[RuleSet]
}

func NewCachingRuleProvider(upstream RuleProvider, opts CacheOptions, policy FailPolicy, obs Observer) *CachingRuleProvider {
	if obs == nil {
		obs = NopObserver{}
	}
	return &CachingRuleProvider{Upstream: upstream, Policy: policy, Observer: obs, cache: newTTLCache[RuleSet](opts)}
}

func (c *CachingRuleProvider) Rules(ctx context.Context, serviceName string) (RuleSet, error) {
	entry, needRefresh := c.cache.get(serviceName)
	if !needRefresh {
		return entry.value, nil
	}
	if entry.err == nil && c.cache.opts.Now().Sub(entry.fetched) < entry.ttl+c.cache.opts.StaleTTL && entry.fetched != (time.Time{}) {
		if c.cache.joinOrStart(serviceName) {
			go c.refresh(context.Background(), serviceName)
		}
		return entry.value, nil
	}
	if c.cache.joinOrStart(serviceName) {
		c.refresh(ctx, serviceName)
	}
	entry, _ = c.cache.get(serviceName)
	return c.applyPolicy(entry)
}

func (c *CachingRuleProvider) refresh(ctx context.Context, serviceName string) {
	defer c.cache.done(serviceName)
	value, err := c.Upstream.Rules(ctx, serviceName)
	c.Observer.ObserveRules(serviceName, value.Version, err)
	if err != nil {
		c.cache.mu.Lock()
		prev, ok := c.cache.ents[serviceName]
		c.cache.mu.Unlock()
		if ok && prev.err == nil {
			c.cache.mu.Lock()
			c.cache.ents[serviceName] = cacheEntry[RuleSet]{value: prev.value, fetched: prev.fetched, ttl: prev.ttl, err: err}
			c.cache.mu.Unlock()
			return
		}
		c.cache.put(serviceName, RuleSet{}, err)
		return
	}
	c.cache.put(serviceName, value, nil)
}

func (c *CachingRuleProvider) applyPolicy(entry cacheEntry[RuleSet]) (RuleSet, error) {
	if entry.err == nil {
		return entry.value, nil
	}
	if c.Policy == FailFast {
		return RuleSet{}, entry.err
	}
	return entry.value, nil
}
```

> Note: `Observer`/`NopObserver` are defined in Task 2. To keep this task independently green, add a temporary minimal `Observer`/`NopObserver` here, then Task 2 replaces/expands them. **Implementer: if Task 2 is sequenced immediately, define the full Observer in Task 2 and run Task 1+2 tests together.** The single-flight `joinOrStart` releases/re-acquires `c.mu` around `wg.Wait()` to avoid deadlock; reviewers should confirm the lock dance.

- [ ] **Step 5: Run tests to verify they pass**

Run (sync + test). Expected: PASS for all cache/fail tests (after Observer exists — see note).

- [ ] **Step 6: Commit**

```bash
git add internal/assets/_data/optional/release_ops.go \
  tools/verifyexamples/polaris-adapter/release/ops_test.go \
  tools/verifyexamples/polaris-adapter/.gitignore
git commit -m "feat(release): add SDK-neutral TTL cache + fail-policy decorators (#34)"
```

---

## Task 2: Observer interface + Nop + Slog observers

**Files:**
- Modify: `internal/assets/_data/optional/release_ops.go` (append Observer section)
- Modify: `tools/verifyexamples/polaris-adapter/release/ops_test.go` (observer tests)

**Interfaces:**
- Produces: `Observer` interface, `NopObserver`, `NewSlogObserver(*slog.Logger) *SlogObserver`.

- [ ] **Step 1: Write the failing tests** (append to `ops_test.go`)

```go
func TestSlogObserver_DecisionDoesNotLeakCredentials(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	obs := NewSlogObserver(logger)
	pools := Pools{Stable: staticIns("stable"), Canary: staticIns("canary")}
	obs.ObserveDecision("svc", Decision{Track: TrackCanary, Reason: "weighted", Rule: "r1"}, pools)
	obs.ObserveDiscovery("svc", 2, nil)
	obs.ObserveFallback("svc", "empty_canary")
	out := buf.String()
	for _, want := range []string{"canary_decision", "canary", "weighted"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q: %s", want, out)
		}
	}
	for _, bad := range []string{"POLARIS_TOKEN", "token"} {
		if strings.Contains(strings.ToLower(out), strings.ToLower(bad)) {
			t.Errorf("log may leak credentials (%q): %s", bad, out)
		}
	}
}

func TestNopObserver_Safe(t *testing.T) {
	var o Observer = NopObserver{}
	o.ObserveDecision("s", Decision{}, Pools{})
	o.ObserveDiscovery("s", 0, nil)
	o.ObserveRules("s", 0, nil)
	o.ObserveFallback("s", "x")
}
```
(Add imports `bytes`, `strings` to the test file.)

- [ ] **Step 2: Run tests to verify they fail** — Expected: `undefined: NewSlogObserver`.

- [ ] **Step 3: Implement** (append to `release_ops.go`)

```go
// Observer receives decision-level telemetry. Implementations must be safe for
// concurrent use and MUST NOT emit credentials or other sensitive values.
type Observer interface {
	ObserveDecision(service string, d Decision, pools Pools)
	ObserveFallback(service, reason string)
	ObserveDiscovery(service string, instances int, err error)
	ObserveRules(service string, version int, err error)
}

// NopObserver discards all telemetry. It is the default when none is supplied.
type NopObserver struct{}

func (NopObserver) ObserveDecision(string, Decision, Pools) {}
func (NopObserver) ObserveFallback(string, string)          {}
func (NopObserver) ObserveDiscovery(string, int, error)     {}
func (NopObserver) ObserveRules(string, int, error)         {}

// SlogObserver emits structured decision logs via log/slog (standard library).
type SlogObserver struct{ Logger *slog.Logger }

func NewSlogObserver(logger *slog.Logger) *SlogObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogObserver{Logger: logger}
}

func (o *SlogObserver) ObserveDecision(service string, d Decision, pools Pools) {
	o.Logger.Info("canary_decision",
		slog.String("service", service),
		slog.String("track", d.Track),
		slog.String("reason", d.Reason),
		slog.String("rule", d.Rule),
		slog.String("fallback", d.Fallback),
		slog.Int("pool_stable", len(pools.Stable)),
		slog.Int("pool_canary", len(pools.Canary)),
		slog.Int("pool_unknown", len(pools.Unknown)),
	)
}

func (o *SlogObserver) ObserveFallback(service, reason string) {
	o.Logger.Warn("canary_fallback", slog.String("service", service), slog.String("reason", reason))
}

func (o *SlogObserver) ObserveDiscovery(service string, instances int, err error) {
	if err != nil {
		o.Logger.Error("canary_discovery_error", slog.String("service", service), slog.String("error", err.Error()))
		return
	}
	o.Logger.Debug("canary_discovery", slog.String("service", service), slog.Int("instances", instances))
}

func (o *SlogObserver) ObserveRules(service string, version int, err error) {
	if err != nil {
		o.Logger.Error("canary_rule_error", slog.String("service", service), slog.String("error", err.Error()))
		return
	}
	o.Logger.Debug("canary_rules", slog.String("service", service), slog.Int("version", version))
}
```

- [ ] **Step 4: Run tests to verify they pass.**
- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/optional/release_ops.go tools/verifyexamples/polaris-adapter/release/ops_test.go
git commit -m "feat(release): add Observer interface with slog + nop implementations (#34)"
```

---

## Task 3: Engine + dry-run/shadow

**Files:**
- Modify: `internal/assets/_data/optional/release_ops.go` (append Engine)
- Modify: `tools/verifyexamples/polaris-adapter/release/ops_test.go` (engine tests)

**Interfaces:**
- Consumes: `CachingDiscoverer`/`Discoverer`, `CachingRuleProvider`/`RuleProvider`, `Observer`, `Decide`, `SplitInstances`, `SelectInstance`, `Traffic`, `Selection`.
- Produces: `Engine`, `EngineOptions`, `NewEngine`, `(*Engine).Select`.

- [ ] **Step 1: Write the failing tests** (append to `ops_test.go`)

```go
type recordingObserver struct {
	NopObserver
	decisions []Decision
	fallbacks []string
}

func (r *recordingObserver) ObserveDecision(_ string, d Decision, _ Pools) {
	r.decisions = append(r.decisions, d)
}
func (r *recordingObserver) ObserveFallback(_ string, reason string) {
	r.fallbacks = append(r.fallbacks, reason)
}

func weightedRules(canary int) RuleSet {
	return RuleSet{Version: 1, Enabled: true, Service: "svc", DefaultTrack: TrackStable, Fallback: FallbackStable,
		Rules: []Rule{{Name: "split", Priority: 10, Track: "", Weighted: &Weighted{Stable: 100 - canary, Canary: canary},
			Match: Match{}}}}
}

func TestEngine_SelectRoutesAndObserves(t *testing.T) {
	obs := &recordingObserver{}
	disc := StaticDiscoverer(staticIns("stable", "canary"))
	rules := StaticRuleProvider{RuleSet: weightedRules(50)}
	e := NewEngine(EngineOptions{ServiceName: "svc", Discoverer: disc, Rules: rules, Observer: obs})
	sel, err := e.Select(context.Background(), Traffic{UserID: "u1"})
	if err != nil || !sel.OK {
		t.Fatalf("select: ok=%v err=%v", sel.OK, err)
	}
	if len(obs.decisions) != 1 {
		t.Fatalf("expected 1 observed decision, got %d", len(obs.decisions))
	}
}

func TestEngine_DryRunNeverRoutesToCanary(t *testing.T) {
	disc := StaticDiscoverer(staticIns("stable", "canary"))
	rules := StaticRuleProvider{RuleSet: weightedRules(100)} // 100% canary intent
	e := NewEngine(EngineOptions{ServiceName: "svc", Discoverer: disc, Rules: rules, DryRun: true})
	for i := 0; i < 20; i++ {
		sel, err := e.Select(context.Background(), Traffic{UserID: "u" + string(rune('a'+i%5))})
		if err != nil {
			t.Fatal(err)
		}
		if sel.Instance.Track() == TrackCanary {
			t.Fatalf("dry-run must not route to canary, got track=%q", sel.Instance.Track())
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail** — Expected: `undefined: NewEngine`.

- [ ] **Step 3: Implement** (append to `release_ops.go`)

```go
// EngineOptions configures an Engine.
type EngineOptions struct {
	ServiceName string
	Discoverer  Discoverer   // typically a *CachingDiscoverer
	Rules       RuleProvider // typically a *CachingRuleProvider
	Observer    Observer     // optional; NopObserver when nil
	DryRun      bool         // shadow mode: record intent, route to stable/default
}

// Engine composes discovery + rules + observation + dry-run over the canonical
// Decide/SplitInstances/SelectInstance primitives (which remain untouched).
type Engine struct {
	ServiceName string
	Discoverer  Discoverer
	Rules       RuleProvider
	Observer    Observer
	DryRun      bool
}

func NewEngine(opts EngineOptions) *Engine {
	if opts.Observer == nil {
		opts.Observer = NopObserver{}
	}
	return &Engine{ServiceName: opts.ServiceName, Discoverer: opts.Discoverer, Rules: opts.Rules, Observer: opts.Observer, DryRun: opts.DryRun}
}

// Select runs one canary decision. In DryRun mode it observes the real decision
// but returns the stable/default selection so canary traffic is not affected.
func (e *Engine) Select(ctx context.Context, traffic Traffic) (Selection, error) {
	if e.Discoverer == nil {
		return Selection{}, errors.New("release engine: Discoverer is nil")
	}
	instances, err := e.Discoverer.Discover(ctx, e.ServiceName)
	if err != nil {
		return Selection{}, err
	}
	rules := RuleSet{Enabled: false, Service: e.ServiceName, DefaultTrack: TrackStable, Fallback: FallbackStable}
	if e.Rules != nil {
		rules, err = e.Rules.Rules(ctx, e.ServiceName)
		if err != nil {
			return Selection{}, err
		}
	}
	pools := SplitInstances(instances)
	decision := Decide(traffic, rules)
	e.Observer.ObserveDecision(e.ServiceName, decision, pools)

	if e.DryRun {
		// Record intent only; route as if canary were never chosen.
		shadow := decision
		shadow.Track = normalizeTrackOrDefault(rules.DefaultTrack, TrackStable)
		shadow.Reason = "dry_run:" + decision.Reason
		sticky := firstNonEmpty(traffic.StickyKey, traffic.UserID, traffic.TenantID, traffic.Lane, rules.Service)
		ins, ok := SelectInstance(pools, shadow, sticky)
		return Selection{Instance: ins, Decision: shadow, Pools: pools, OK: ok}, nil
	}

	sticky := firstNonEmpty(traffic.StickyKey, traffic.UserID, traffic.TenantID, traffic.Lane, rules.Service)
	ins, ok := SelectInstance(pools, decision, sticky)
	if !ok {
		e.Observer.ObserveFallback(e.ServiceName, "no_instance_for_"+decision.Track)
	}
	return Selection{Instance: ins, Decision: decision, Pools: pools, OK: ok}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass.**
- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/optional/release_ops.go tools/verifyexamples/polaris-adapter/release/ops_test.go
git commit -m "feat(release): add Engine with dry-run/shadow canary mode (#34)"
```

---

## Task 4: Emit `release_ops.go` with the canary add-on

**Files:**
- Modify: `internal/scaffold/infra/infra.go` (`assetFiles`, `KindReleaseCanary` branch)
- Modify: `internal/scaffold/infra/infra_test.go` (assert ops.go emitted)
- Create (golden): `internal/scaffold/infra/testdata/infra-canary/ops.go`

**Interfaces:**
- Consumes: asset `optional/release_ops.go` (Tasks 1–3).
- Produces: `add infra canary` / `release_canary` now also writes `internal/base/release/ops.go`.

- [ ] **Step 1: Write the failing test** (add to `infra_test.go`, near `TestAddReleaseCanaryForHertz`)

```go
func TestAddReleaseCanaryEmitsOps(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindReleaseCanary})
	if err != nil {
		t.Fatalf("Add release_canary: %v", err)
	}
	opsPath := filepath.Join(root, "internal", "base", "release", "ops.go")
	if !slices.Contains(res.WrittenPaths, opsPath) {
		t.Fatalf("expected ops.go in WrittenPaths, got %v", res.WrittenPaths)
	}
	body, err := os.ReadFile(opsPath)
	if err != nil {
		t.Fatalf("read ops.go: %v", err)
	}
	for _, want := range []string{"package release", "func NewCachingDiscoverer(", "type Observer interface", "func NewEngine("} {
		if !strings.Contains(string(body), want) {
			t.Errorf("ops.go missing %q", want)
		}
	}
}
```
(Ensure `slices` is imported.)

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/scaffold/infra/ -run TestAddReleaseCanaryEmitsOps -count=1`. Expected: FAIL (ops.go not in WrittenPaths).

- [ ] **Step 3: Implement** — in `assetFiles`, extend the `KindReleaseCanary` branch (lines ~309-324). After building `files` for the seam + framework adapter, append the ops asset for BOTH service kinds:

```go
	if infraKind == KindObservabilityLog || infraKind == KindReleaseCanary {
		files := []addOnFile{{
			SourcePath:    "optional/" + infraKind + ".go",
			OutputRelPath: outputRelPaths[infraKind],
		}}
		adapterName := frameworkAdapterName(infraKind, serviceKind)
		switch serviceKind {
		case manifest.KindHertz:
			files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + infraKind + ".go", OutputRelPath: adapterName})
		case manifest.KindKitex:
			files = append(files, addOnFile{SourcePath: serviceKind + "/optional/" + infraKind + ".go", OutputRelPath: adapterName})
		default:
			return nil, fmt.Errorf("infra: unsupported service kind %q", serviceKind)
		}
		if infraKind == KindReleaseCanary {
			files = append(files, addOnFile{
				SourcePath:    "optional/release_ops.go",
				OutputRelPath: filepath.Join("internal", "base", "release", "ops.go"),
			})
		}
		return files, nil
	}
```

- [ ] **Step 4: Run test to verify it passes.**
- [ ] **Step 5: Re-record the canary golden** — `go test ./internal/scaffold/infra/ -run TestGenerateGoldenInfraCanary -update-golden -count=1`. Verify `testdata/infra-canary/ops.go` is created and byte-identical to the asset (`diff internal/assets/_data/optional/release_ops.go internal/scaffold/infra/testdata/infra-canary/ops.go` → identical, since no `{{.Module}}` placeholders).
- [ ] **Step 6: Commit**

```bash
git add internal/scaffold/infra/infra.go internal/scaffold/infra/infra_test.go internal/scaffold/infra/testdata/infra-canary/ops.go
git commit -m "feat(infra): emit release_ops.go with release_canary add-on (#34)"
```

---

## Task 5: Kitex canary LB — full-pool selection via Discoverer

**Files:**
- Modify: `internal/assets/_data/kitex/optional/release_canary.go` (`KitexCanaryLoadBalancer` + `kitexCanaryPicker`)
- Modify: `tools/verifyexamples/polaris-adapter/release/ops_test.go` OR new `lb_test.go` (kitex LB is kitex-coupled — see note)
- Modify (golden): `internal/scaffold/infra/testdata/infra-canary/kitex.go` (re-record)

> **Test-home note:** `kitex/optional/release_canary.go` imports kitex packages, so it does NOT compile in the stdlib/polaris verify module. Its logic is validated by (a) the golden byte-lock, (b) an asset parse test, and (c) the SDK compile gate is not applicable (kitex deps). Add a **parse + symbol asset test** in `internal/assets/` (Task 8 covers assets). Behavioral coverage for the full-pool path is exercised by the harness (Task 7) using the SDK-neutral `Engine`, which the LB now delegates to.

**Interfaces:**
- Consumes: `release.Discoverer`, `release.Observer`, `release.Engine`, `release.NewEngine`.
- Produces: `KitexCanaryLoadBalancer{ServiceName, RuleProvider, Fallback, Discoverer, Observer}`; when `Discoverer != nil`, picker selects over the full pool via `Engine` and maps the chosen `release.Instance` back to a kitex `discovery.Instance`.

- [ ] **Step 1: Write the asset parse/symbol test first** (add to `internal/assets/assets_test.go` or new `release_kitex_canary_asset_test.go`)

```go
func TestKitexCanaryLBAssetHasDiscovererSeam(t *testing.T) {
	b, err := fs.ReadFile(FS(), "kitex/optional/release_canary.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)
	if _, err := parser.ParseFile(token.NewFileSet(), "release_canary.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{"Discoverer Discoverer", "Observer Observer", "NewEngine(", "KitexResultDiscoverer"} {
		if !strings.Contains(src, want) {
			t.Errorf("kitex canary LB asset missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/assets/ -run TestKitexCanaryLBAssetHasDiscovererSeam -count=1`. Expected: FAIL (missing `Discoverer Discoverer`).

- [ ] **Step 3: Implement the LB change** — in `internal/assets/_data/kitex/optional/release_canary.go`:

Add fields to the struct (backward compatible — existing callers leave them nil):
```go
type KitexCanaryLoadBalancer struct {
	ServiceName  string
	RuleProvider RuleProvider
	Fallback     loadbalance.Loadbalancer
	// Discoverer, when non-nil, supplies the FULL stable+canary instance set
	// (e.g. a CachingDiscoverer wrapping the polaris adapter's GetAllInstances
	// Discoverer). This decouples canary splitting from the kitex resolver,
	// which may return a routing-filtered subset. When nil, the picker falls
	// back to the resolver-provided discovery.Result (legacy behavior).
	Discoverer Discoverer
	Observer   Observer
}
```

Update `GetPicker` to pass them through:
```go
func (lb KitexCanaryLoadBalancer) GetPicker(result discovery.Result) loadbalance.Picker {
	var fallback loadbalance.Picker
	if lb.Fallback != nil {
		fallback = lb.Fallback.GetPicker(result)
	}
	return kitexCanaryPicker{serviceName: lb.ServiceName, result: result, rules: lb.RuleProvider, fallback: fallback, discoverer: lb.Discoverer, observer: lb.Observer}
}
```

Extend `kitexCanaryPicker` + `Next`:
```go
type kitexCanaryPicker struct {
	serviceName string
	result      discovery.Result
	rules       RuleProvider
	fallback    loadbalance.Picker
	discoverer  Discoverer
	observer    Observer
}

func (p kitexCanaryPicker) Next(ctx context.Context, request interface{}) discovery.Instance {
	traffic := TrafficFromContext(ctx)
	if traffic.StickyKey == "" && traffic.Lane == "" && traffic.UserID == "" && traffic.TenantID == "" {
		traffic = TrafficFromKitex(ctx)
	}
	// Full-pool path: source instances from the adapter Discoverer (decoupled
	// from the resolver's possibly-filtered result).
	if p.discoverer != nil {
		return p.nextFromDiscoverer(ctx, request, traffic)
	}
	return p.nextFromResult(ctx, request, traffic)
}

func (p kitexCanaryPicker) nextFromDiscoverer(ctx context.Context, request interface{}, traffic Traffic) discovery.Instance {
	obs := p.observer
	if obs == nil {
		obs = NopObserver{}
	}
	engine := NewEngine(EngineOptions{ServiceName: p.serviceName, Discoverer: p.discoverer, Rules: p.rules, Observer: obs})
	selection, err := engine.Select(ctx, traffic)
	if err != nil || !selection.OK {
		return p.fallbackOrWeighted(ctx, request, traffic)
	}
	if ins := findKitexInstance(p.result.Instances, selection.Instance.ID); ins != nil {
		return ins
	}
	// Selected full-pool instance is absent from the resolver result (expected
	// when the resolver filters): synthesize a routable kitex instance.
	return kitexInstanceFromRelease(selection.Instance)
}

func (p kitexCanaryPicker) nextFromResult(ctx context.Context, request interface{}, traffic Traffic) discovery.Instance {
	if len(p.result.Instances) == 0 {
		return nil
	}
	selector := Selector{ServiceName: p.serviceName, Discoverer: KitexResultDiscoverer{Result: p.result}, RuleProvider: p.rules}
	selection, err := selector.Select(ctx, traffic)
	if err != nil {
		return p.fallbackOrWeighted(ctx, request, traffic)
	}
	if selection.OK {
		if ins := findKitexInstance(p.result.Instances, selection.Instance.ID); ins != nil {
			return ins
		}
	}
	if NormalizeFallback(selection.Decision.Fallback) == FallbackFailFast {
		return nil
	}
	return p.fallbackOrWeighted(ctx, request, traffic)
}
```

Add the synthesizer (uses kitex `discovery` instance type; implement a minimal `discovery.Instance`):
```go
// releaseInstance is a minimal discovery.Instance synthesized from a full-pool
// release.Instance when the resolver result does not contain it.
type releaseInstance struct {
	addr   net.Addr
	weight int
	tags   map[string]string
}

func (i releaseInstance) Address() net.Addr          { return i.addr }
func (i releaseInstance) Weight() int                 { return i.weight }
func (i releaseInstance) Tag(key string) (string, bool) {
	v, ok := i.tags[key]
	return v, ok
}

func kitexInstanceFromRelease(ins Instance) discovery.Instance {
	tags := map[string]string{}
	for k, v := range ins.Metadata {
		tags[k] = v
	}
	return releaseInstance{addr: parseReleaseAddr(ins.Address), weight: ins.Weight, tags: tags}
}
```

Add `net` import and a `parseReleaseAddr` helper:
```go
func parseReleaseAddr(s string) net.Addr {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return &net.TCPAddr{IP: net.ParseIP(s)}
	}
	p, _ := strconv.Atoi(port)
	return &net.TCPAddr{IP: net.ParseIP(host), Port: p}
}
```
(Add `strconv` import; `net` import.)

- [ ] **Step 4: Run the asset test to verify it passes** — `go test ./internal/assets/ -run TestKitexCanaryLBAssetHasDiscovererSeam -count=1`. Expected: PASS.
- [ ] **Step 5: Re-record kitex canary golden** — `go test ./internal/scaffold/infra/ -run TestGenerateGoldenInfraCanary -update-golden -count=1`; confirm `testdata/infra-canary/kitex.go` updated with the new fields/helpers.
- [ ] **Step 6: Commit**

```bash
git add internal/assets/_data/kitex/optional/release_canary.go internal/assets/ \
  internal/scaffold/infra/testdata/infra-canary/kitex.go
git commit -m "feat(release): kitex canary LB sources full pool via Discoverer (#34)"
```

---

## Task 6: OTel Observer asset + emit with polaris_adapter

**Files:**
- Create: `internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go`
- Modify: `internal/scaffold/infra/infra.go` (`assetFiles` polaris branch; `goGetDeps[KindPolarisAdapter]`)
- Modify: `internal/scaffold/infra/infra_test.go` (assert observer emitted)
- Create (golden): `internal/scaffold/infra/testdata/infra-polaris-adapter/polaris_observer_otel.go`

**Interfaces:**
- Consumes: `Observer` interface (Task 2); OTel metric API.
- Produces: `NewOTelObserver(meter metric.Meter) (Observer, error)`.

- [ ] **Step 1: Write the failing infra test**

```go
func TestAddInfraPolarisAdapterEmitsOTelObserver(t *testing.T) {
	root := seedKitexProject(t, nil)
	res, err := Add(Options{Root: root, Kind: KindPolarisAdapter})
	if err != nil {
		t.Fatalf("Add polaris_adapter: %v", err)
	}
	obsPath := filepath.Join(root, "internal", "base", "release", "polaris_observer_otel.go")
	if !slices.Contains(res.WrittenPaths, obsPath) {
		t.Fatalf("expected polaris_observer_otel.go in WrittenPaths, got %v", res.WrittenPaths)
	}
}
```

- [ ] **Step 2: Run to verify it fails** — Expected: FAIL.

- [ ] **Step 3: Create the OTel observer asset** (`internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go`)

```go
// Optional OTel-backed Observer for the Polaris canary adapter (OPT-IN, kitex).
//
// Generated by `ncgo add infra polaris_adapter`. It implements release.Observer
// (defined in ops.go, same package) on top of the OpenTelemetry metric API. The
// generated kitex base template already wires go-framework OTLP and registers a
// global meter provider, so otel.Meter("ncgo.canary") is production-ready.
//
// Credentials are never emitted as metric labels or log fields.
//
// Required dependencies (already present in generated kitex projects via
// go-framework OTLP):
//
//	go.opentelemetry.io/otel
//	go.opentelemetry.io/otel/metric

package release

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelObserver reports decision-level canary metrics via the OTel metric API.
type OTelObserver struct {
	decision  metric.Int64Counter
	fallback  metric.Int64Counter
	discErr   metric.Int64Counter
	ruleErr   metric.Int64Counter
	poolStable metric.Int64Gauge
	poolCanary metric.Int64Gauge
	poolUnknown metric.Int64Gauge
}

// NewOTelObserver builds an Observer on the given meter. Pass nil to use the
// global meter (otel.Meter). Metrics are scoped under "ncgo.canary".
func NewOTelObserver(meter metric.Meter) (*OTelObserver, error) {
	if meter == nil {
		meter = otel.Meter("ncgo.canary")
	}
	o := &OTelObserver{}
	var err error
	if o.decision, err = meter.Int64Counter("ncgo.canary.decision", metric.WithDescription("canary routing decisions")); err != nil {
		return nil, err
	}
	if o.fallback, err = meter.Int64Counter("ncgo.canary.fallback", metric.WithDescription("canary fallback events")); err != nil {
		return nil, err
	}
	if o.discErr, err = meter.Int64Counter("ncgo.canary.discovery_error", metric.WithDescription("discovery failures")); err != nil {
		return nil, err
	}
	if o.ruleErr, err = meter.Int64Counter("ncgo.canary.rule_error", metric.WithDescription("rule load failures")); err != nil {
		return nil, err
	}
	if o.poolStable, err = meter.Int64Gauge("ncgo.canary.pool_size", metric.WithDescription("instance pool size (track=stable)")); err != nil {
		return nil, err
	}
	if o.poolCanary, err = meter.Int64Gauge("ncgo.canary.pool_size_canary", metric.WithDescription("instance pool size (track=canary)")); err != nil {
		return nil, err
	}
	if o.poolUnknown, err = meter.Int64Gauge("ncgo.canary.pool_size_unknown", metric.WithDescription("instance pool size (track=unknown)")); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *OTelObserver) ObserveDecision(service string, d Decision, pools Pools) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String("service", service),
		attribute.String("track", d.Track),
		attribute.String("reason", d.Reason),
	)
	o.decision.Add(ctx, 1, attrs)
	o.poolStable.Record(ctx, int64(len(pools.Stable)), metric.WithAttributes(attribute.String("service", service)))
	o.poolCanary.Record(ctx, int64(len(pools.Canary)), metric.WithAttributes(attribute.String("service", service)))
	o.poolUnknown.Record(ctx, int64(len(pools.Unknown)), metric.WithAttributes(attribute.String("service", service)))
}

func (o *OTelObserver) ObserveFallback(service, reason string) {
	o.fallback.Add(context.Background(), 1, metric.WithAttributes(attribute.String("service", service), attribute.String("reason", reason)))
}

func (o *OTelObserver) ObserveDiscovery(service string, _ int, err error) {
	if err != nil {
		o.discErr.Add(context.Background(), 1, metric.WithAttributes(attribute.String("service", service)))
	}
}

func (o *OTelObserver) ObserveRules(service string, _ int, err error) {
	if err != nil {
		o.ruleErr.Add(context.Background(), 1, metric.WithAttributes(attribute.String("service", service)))
	}
}
```

> **Label-cardinality guard (design §10.4):** the `rule` name is intentionally NOT a metric label (high cardinality); it stays in structured logs only (SlogObserver). Decision metric labels are `service`/`track`/`reason` — bounded.

- [ ] **Step 4: Wire emission + deps** — in `infra.go`:

`assetFiles` polaris branch:
```go
	if infraKind == KindPolarisAdapter {
		if serviceKind != manifest.KindKitex {
			return nil, fmt.Errorf("infra: kind %q is only supported for kitex services", infraKind)
		}
		return []addOnFile{
			{SourcePath: "kitex/optional/polaris_canary_adapter.go", OutputRelPath: outputRelPaths[KindPolarisAdapter]},
			{SourcePath: "kitex/optional/polaris_canary_observer_otel.go", OutputRelPath: filepath.Join("internal", "base", "release", "polaris_observer_otel.go")},
		}, nil
	}
```

`goGetDeps[KindPolarisAdapter]` — add OTel:
```go
	KindPolarisAdapter: {"github.com/polarismesh/polaris-go", "gopkg.in/yaml.v3", "github.com/byx-darwin/go-tools/go-common", "go.opentelemetry.io/otel", "go.opentelemetry.io/otel/metric"},
```

Also extend `nextSteps[KindPolarisAdapter]` (around line 84-90) with `go get go.opentelemetry.io/otel/metric`.

- [ ] **Step 5: Run infra test to verify it passes.**
- [ ] **Step 6: Re-record golden** — `go test ./internal/scaffold/infra/ -run TestGenerateGoldenInfraPolarisAdapter -update-golden -count=1`; confirm `testdata/infra-polaris-adapter/polaris_observer_otel.go` created.
- [ ] **Step 7: Commit**

```bash
git add internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go internal/scaffold/infra/infra.go \
  internal/scaffold/infra/infra_test.go internal/scaffold/infra/testdata/infra-polaris-adapter/polaris_observer_otel.go
git commit -m "feat(infra): emit OTel canary observer with polaris_adapter (#34)"
```

---

## Task 7: Runtime canary harness (AC5) + extend verify script

**Files:**
- Create: `tools/verifyexamples/polaris-adapter/release/harness_test.go`
- Modify: `scripts/verify-polaris-adapter.sh` (copy ops + observer; `go build ./... && go test ./...`)
- Modify: `tools/verifyexamples/polaris-adapter/main.go` (reference observer so it compiles)
- Modify: `tools/verifyexamples/polaris-adapter/go.mod` (via `go mod tidy` — adds `go.opentelemetry.io/otel/metric`)

**Interfaces:**
- Consumes: `Engine`, `NewEngine`, `NewCachingDiscoverer`, `NewCachingRuleProvider`, `StaticDiscoverer`, `StaticRuleProvider`, `NewOTelObserver`, `Weighted`, `RuleSet`.

- [ ] **Step 1: Extend the verify script** — after the existing `cp` lines, add copies for ops + observer, and switch the final build to build+test:

```bash
OPS_ASSET="${REPO_ROOT}/internal/assets/_data/optional/release_ops.go"
OTEL_ASSET="${REPO_ROOT}/internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go"
# ... existing existence checks for ADAPTER_ASSET / CANARY_ASSET ...
if [[ ! -f "${OPS_ASSET}" ]]; then echo "ERROR: ops asset not found: ${OPS_ASSET}" >&2; exit 1; fi
if [[ ! -f "${OTEL_ASSET}" ]]; then echo "ERROR: otel observer asset not found: ${OTEL_ASSET}" >&2; exit 1; fi
# ... after the existing cp commands ...
cp "${OPS_ASSET}" "${RELEASE_PKG}/release_ops.go"
cp "${OTEL_ASSET}" "${RELEASE_PKG}/polaris_canary_observer_otel.go"
# ... replace the final `go build ./...` with: ...
go build ./...
go test ./release/ -count=1
echo "polaris-adapter compile + unit OK"
```

- [ ] **Step 2: Update `main.go`** to reference the observer constructor (so `go build ./...` type-checks the OTel file):

```go
var (
	_ = release.NewPolarisSelector
	_ = release.NewPolarisInstanceLister
	_ = release.NewPolarisRuleLoader
	_ = release.NewOTelObserver
	_ = release.NewEngine
)
```

- [ ] **Step 3: Write the harness tests** (`tools/verifyexamples/polaris-adapter/release/harness_test.go`)

```go
package release

import (
	"context"
	"math"
	"testing"
	"time"
)

// harnessDiscoverer serves 2 stable + 1 canary instance (fake; no live Polaris).
func harnessInstances() []Instance {
	return []Instance{
		{ID: "s1", Address: "10.0.0.1:8080", Weight: 1, Healthy: true, Enabled: true, Metadata: map[string]string{MetadataReleaseTrack: TrackStable}},
		{ID: "s2", Address: "10.0.0.2:8080", Weight: 1, Healthy: true, Enabled: true, Metadata: map[string]string{MetadataReleaseTrack: TrackStable}},
		{ID: "c1", Address: "10.0.0.3:8080", Weight: 1, Healthy: true, Enabled: true, Metadata: map[string]string{MetadataReleaseTrack: TrackCanary}},
	}
}

func harnessRules(canaryPct int) StaticRuleProvider {
	return StaticRuleProvider{RuleSet: RuleSet{
		Version: 1, Enabled: true, Service: "svc", DefaultTrack: TrackStable, Fallback: FallbackStable,
		Rules: []Rule{{Name: "weighted-split", Priority: 10, Weighted: &Weighted{Stable: 100 - canaryPct, Canary: canaryPct}}},
	}}
}

// TestHarness_SplitMatchesWeight asserts the stable/canary split approximates
// the configured weight over many distinct sticky keys.
func TestHarness_SplitMatchesWeight(t *testing.T) {
	e := NewEngine(EngineOptions{ServiceName: "svc", Discoverer: StaticDiscoverer(harnessInstances()), Rules: harnessRules(20)})
	const N = 4000
	canary := 0
	for i := 0; i < N; i++ {
		sel, err := e.Select(context.Background(), Traffic{UserID: "user-" + itoa(i)})
		if err != nil || !sel.OK {
			t.Fatalf("select %d: ok=%v err=%v", i, sel.OK, err)
		}
		if sel.Instance.Track() == TrackCanary {
			canary++
		}
	}
	ratio := float64(canary) / float64(N)
	if math.Abs(ratio-0.20) > 0.05 {
		t.Fatalf("canary ratio = %.3f, want ~0.20 (±0.05)", ratio)
	}
}

// TestHarness_FailOpenKeepsAvailability drives traffic while discovery fails and
// asserts the engine still routes (via cached instances) rather than erroring.
func TestHarness_FailOpenKeepsAvailability(t *testing.T) {
	now, adv := clock(time.Unix(2000, 0))
	up := &fakeDiscoverer{ins: harnessInstances()}
	cached := NewCachingDiscoverer(up, CacheOptions{TTL: time.Second, StaleTTL: time.Minute, Jitter: 0, Now: now}, FailOpen, NopObserver{})
	e := NewEngine(EngineOptions{ServiceName: "svc", Discoverer: cached, Rules: harnessRules(50)})
	if _, err := e.Select(context.Background(), Traffic{UserID: "warm"}); err != nil {
		t.Fatal(err)
	}
	up.err = errDown
	adv(2 * time.Second)
	sel, err := e.Select(context.Background(), Traffic{UserID: "during-outage"})
	if err != nil {
		t.Fatalf("FailOpen must keep routing during outage, got %v", err)
	}
	if !sel.OK {
		t.Fatal("FailOpen with cached instances must select an instance")
	}
}

// TestHarness_DryRunShadow asserts dry-run records canary intent but never
// routes real traffic to the canary pool.
func TestHarness_DryRunShadow(t *testing.T) {
	obs := &recordingObserver{}
	e := NewEngine(EngineOptions{ServiceName: "svc", Discoverer: StaticDiscoverer(harnessInstances()), Rules: harnessRules(100), Observer: obs, DryRun: true})
	for i := 0; i < 50; i++ {
		sel, err := e.Select(context.Background(), Traffic{UserID: "user-" + itoa(i)})
		if err != nil {
			t.Fatal(err)
		}
		if sel.Instance.Track() == TrackCanary {
			t.Fatalf("dry-run routed to canary at i=%d", i)
		}
	}
	if len(obs.decisions) != 50 {
		t.Fatalf("dry-run must still observe decisions; got %d", len(obs.decisions))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
```
(Define `var errDown = errors.New("registry down")` — add `errors` import; reuse `clock`, `fakeDiscoverer`, `recordingObserver` from `ops_test.go` — same package.)

- [ ] **Step 4: Run the verify script** — `./scripts/verify-polaris-adapter.sh`. First run does `go mod tidy` (needs network) to add `go.opentelemetry.io/otel/metric`. Expected: "polaris-adapter compile + unit OK", all harness + ops tests PASS.
- [ ] **Step 5: Commit**

```bash
git add scripts/verify-polaris-adapter.sh tools/verifyexamples/polaris-adapter/main.go \
  tools/verifyexamples/polaris-adapter/release/harness_test.go tools/verifyexamples/polaris-adapter/go.mod tools/verifyexamples/polaris-adapter/go.sum
git commit -m "test(release): runtime canary harness + extend verify gate to compile+test (#34)"
```

---

## Task 8: Asset sanity tests for new files

**Files:**
- Create: `internal/assets/release_ops_asset_test.go`

- [ ] **Step 1: Write the test**

```go
package assets

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

func TestReleaseOpsAssetSanity(t *testing.T) {
	b, err := fs.ReadFile(FS(), "optional/release_ops.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)
	if _, err := parser.ParseFile(token.NewFileSet(), "release_ops.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{"package release", "func NewCachingDiscoverer(", "func NewCachingRuleProvider(", "type Observer interface", "func NewSlogObserver(", "func NewEngine(", "FailOpen", "FailFast"} {
		if !strings.Contains(src, want) {
			t.Errorf("release_ops.go missing %q", want)
		}
	}
	// SDK-neutral: must not import polaris-go or otel.
	for _, bad := range []string{"polarismesh/polaris-go", "go.opentelemetry.io"} {
		if strings.Contains(src, bad) {
			t.Errorf("release_ops.go must stay dependency-neutral, found %q", bad)
		}
	}
}

func TestPolarisOTelObserverAssetSanity(t *testing.T) {
	b, err := fs.ReadFile(FS(), "kitex/optional/polaris_canary_observer_otel.go")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	src := string(b)
	if _, err := parser.ParseFile(token.NewFileSet(), "polaris_canary_observer_otel.go", src, parser.ImportsOnly); err != nil {
		t.Fatalf("asset does not parse: %v", err)
	}
	for _, want := range []string{"package release", "func NewOTelObserver(", "go.opentelemetry.io/otel/metric"} {
		if !strings.Contains(src, want) {
			t.Errorf("observer asset missing %q", want)
		}
	}
	for _, bad := range []string{"POLARIS_TOKEN", "password", "token = \""} {
		if strings.Contains(src, bad) {
			t.Errorf("observer asset appears to leak credentials: %q", bad)
		}
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/assets/ -count=1`. Expected: PASS.
- [ ] **Step 3: Commit**

```bash
git add internal/assets/release_ops_asset_test.go
git commit -m "test(assets): sanity checks for release_ops + otel observer assets (#34)"
```

---

## Task 9: Documentation (EN + ZH) + design §7 refinement

**Files:**
- Modify: `internal/assets/_data/docs/kitex/design-doc.en.md`
- Modify: `internal/assets/_data/docs/kitex/design-doc.zh-CN.md`
- Modify: `docs/superpowers/specs/2026-07-31-polaris-adapter-ga-design.md` (§7 refinement)

- [ ] **Step 1: Add a canary GA subsection to the EN design doc** (after the existing registry/canary content, §3 area). Include:
  - `release_ops.go` decorators: cache (TTL default 30s / stale 5m / jitter ±20%), `FailPolicy` (FailOpen default / FailFast), `Observer` (SlogObserver default; OTel via `NewOTelObserver`), `Engine` + `DryRun`.
  - Resolver-visibility note: kitex resolver uses `GetInstances` (routing-filtered); the canary LB sources the full pool via `KitexCanaryLoadBalancer.Discoverer` (adapter `GetAllInstances`-backed). Show the wiring snippet:
    ```go
    disc, _ := release.NewPolarisInstanceLister(discoveryCfg)
    cached := release.NewCachingDiscoverer(release.PolarisDiscoverer{Config: discoveryCfg, ListInstances: disc}, release.CacheOptions{}, release.FailOpen, obs)
    lb := release.NewKitexCanaryLoadBalancer("svc", rulesProvider, fallbackLB)
    lb.Discoverer = cached
    lb.Observer = obs
    ```
  - Troubleshooting: registry unreachable → FailOpen serves stale/empty; high metric cardinality avoided (rule name in logs only); dry-run usage for first-time RuleSet rollout.

- [ ] **Step 2: Mirror the same content in the ZH design doc** (keep terminology aligned: 缓存/降级语义/可观测性/dry-run/resolver 可见性).

- [ ] **Step 3: Update design doc §7** to record the harness-location refinement (verify module committed tests; verify gate compile+test).

- [ ] **Step 4: Run markdown diagnostics** — `go test ./... -run TestDocs -count=1` if a docs test exists; otherwise verify embedded docs still parse by running `go build ./... && go test ./internal/assets/ -count=1`.

- [ ] **Step 5: Commit**

```bash
git add internal/assets/_data/docs/kitex/design-doc.en.md internal/assets/_data/docs/kitex/design-doc.zh-CN.md \
  docs/superpowers/specs/2026-07-31-polaris-adapter-ga-design.md
git commit -m "docs(release): canary GA hardening (cache/fail/observability/dry-run) EN+ZH (#34)"
```

---

## Task 10: Full validation

- [ ] **Step 1: Repo-wide checks** (from CONTRIBUTING / CLAUDE.md)

```bash
go build ./... && go build .
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './.git/*' -not -path './tools/verifyexamples/*')
go test ./... -count=1
```
Expected: all green; `gofmt -l` prints nothing.

- [ ] **Step 2: Compile+test gate** — `./scripts/verify-polaris-adapter.sh`. Expected: "polaris-adapter compile + unit OK".

- [ ] **Step 3: Byte-identical canonical seam check**

```bash
git diff $(git merge-base HEAD origin/main)..HEAD -- internal/assets/_data/optional/release_canary.go
```
Expected: empty output (canonical seam untouched).

- [ ] **Step 4: Smoke** — `./scripts/smoke.sh`. Expected: pass.

- [ ] **Step 5: Final commit (if any formatting fixups)** — only if step 1 produced changes.

---

## Self-Review

**Spec coverage (design §1 + Issue #34 ACs):**
- AC1 cache+TTL (SWR + jitter + configurable) → Task 1 (`CachingDiscoverer`/`CachingRuleProvider`, `CacheOptions`). ✅
- AC2 explicit fail semantics (fail-open/fail-fast, testable) → Task 1 (`FailPolicy`, FailOpen default) + tests. ✅
- AC3 decision observability (logs + metrics → OTel) → Task 2 (`Observer`/`SlogObserver`) + Task 6 (`OTelObserver`). ✅
- AC4 dry-run/shadow → Task 3 (`Engine.DryRun`). ✅
- AC5 runtime harness (fake instances, assert split) → Task 7 (`harness_test.go`). ✅
- Pre-verification (resolver visibility) → Task 0 (evidence) + Task 5 (LB full-pool). ✅
- Constraints (SDK-neutral core byte-identical, env-only creds, opt-in/kitex-only, CI compile+unit) → Global Constraints + Task 10 step 3 + asset sanity (Task 8). ✅
- Docs EN+ZH → Task 9. ✅

**Placeholder scan:** no TBD/TODO; all code steps contain concrete code; commands have expected output. The Task 1 note about Observer ordering is an explicit sequencing instruction, not a placeholder. ✅

**Type consistency:** `NewCachingDiscoverer(Discoverer, CacheOptions, FailPolicy, Observer)`, `NewEngine(EngineOptions{ServiceName, Discoverer, Rules, Observer, DryRun})`, `Observer` method set (`ObserveDecision(service string, d Decision, pools Pools)`, `ObserveFallback(service, reason string)`, `ObserveDiscovery(service string, instances int, err error)`, `ObserveRules(service string, version int, err error)`) used consistently across Tasks 1–7. `KitexCanaryLoadBalancer.Discoverer/Observer` fields consistent (Task 5). ✅

**Known risk for reviewer:** Task 1 `ttlCache.joinOrStart` lock handling around `wg.Wait()` must be checked for deadlock; the plan releases `c.mu` before waiting and re-acquires after. If flaky, replace with `golang.org/x/sync/singleflight` — but that adds a dependency to the stdlib-only core, so prefer fixing the hand-rolled version.
