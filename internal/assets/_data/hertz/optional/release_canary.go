// Optional Hertz release-canary adapter for internal/base/release.

package release

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func HertzTraffic() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ctx = WithTraffic(ctx, TrafficFromHertz(c))
		c.Next(ctx)
	}
}

func TrafficFromHertz(c *app.RequestContext) Traffic {
	traffic := Traffic{
		Lane:     c.Request.Header.Get(HeaderTrafficLane),
		UserID:   c.Request.Header.Get(HeaderUserID),
		TenantID: c.Request.Header.Get(HeaderTenantID),
		Headers:  map[string]string{},
	}
	put(traffic.Headers, HeaderTrafficLane, traffic.Lane)
	put(traffic.Headers, HeaderUserID, traffic.UserID)
	put(traffic.Headers, HeaderTenantID, traffic.TenantID)
	traffic.StickyKey = firstNonEmpty(traffic.UserID, traffic.TenantID, traffic.Lane)
	return traffic
}

func HertzDecision(ctx context.Context, rules RuleSet) Decision {
	return Decide(TrafficFromContext(ctx), rules)
}
