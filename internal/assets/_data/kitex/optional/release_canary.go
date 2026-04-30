// Optional Kitex release-canary adapter for internal/base/release.

package release

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"
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
