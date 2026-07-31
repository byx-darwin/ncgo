package release

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// errDown simulates a Polaris registry outage for FailOpen tests.
var errDown = errors.New("registry down")

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
