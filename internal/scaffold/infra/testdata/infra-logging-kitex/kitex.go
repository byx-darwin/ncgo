// Optional Kitex logging interceptors for internal/base/logging.

package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
	goclog "github.com/byx-darwin/go-tools/go-common/log"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/x/demo/internal/base/conf"
)

const (
	MetadataRequestID   = "x-request-id"
	MetadataTrafficLane = "x-traffic-lane"
)

// InitFromConf initializes the global logger from kitex conf.LogConfig.
func InitFromConf(cfg conf.LogConfig, release goclog.ReleaseInfo) error {
	gcfg := goclog.Config{
		Level:  cfg.Level,
		Format: cfg.Format,
		Mode:   cfg.Mode,
	}
	return goclog.Init(gcfg, release)
}

func KitexRequestID() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			requestID, source := kitexRequestID(ctx)
			ctx = metainfo.WithPersistentValue(ctx, MetadataRequestID, requestID)
			ctx = WithRequestID(ctx, requestID, source)
			if lane := KitexMetaValue(ctx, MetadataTrafficLane); lane != "" {
				ctx = WithTrafficLane(ctx, lane)
			}
			return next(ctx, req, resp)
		}
	}
}

func KitexAccessLog() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			started := time.Now()
			err := next(ctx, req, resp)
			service, method := kitexRPCNames(ctx)
			latencyMS := float64(time.Since(started).Microseconds()) / 1000
			if err != nil {
				goclog.RPC(ctx).ErrorContext(ctx, "kitex rpc failed", err,
					"rpc.system", "kitex",
					"rpc.service", service,
					"rpc.method", method,
					"latency_ms", latencyMS,
				)
				return err
			}
			goclog.RPC(ctx).InfoContext(ctx, "kitex rpc",
				"rpc.system", "kitex",
				"rpc.service", service,
				"rpc.method", method,
				"latency_ms", latencyMS,
			)
			return nil
		}
	}
}

func KitexRecovery() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = goerror.In("kitex.recovery").
						Tags("panic", "kitex").
						Code("panic_recovered").
						With("panic", fmt.Sprint(recovered)).
						New("kitex panic recovered")
					goclog.L().WithCategory(goclog.CategoryPanic).ErrorContext(ctx, "kitex panic recovered", err)
				}
			}()
			return next(ctx, req, resp)
		}
	}
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

func kitexRequestID(ctx context.Context) (string, string) {
	if requestID := KitexMetaValue(ctx, MetadataRequestID); requestID != "" {
		return requestID, "metadata"
	}
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String(), "trace_id"
	}
	return newKitexRequestID(), "generated"
}

func kitexRPCNames(ctx context.Context) (string, string) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil || ri.Invocation() == nil {
		return "unknown", "unknown"
	}
	return ri.Invocation().ServiceName(), ri.Invocation().MethodName()
}

func newKitexRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
