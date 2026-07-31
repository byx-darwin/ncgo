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
	"errors"
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
	// StaleTTL=0 means "no stale window" (don't serve stale). Negative values
	// are treated as unset and receive the production default.
	if o.StaleTTL < 0 {
		o.StaleTTL = defaultCacheStaleTTL
	}
	// Jitter=0 means "no jitter". Negative values are treated as unset.
	if o.Jitter < 0 {
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
	opts  CacheOptions
	mu    sync.Mutex
	ents  map[string]cacheEntry[T]
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
	if wg, ok := c.calls[key]; ok {
		c.mu.Unlock()
		wg.Wait()
		return false
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	c.calls[key] = wg
	c.mu.Unlock()
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
	if !needRefresh {
		return entry.value, nil
	}
	// Within the stale window with a good cached value: serve stale, refresh async.
	if entry.err == nil && entry.fetched != (time.Time{}) && c.cache.opts.Now().Sub(entry.fetched) < entry.ttl+c.cache.opts.StaleTTL {
		if c.cache.joinOrStart(serviceName) {
			go c.refresh(context.Background(), serviceName)
		}
		return entry.value, nil
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
		c.cache.mu.Lock()
		prev, ok := c.cache.ents[serviceName]
		if ok && prev.err == nil {
			c.cache.ents[serviceName] = cacheEntry[[]Instance]{value: prev.value, fetched: prev.fetched, ttl: prev.ttl, err: err}
			c.cache.mu.Unlock()
			return
		}
		c.cache.mu.Unlock()
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
	if entry.err == nil && entry.fetched != (time.Time{}) && c.cache.opts.Now().Sub(entry.fetched) < entry.ttl+c.cache.opts.StaleTTL {
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
		if ok && prev.err == nil {
			c.cache.ents[serviceName] = cacheEntry[RuleSet]{value: prev.value, fetched: prev.fetched, ttl: prev.ttl, err: err}
			c.cache.mu.Unlock()
			return
		}
		c.cache.mu.Unlock()
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
