// Optional Kitex release-canary adapter for internal/base/release.

package release

import (
	"context"
	"net"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/loadbalance"
)

const (
	MetadataTrafficLane = "traffic.lane"
	MetadataUserID      = "traffic.user_id"
	MetadataTenantID    = "traffic.tenant_id"
)

func KitexTraffic() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			traffic := TrafficFromKitex(ctx)
			ctx = WithTraffic(ctx, traffic)
			ctx = InjectKitexTraffic(ctx, traffic)
			return next(ctx, req, resp)
		}
	}
}

func TrafficFromKitex(ctx context.Context) Traffic {
	lane := firstNonEmpty(KitexMetaValue(ctx, MetadataTrafficLane), KitexMetaValue(ctx, HeaderTrafficLane))
	userID := firstNonEmpty(KitexMetaValue(ctx, MetadataUserID), KitexMetaValue(ctx, HeaderUserID))
	tenantID := firstNonEmpty(KitexMetaValue(ctx, MetadataTenantID), KitexMetaValue(ctx, HeaderTenantID))
	return Traffic{
		Lane:      lane,
		UserID:    userID,
		TenantID:  tenantID,
		StickyKey: firstNonEmpty(userID, tenantID, lane),
		Headers: map[string]string{
			MetadataTrafficLane: lane,
			MetadataUserID:      userID,
			MetadataTenantID:    tenantID,
		},
	}
}

func InjectKitexTraffic(ctx context.Context, traffic Traffic) context.Context {
	if traffic.Lane != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetadataTrafficLane, traffic.Lane)
		ctx = metainfo.WithPersistentValue(ctx, HeaderTrafficLane, traffic.Lane)
	}
	if traffic.UserID != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetadataUserID, traffic.UserID)
		ctx = metainfo.WithPersistentValue(ctx, HeaderUserID, traffic.UserID)
	}
	if traffic.TenantID != "" {
		ctx = metainfo.WithPersistentValue(ctx, MetadataTenantID, traffic.TenantID)
		ctx = metainfo.WithPersistentValue(ctx, HeaderTenantID, traffic.TenantID)
	}
	return ctx
}

func KitexMetaValue(ctx context.Context, key string) string {
	if value, ok := metainfo.GetPersistentValue(ctx, key); ok {
		return value
	}
	if value, ok := metainfo.GetValue(ctx, key); ok {
		return value
	}
	return ""
}

func KitexDecision(ctx context.Context, rules RuleSet) Decision {
	return Decide(TrafficFromContext(ctx), rules)
}

type KitexCanaryLoadBalancer struct {
	ServiceName  string
	RuleProvider RuleProvider
	Fallback     loadbalance.Loadbalancer
	// Discoverer, when non-nil, supplies the FULL stable+canary instance set
	// (e.g. a *CachingDiscoverer wrapping the polaris adapter's GetAllInstances
	// Discoverer). This decouples canary splitting from the kitex resolver,
	// which may return a routing-filtered subset. When nil, the picker falls
	// back to the resolver-provided discovery.Result (legacy behavior).
	Discoverer Discoverer
	Observer   Observer
}

func NewKitexCanaryLoadBalancer(serviceName string, rules RuleProvider, fallback loadbalance.Loadbalancer) KitexCanaryLoadBalancer {
	return KitexCanaryLoadBalancer{ServiceName: serviceName, RuleProvider: rules, Fallback: fallback}
}

func (lb KitexCanaryLoadBalancer) GetPicker(result discovery.Result) loadbalance.Picker {
	var fallback loadbalance.Picker
	if lb.Fallback != nil {
		fallback = lb.Fallback.GetPicker(result)
	}
	return kitexCanaryPicker{serviceName: lb.ServiceName, result: result, rules: lb.RuleProvider, fallback: fallback, discoverer: lb.Discoverer, observer: lb.Observer}
}

func (lb KitexCanaryLoadBalancer) Name() string { return "ncgo_canary" }

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

func (p kitexCanaryPicker) fallbackOrWeighted(ctx context.Context, request interface{}, traffic Traffic) discovery.Instance {
	if p.fallback != nil {
		return p.fallback.Next(ctx, request)
	}
	return pickKitexWeighted(p.result.Instances, firstNonEmpty(traffic.StickyKey, traffic.UserID, traffic.TenantID, traffic.Lane, p.serviceName))
}

type KitexResultDiscoverer struct {
	Result discovery.Result
}

func (d KitexResultDiscoverer) Discover(ctx context.Context, serviceName string) ([]Instance, error) {
	_ = ctx
	_ = serviceName
	return releaseInstancesFromKitex(d.Result.Instances), nil
}

func releaseInstancesFromKitex(instances []discovery.Instance) []Instance {
	out := make([]Instance, 0, len(instances))
	for _, ins := range instances {
		if ins == nil || ins.Address() == nil {
			continue
		}
		metadata := kitexInstanceTags(ins)
		out = append(out, Instance{
			ID:          ins.Address().String(),
			ServiceName: metadata[MetadataServiceName],
			Address:     ins.Address().String(),
			Weight:      ins.Weight(),
			Healthy:     true,
			Enabled:     true,
			Metadata:    metadata,
		})
	}
	return out
}

func kitexInstanceTags(ins discovery.Instance) map[string]string {
	metadata := map[string]string{}
	if tagged, ok := ins.(interface{ Tags() map[string]string }); ok {
		for k, v := range tagged.Tags() {
			metadata[k] = v
		}
	}
	for _, key := range []string{MetadataServiceName, MetadataVersion, MetadataReleaseTrack, MetadataEnv} {
		if _, ok := metadata[key]; ok {
			continue
		}
		if value, ok := ins.Tag(key); ok {
			metadata[key] = value
		}
	}
	return metadata
}

func findKitexInstance(instances []discovery.Instance, id string) discovery.Instance {
	for _, ins := range instances {
		if ins != nil && ins.Address() != nil && ins.Address().String() == id {
			return ins
		}
	}
	return nil
}

// releaseInstance is a minimal discovery.Instance synthesized from a full-pool
// release.Instance when the resolver result does not contain it. It satisfies
// kitex's discovery.Instance interface (Address, Weight, Tag).
type releaseInstance struct {
	addr   net.Addr
	weight int
	tags   map[string]string
}

func (i releaseInstance) Address() net.Addr { return i.addr }
func (i releaseInstance) Weight() int       { return i.weight }
func (i releaseInstance) Tag(key string) (string, bool) {
	v, ok := i.tags[key]
	return v, ok
}

// kitexInstanceFromRelease synthesizes a kitex discovery.Instance from a
// release.Instance. Used when the Discoverer-backed engine picks an instance
// that is absent from the resolver's (possibly filtered) result.
func kitexInstanceFromRelease(ins Instance) discovery.Instance {
	tags := map[string]string{}
	for k, v := range ins.Metadata {
		tags[k] = v
	}
	return releaseInstance{addr: parseReleaseAddr(ins.Address), weight: ins.Weight, tags: tags}
}

// parseReleaseAddr turns a "host:port" string (or bare IP) into a net.Addr.
// The result is usable as a kitex instance address for downstream dialing.
func parseReleaseAddr(s string) net.Addr {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		return &net.TCPAddr{IP: net.ParseIP(s)}
	}
	p, _ := strconv.Atoi(port)
	return &net.TCPAddr{IP: net.ParseIP(host), Port: p}
}

func pickKitexWeighted(instances []discovery.Instance, stickyKey string) discovery.Instance {
	available := make([]discovery.Instance, 0, len(instances))
	for _, ins := range instances {
		if ins != nil {
			available = append(available, ins)
		}
	}
	if len(available) == 0 {
		return nil
	}
	total := 0
	for _, ins := range available {
		if ins.Weight() > 0 {
			total += ins.Weight()
		}
	}
	if total <= 0 {
		return available[int(hash(stickyKey)%uint32(len(available)))]
	}
	bucket := int(hash(stickyKey) % uint32(total))
	for _, ins := range available {
		if ins.Weight() <= 0 {
			continue
		}
		if bucket < ins.Weight() {
			return ins
		}
		bucket -= ins.Weight()
	}
	return available[0]
}
