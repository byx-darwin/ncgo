// Optional release canary add-on for Hertz and Kitex services.
//
// This package is intentionally SDK-neutral. It defines the common release
// metadata, canary rule model, traffic context, and Nacos/Polaris discovery
// instance model. Wire concrete Nacos/Polaris SDK clients in adapters later.

package release

import (
	"context"
	"errors"
	"hash/fnv"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	TrackStable = "stable"
	TrackCanary = "canary"

	FallbackStable   = "stable"
	FallbackFailFast = "fail_fast"

	ProviderNacos   = "nacos"
	ProviderPolaris = "polaris"

	MetadataServiceName  = "service.name"
	MetadataServiceKind  = "service.kind"
	MetadataVersion      = "service.version"
	MetadataReleaseTrack = "release.track"
	MetadataGitSHA       = "release.git_sha"
	MetadataBuildTime    = "release.build_time"
	MetadataEnv          = "release.env"

	HeaderTrafficLane = "X-Traffic-Lane"
	HeaderUserID      = "X-User-ID"
	HeaderTenantID    = "X-Tenant-ID"
)

type ReleaseInfo struct {
	ServiceName string `json:"service_name" yaml:"service_name"`
	ServiceKind string `json:"service_kind" yaml:"service_kind"`
	Version     string `json:"version" yaml:"version"`
	Track       string `json:"track" yaml:"track"`
	GitSHA      string `json:"git_sha" yaml:"git_sha"`
	BuildTime   string `json:"build_time" yaml:"build_time"`
	Env         string `json:"env" yaml:"env"`
}

func FromEnv(serviceName, serviceKind string) ReleaseInfo {
	return ReleaseInfo{
		ServiceName: firstNonEmpty(os.Getenv("SERVICE_NAME"), serviceName),
		ServiceKind: firstNonEmpty(os.Getenv("SERVICE_KIND"), serviceKind),
		Version:     firstNonEmpty(os.Getenv("SERVICE_VERSION"), "dev"),
		Track:       NormalizeTrack(firstNonEmpty(os.Getenv("RELEASE_TRACK"), TrackStable)),
		GitSHA:      os.Getenv("GIT_SHA"),
		BuildTime:   firstNonEmpty(os.Getenv("BUILD_TIME"), time.Now().UTC().Format(time.RFC3339)),
		Env:         os.Getenv("RELEASE_ENV"),
	}
}

func (r ReleaseInfo) Metadata(extra map[string]string) map[string]string {
	out := map[string]string{}
	put(out, MetadataServiceName, r.ServiceName)
	put(out, MetadataServiceKind, r.ServiceKind)
	put(out, MetadataVersion, r.Version)
	put(out, MetadataReleaseTrack, NormalizeTrack(firstNonEmpty(r.Track, TrackStable)))
	put(out, MetadataGitSHA, r.GitSHA)
	put(out, MetadataBuildTime, r.BuildTime)
	put(out, MetadataEnv, r.Env)
	for k, v := range extra {
		put(out, k, v)
	}
	return out
}

type Traffic struct {
	Lane      string            `json:"lane" yaml:"lane"`
	UserID    string            `json:"user_id" yaml:"user_id"`
	TenantID  string            `json:"tenant_id" yaml:"tenant_id"`
	Region    string            `json:"region" yaml:"region"`
	StickyKey string            `json:"sticky_key" yaml:"sticky_key"`
	Headers   map[string]string `json:"headers" yaml:"headers"`
	Cookies   map[string]string `json:"cookies" yaml:"cookies"`
}

type contextKey string

const contextKeyTraffic contextKey = "release_traffic"

func WithTraffic(ctx context.Context, traffic Traffic) context.Context {
	return context.WithValue(ctx, contextKeyTraffic, traffic)
}

func TrafficFromContext(ctx context.Context) Traffic {
	traffic, _ := ctx.Value(contextKeyTraffic).(Traffic)
	return traffic
}

type RuleSet struct {
	Version      int    `json:"version" yaml:"version"`
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	Service      string `json:"service" yaml:"service"`
	DefaultTrack string `json:"default_track" yaml:"default_track"`
	Fallback     string `json:"fallback" yaml:"fallback"`
	Rules        []Rule `json:"rules" yaml:"rules"`
}

type Rule struct {
	Name     string    `json:"name" yaml:"name"`
	Priority int       `json:"priority" yaml:"priority"`
	Match    Match     `json:"match" yaml:"match"`
	Track    string    `json:"track" yaml:"track"`
	Weighted *Weighted `json:"weighted" yaml:"weighted"`
}

type Match struct {
	Headers map[string]string `json:"headers" yaml:"headers"`
	Cookies map[string]string `json:"cookies" yaml:"cookies"`
	Users   []string          `json:"users" yaml:"users"`
	Tenants []string          `json:"tenants" yaml:"tenants"`
	Regions []string          `json:"regions" yaml:"regions"`
}

type Weighted struct {
	Stable int `json:"stable" yaml:"stable"`
	Canary int `json:"canary" yaml:"canary"`
}

type Decision struct {
	Track    string
	Fallback string
	Reason   string
	Rule     string
}

func Decide(traffic Traffic, rules RuleSet) Decision {
	fallback := NormalizeFallback(firstNonEmpty(rules.Fallback, FallbackStable))
	if track := NormalizeTrack(traffic.Lane); track != "" {
		return Decision{Track: track, Fallback: fallback, Reason: "explicit_lane"}
	}
	defaultTrack := normalizeTrackOrDefault(rules.DefaultTrack, TrackStable)
	if !rules.Enabled {
		return Decision{Track: defaultTrack, Fallback: fallback, Reason: "disabled"}
	}
	ordered := append([]Rule(nil), rules.Rules...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority > ordered[j].Priority })
	for _, rule := range ordered {
		if !matchTraffic(traffic, rule.Match) {
			continue
		}
		if rule.Weighted != nil {
			return Decision{Track: weightedTrack(traffic, *rule.Weighted, defaultTrack), Fallback: fallback, Reason: "weighted", Rule: rule.Name}
		}
		if track := NormalizeTrack(rule.Track); track != "" {
			return Decision{Track: track, Fallback: fallback, Reason: "rule", Rule: rule.Name}
		}
	}
	return Decision{Track: defaultTrack, Fallback: fallback, Reason: "default"}
}

type Instance struct {
	ID          string            `json:"id" yaml:"id"`
	Provider    string            `json:"provider" yaml:"provider"`
	ServiceName string            `json:"service_name" yaml:"service_name"`
	Namespace   string            `json:"namespace" yaml:"namespace"`
	Group       string            `json:"group" yaml:"group"`
	Cluster     string            `json:"cluster" yaml:"cluster"`
	Address     string            `json:"address" yaml:"address"`
	Weight      int               `json:"weight" yaml:"weight"`
	Healthy     bool              `json:"healthy" yaml:"healthy"`
	Enabled     bool              `json:"enabled" yaml:"enabled"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
}

func (i Instance) Track() string {
	return NormalizeTrack(i.Metadata[MetadataReleaseTrack])
}

type Pools struct {
	Stable  []Instance
	Canary  []Instance
	Unknown []Instance
}

func SplitInstances(instances []Instance) Pools {
	pools := Pools{}
	for _, ins := range instances {
		if !ins.Healthy || !ins.Enabled {
			continue
		}
		switch ins.Track() {
		case TrackStable:
			pools.Stable = append(pools.Stable, ins)
		case TrackCanary:
			pools.Canary = append(pools.Canary, ins)
		default:
			pools.Unknown = append(pools.Unknown, ins)
		}
	}
	return pools
}

func SelectInstance(pools Pools, decision Decision, stickyKey string) (Instance, bool) {
	switch NormalizeTrack(decision.Track) {
	case TrackCanary:
		if ins, ok := pickWeighted(pools.Canary, stickyKey); ok {
			return ins, true
		}
		if NormalizeFallback(decision.Fallback) == FallbackFailFast {
			return Instance{}, false
		}
		return pickWeighted(pools.Stable, stickyKey)
	default:
		if ins, ok := pickWeighted(pools.Stable, stickyKey); ok {
			return ins, true
		}
		return pickWeighted(pools.Unknown, stickyKey)
	}
}

type Discoverer interface {
	Discover(ctx context.Context, serviceName string) ([]Instance, error)
}

type RuleProvider interface {
	Rules(ctx context.Context, serviceName string) (RuleSet, error)
}

type StaticDiscoverer []Instance

func (d StaticDiscoverer) Discover(ctx context.Context, serviceName string) ([]Instance, error) {
	_ = ctx
	_ = serviceName
	return append([]Instance(nil), d...), nil
}

type StaticRuleProvider struct {
	RuleSet RuleSet
}

func (p StaticRuleProvider) Rules(ctx context.Context, serviceName string) (RuleSet, error) {
	_ = ctx
	if p.RuleSet.Service == "" {
		p.RuleSet.Service = serviceName
	}
	return p.RuleSet, nil
}

type Selector struct {
	ServiceName  string
	Discoverer   Discoverer
	RuleProvider RuleProvider
}

type Selection struct {
	Instance Instance
	Decision Decision
	Pools    Pools
	OK       bool
}

func (s Selector) Select(ctx context.Context, traffic Traffic) (Selection, error) {
	if s.Discoverer == nil {
		return Selection{}, errors.New("release selector: Discoverer is nil")
	}
	instances, err := s.Discoverer.Discover(ctx, s.ServiceName)
	if err != nil {
		return Selection{}, err
	}
	rules := RuleSet{Enabled: false, Service: s.ServiceName, DefaultTrack: TrackStable, Fallback: FallbackStable}
	if s.RuleProvider != nil {
		rules, err = s.RuleProvider.Rules(ctx, s.ServiceName)
		if err != nil {
			return Selection{}, err
		}
	}
	return Select(traffic, instances, rules), nil
}

func Select(traffic Traffic, instances []Instance, rules RuleSet) Selection {
	pools := SplitInstances(instances)
	decision := Decide(traffic, rules)
	stickyKey := firstNonEmpty(traffic.StickyKey, traffic.UserID, traffic.TenantID, traffic.Lane, rules.Service)
	instance, ok := SelectInstance(pools, decision, stickyKey)
	return Selection{Instance: instance, Decision: decision, Pools: pools, OK: ok}
}

type NacosDiscoveryConfig struct {
	NamespaceID string `json:"namespace_id" yaml:"namespace_id"`
	GroupName   string `json:"group_name" yaml:"group_name"`
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`
	ServiceName string `json:"service_name" yaml:"service_name"`
}

type PolarisDiscoveryConfig struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Service   string `json:"service" yaml:"service"`
}

const (
	DefaultNacosCanaryGroup   = "NCGO_CANARY"
	DefaultPolarisCanaryGroup = "ncgo-canary"
)

type NacosRuleConfig struct {
	NamespaceID string `json:"namespace_id" yaml:"namespace_id"`
	GroupName   string `json:"group_name" yaml:"group_name"`
	DataID      string `json:"data_id" yaml:"data_id"`
}

type PolarisRuleConfig struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Group     string `json:"group" yaml:"group"`
	FileName  string `json:"file_name" yaml:"file_name"`
}

type NacosInstance struct {
	ID          string            `json:"id" yaml:"id"`
	ServiceName string            `json:"service_name" yaml:"service_name"`
	GroupName   string            `json:"group_name" yaml:"group_name"`
	ClusterName string            `json:"cluster_name" yaml:"cluster_name"`
	IP          string            `json:"ip" yaml:"ip"`
	Port        int               `json:"port" yaml:"port"`
	Weight      int               `json:"weight" yaml:"weight"`
	Healthy     bool              `json:"healthy" yaml:"healthy"`
	Enabled     bool              `json:"enabled" yaml:"enabled"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
}

type PolarisInstance struct {
	ID        string            `json:"id" yaml:"id"`
	Namespace string            `json:"namespace" yaml:"namespace"`
	Service   string            `json:"service" yaml:"service"`
	Host      string            `json:"host" yaml:"host"`
	Port      int               `json:"port" yaml:"port"`
	Weight    int               `json:"weight" yaml:"weight"`
	Healthy   bool              `json:"healthy" yaml:"healthy"`
	Isolate   bool              `json:"isolate" yaml:"isolate"`
	Metadata  map[string]string `json:"metadata" yaml:"metadata"`
}

type NacosInstanceLister func(ctx context.Context, cfg NacosDiscoveryConfig) ([]NacosInstance, error)
type PolarisInstanceLister func(ctx context.Context, cfg PolarisDiscoveryConfig) ([]PolarisInstance, error)
type NacosRuleLoader func(ctx context.Context, cfg NacosRuleConfig) (RuleSet, error)
type PolarisRuleLoader func(ctx context.Context, cfg PolarisRuleConfig) (RuleSet, error)

type NacosDiscoverer struct {
	Config        NacosDiscoveryConfig
	ListInstances NacosInstanceLister
}

func (d NacosDiscoverer) Discover(ctx context.Context, serviceName string) ([]Instance, error) {
	if d.ListInstances == nil {
		return nil, errors.New("release nacos discoverer: ListInstances is nil")
	}
	cfg := d.Config
	if cfg.ServiceName == "" {
		cfg.ServiceName = serviceName
	}
	instances, err := d.ListInstances(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return InstancesFromNacos(instances, cfg), nil
}

type PolarisDiscoverer struct {
	Config        PolarisDiscoveryConfig
	ListInstances PolarisInstanceLister
}

func (d PolarisDiscoverer) Discover(ctx context.Context, serviceName string) ([]Instance, error) {
	if d.ListInstances == nil {
		return nil, errors.New("release polaris discoverer: ListInstances is nil")
	}
	cfg := d.Config
	if cfg.Service == "" {
		cfg.Service = serviceName
	}
	instances, err := d.ListInstances(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return InstancesFromPolaris(instances, cfg), nil
}

type NacosRuleProvider struct {
	Config    NacosRuleConfig
	LoadRules NacosRuleLoader
}

func (p NacosRuleProvider) Rules(ctx context.Context, serviceName string) (RuleSet, error) {
	if p.LoadRules == nil {
		return RuleSet{}, errors.New("release nacos rule provider: LoadRules is nil")
	}
	cfg := p.Config
	if cfg.GroupName == "" {
		cfg.GroupName = DefaultNacosCanaryGroup
	}
	if cfg.DataID == "" {
		cfg.DataID = CanaryRuleFileName(serviceName)
	}
	rules, err := p.LoadRules(ctx, cfg)
	if err != nil {
		return RuleSet{}, err
	}
	if rules.Service == "" {
		rules.Service = serviceName
	}
	return rules, nil
}

type PolarisRuleProvider struct {
	Config    PolarisRuleConfig
	LoadRules PolarisRuleLoader
}

func (p PolarisRuleProvider) Rules(ctx context.Context, serviceName string) (RuleSet, error) {
	if p.LoadRules == nil {
		return RuleSet{}, errors.New("release polaris rule provider: LoadRules is nil")
	}
	cfg := p.Config
	if cfg.Group == "" {
		cfg.Group = DefaultPolarisCanaryGroup
	}
	if cfg.FileName == "" {
		cfg.FileName = CanaryRuleFileName(serviceName)
	}
	rules, err := p.LoadRules(ctx, cfg)
	if err != nil {
		return RuleSet{}, err
	}
	if rules.Service == "" {
		rules.Service = serviceName
	}
	return rules, nil
}

func InstancesFromNacos(instances []NacosInstance, cfg NacosDiscoveryConfig) []Instance {
	out := make([]Instance, 0, len(instances))
	for _, ins := range instances {
		metadata := copyMetadata(ins.Metadata)
		serviceName := firstNonEmpty(ins.ServiceName, cfg.ServiceName, metadata[MetadataServiceName])
		address := hostPort(ins.IP, ins.Port)
		putDefault(metadata, MetadataServiceName, serviceName)
		weight := firstPositive(ins.Weight, MetadataWeight(metadata))
		out = append(out, Instance{
			ID:          firstNonEmpty(ins.ID, address),
			Provider:    ProviderNacos,
			ServiceName: serviceName,
			Namespace:   cfg.NamespaceID,
			Group:       firstNonEmpty(ins.GroupName, cfg.GroupName),
			Cluster:     firstNonEmpty(ins.ClusterName, cfg.ClusterName),
			Address:     address,
			Weight:      weight,
			Healthy:     ins.Healthy,
			Enabled:     ins.Enabled,
			Metadata:    metadata,
		})
	}
	return out
}

func InstancesFromPolaris(instances []PolarisInstance, cfg PolarisDiscoveryConfig) []Instance {
	out := make([]Instance, 0, len(instances))
	for _, ins := range instances {
		metadata := copyMetadata(ins.Metadata)
		serviceName := firstNonEmpty(ins.Service, cfg.Service, metadata[MetadataServiceName])
		address := hostPort(ins.Host, ins.Port)
		putDefault(metadata, MetadataServiceName, serviceName)
		weight := firstPositive(ins.Weight, MetadataWeight(metadata))
		out = append(out, Instance{
			ID:          firstNonEmpty(ins.ID, address),
			Provider:    ProviderPolaris,
			ServiceName: serviceName,
			Namespace:   firstNonEmpty(ins.Namespace, cfg.Namespace),
			Address:     address,
			Weight:      weight,
			Healthy:     ins.Healthy,
			Enabled:     !ins.Isolate,
			Metadata:    metadata,
		})
	}
	return out
}

func CanaryRuleFileName(serviceName string) string {
	if strings.TrimSpace(serviceName) == "" {
		return "canary.yaml"
	}
	return strings.TrimSpace(serviceName) + ".canary.yaml"
}

func NormalizeFallback(fallback string) string {
	switch strings.ToLower(strings.TrimSpace(fallback)) {
	case FallbackFailFast:
		return FallbackFailFast
	default:
		return FallbackStable
	}
}

func NormalizeTrack(track string) string {
	switch strings.ToLower(strings.TrimSpace(track)) {
	case TrackStable:
		return TrackStable
	case TrackCanary:
		return TrackCanary
	default:
		return ""
	}
}

func normalizeTrackOrDefault(track, fallback string) string {
	if normalized := NormalizeTrack(track); normalized != "" {
		return normalized
	}
	return fallback
}

func matchTraffic(t Traffic, m Match) bool {
	return matchKV(t.Headers, m.Headers) && matchKV(t.Cookies, m.Cookies) && matchList(t.UserID, m.Users) && matchList(t.TenantID, m.Tenants) && matchList(t.Region, m.Regions)
}

func matchKV(actual, want map[string]string) bool {
	for k, v := range want {
		if actual == nil || actual[k] != v {
			return false
		}
	}
	return true
}

func matchList(actual string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, v := range allowed {
		if v == actual {
			return true
		}
	}
	return false
}

func weightedTrack(t Traffic, w Weighted, fallback string) string {
	total := w.Stable + w.Canary
	if total <= 0 || w.Canary <= 0 {
		return fallback
	}
	key := firstNonEmpty(t.StickyKey, t.UserID, t.TenantID, t.Region, fallback)
	if int(hash(key)%uint32(total)) < w.Canary {
		return TrackCanary
	}
	return TrackStable
}

func pickWeighted(instances []Instance, stickyKey string) (Instance, bool) {
	if len(instances) == 0 {
		return Instance{}, false
	}
	total := 0
	for _, ins := range instances {
		if ins.Weight > 0 {
			total += ins.Weight
		}
	}
	if total <= 0 {
		return instances[int(hash(stickyKey)%uint32(len(instances)))], true
	}
	bucket := int(hash(stickyKey) % uint32(total))
	for _, ins := range instances {
		if ins.Weight <= 0 {
			continue
		}
		if bucket < ins.Weight {
			return ins, true
		}
		bucket -= ins.Weight
	}
	return instances[0], true
}

func hash(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func put(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func putDefault(m map[string]string, key, value string) {
	if m[key] == "" {
		put(m, key, value)
	}
}

func copyMetadata(metadata map[string]string) map[string]string {
	out := make(map[string]string, len(metadata)+1)
	for k, v := range metadata {
		out[k] = v
	}
	return out
}

func hostPort(host string, port int) string {
	if port <= 0 || host == "" {
		return host
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + strconv.Itoa(port)
	}
	return host + ":" + strconv.Itoa(port)
}

func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func MetadataWeight(metadata map[string]string) int {
	if metadata == nil {
		return 0
	}
	weight, _ := strconv.Atoi(metadata["weight"])
	return weight
}
