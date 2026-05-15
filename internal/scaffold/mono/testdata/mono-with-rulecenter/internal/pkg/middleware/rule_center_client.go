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

// RuleCenterClient implements ratelimit.GRPCClient by querying the
// rule-center Kitex service over gRPC.
type RuleCenterClient struct {
	conn    *grpc.ClientConn
	cli     ratelimitv1.RuleServiceClient
	timeout time.Duration
}

// NewRuleCenterClient creates a gRPC client connected to the rule-center
// service at the given address (e.g. "rule-center.internal:8888").
// The connection is lazily established on first call.
func NewRuleCenterClient(address string) (*RuleCenterClient, error) {
	if address == "" {
		return nil, fmt.Errorf("rule_center: address is required")
	}
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("rule_center: dial %s: %w", address, err)
	}
	return &RuleCenterClient{
		conn:    conn,
		cli:     ratelimitv1.NewRuleServiceClient(conn),
		timeout: 200 * time.Millisecond,
	}, nil
}

// NewRuleCenterClientWithTimeout creates a gRPC client with a custom
// per-query timeout.
func NewRuleCenterClientWithTimeout(address string, timeout time.Duration) (*RuleCenterClient, error) {
	c, err := NewRuleCenterClient(address)
	if err != nil {
		return nil, err
	}
	c.timeout = timeout
	return c, nil
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
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	resp, err := c.cli.GetRule(ctx, &ratelimitv1.GetRuleRequest{
		Service: lookup.Service,
		Phase:   lookup.Phase,
		Method:  lookup.Method,
		Path:    lookup.Path,
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
		Enabled:       true,
		Strategy:      rule.GetStrategy(),
		WindowSeconds: int(rule.GetWindowSeconds()),
		MaxRequests:   int(rule.GetMaxRequests()),
		KeyBy:         rule.GetKeyBy(),
		ClientTTLSeconds: int(rule.GetClientTtlSeconds()),
	}
	return cfg, true, nil
}
