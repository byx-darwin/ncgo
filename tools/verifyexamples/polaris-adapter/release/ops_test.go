package release

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
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
