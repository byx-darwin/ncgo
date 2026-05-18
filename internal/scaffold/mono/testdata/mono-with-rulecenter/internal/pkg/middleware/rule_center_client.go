// Optional rule-center gRPC client for Hertz HTTP services.
//
// To enable: copy this file to internal/pkg/middleware/rule_center_client.go
// in your project, then wire it into the rate-limit resolver options:
//
//	rlOpts.RuleCenter = middleware.NewRuleCenterClient(cfg.RateLimit.RuleCenter.Address)
//
// Required dependencies:
//
//	go get google.golang.org/grpc
//	go get google.golang.org/grpc/credentials/insecure
//	go get github.com/samber/oops
//
// TODO(baoyx): This standalone gRPC client is a temporary bridge solution.
// It should eventually be replaced by consuming the Kitex-generated client
// (pkg/client/ruleserviceclient/) which includes retry, circuit-breaker,
// and caller-service metadata. The Kitex-generated client lives alongside
// the rule-center Kitex service.

package middleware

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/x/demo/internal/base/conf"
	"github.com/x/demo/internal/pkg/ratelimit"
	"github.com/x/demo/kitex_gen/ratelimit/v1"
)

// RuleCenterConfig mirrors the timeout pattern of Kitex's generated
// client.Config (pkg/client/<service>/client.go). It controls per-call RPC
// timeout and connection verification for the rule-center gRPC client.
type RuleCenterConfig struct {
	Address                string
	RPCTimeoutMilliseconds int // per-call RPC timeout; default 200ms
	ConnectTimeoutMillis   int // initial connection timeout; default 100ms
}

// DefaultRuleCenterConfig returns a config with safe defaults, mirroring
// the Kitex client pattern (100ms connect, 200ms RPC).
func DefaultRuleCenterConfig(address string) RuleCenterConfig {
	return RuleCenterConfig{
		Address:                address,
		RPCTimeoutMilliseconds: 200,
		ConnectTimeoutMillis:   100,
	}
}

// RuleCenterClient implements ratelimit.GRPCClient by querying the
// rule-center Kitex service over gRPC.
type RuleCenterClient struct {
	conn    *grpc.ClientConn
	cli     ratelimitv1.RuleServiceClient
	timeout time.Duration
}

// NewRuleCenterClient creates a gRPC client connected to the rule-center
// service at the given address (e.g. "rule-center.internal:8888").
// Uses DefaultRuleCenterConfig for timeouts.
func NewRuleCenterClient(address string) (*RuleCenterClient, error) {
	if address == "" {
		return nil, fmt.Errorf("rule_center: address is required")
	}
	return NewRuleCenterClientWithConfig(DefaultRuleCenterConfig(address))
}

// NewRuleCenterClientWithTimeout creates a gRPC client with a custom
// per-call RPC timeout.
func NewRuleCenterClientWithTimeout(address string, timeout time.Duration) (*RuleCenterClient, error) {
	if address == "" {
		return nil, fmt.Errorf("rule_center: address is required")
	}
	cfg := DefaultRuleCenterConfig(address)
	cfg.RPCTimeoutMilliseconds = int(timeout.Milliseconds())
	return NewRuleCenterClientWithConfig(cfg)
}

// NewRuleCenterClientWithConfig creates a gRPC client with explicit timeout
// configuration. This mirrors the Kitex client.New(ctx, cfg) pattern.
// Unlike Kitex clients which use `kitexclient.WithConnectTimeout` and
// `kitexclient.WithRPCTimeout`, the gRPC client uses a context deadline
// on `grpc.Dial` for connect timeout and per-call context deadline for RPC.
func NewRuleCenterClientWithConfig(cfg RuleCenterConfig) (*RuleCenterClient, error) {
	rpcTimeout := time.Duration(cfg.RPCTimeoutMilliseconds) * time.Millisecond
	if rpcTimeout <= 0 {
		rpcTimeout = 200 * time.Millisecond
	}
	connectTimeout := time.Duration(cfg.ConnectTimeoutMillis) * time.Millisecond
	if connectTimeout <= 0 {
		connectTimeout = 100 * time.Millisecond
	}

	connCtx, connCancel := context.WithTimeout(context.Background(), connectTimeout)
	defer connCancel()

	conn, err := grpc.DialContext(connCtx, cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("rule_center: connect %s: %w", cfg.Address, err)
	}

	return &RuleCenterClient{
		conn:    conn,
		cli:     ratelimitv1.NewRuleServiceClient(conn),
		timeout: rpcTimeout,
	}, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Close releases the underlying gRPC connection.
func (c *RuleCenterClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ResolveRateLimitRule implements ratelimit.GRPCClient.
// It calls the rule-center GetRule RPC and converts the proto response
// to *conf.RateLimitRuleConfig.
//
// Return contract:
//   - (*conf.RateLimitRuleConfig, true, nil): matched a remote rule
//   - (nil, false, nil): remote answered "not found"
//   - (nil, false, error): remote query failed
func (c *RuleCenterClient) ResolveRateLimitRule(ctx context.Context, lookup ratelimit.Lookup) (*conf.RateLimitRuleConfig, bool, error) {
	to := c.timeout
	if to <= 0 {
		to = 200 * time.Millisecond
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, to)
	defer cancel()

	resp, err := c.cli.GetRule(ctx, &ratelimitv1.GetRuleRequest{
		Service: lookup.Service,
		Phase:   lookup.Phase,
		Method:  lookup.Method,
		Path:    lookup.Path,
		AppKey:  strPtrOrNil(lookup.AppKey),
	})
	if err != nil {
		return nil, false, fmt.Errorf("rule_center: GetRule(%s, %s, %s, %s): %w",
			lookup.Service, lookup.Phase, lookup.Method, lookup.Path, err)
	}

	if !resp.GetFound() {
		return nil, false, nil
	}

	rule := resp.GetRule()
	if rule == nil {
		return nil, false, nil
	}

	cfg := &conf.RateLimitRuleConfig{
		Enabled:          true,
		Strategy:         rule.GetStrategy(),
		WindowSeconds:    int(rule.GetWindowSeconds()),
		MaxRequests:      int(rule.GetMaxRequests()),
		KeyBy:            rule.GetKeyBy(),
		ClientTTLSeconds: int(rule.GetClientTtlSeconds()),
	}
	return cfg, true, nil
}
