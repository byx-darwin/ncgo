// Optional structured logging add-on for Hertz and Kitex services.
//
// It provides category-based structured logging through go-common/log,
// request/trace context injection, and go-common/error metadata extraction.
//
// Required dependencies:
//
//	go get github.com/byx-darwin/go-tools/go-common

package logging

import (
	"context"
	"log/slog"
	"time"

	goclog "github.com/byx-darwin/go-tools/go-common/log"
)

// Re-export category constants from go-common/log.
const (
	CategoryAccess   = goclog.CategoryAccess
	CategoryError    = goclog.CategoryError
	CategoryBiz      = goclog.CategoryBiz
	CategoryRPC      = goclog.CategoryRPC
	CategoryDB       = goclog.CategoryDB
	CategoryPanic    = goclog.CategoryPanic
	CategoryAudit    = goclog.CategoryAudit
	CategorySecurity = goclog.CategorySecurity
)

type contextKey string

const (
	ContextKeyRequestID       contextKey = "request_id"
	ContextKeyRequestIDSource contextKey = "request_id_source"
	ContextKeyTrafficLane     contextKey = "traffic_lane"
)

func WithRequestID(ctx context.Context, requestID, source string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)
	ctx = context.WithValue(ctx, ContextKeyRequestIDSource, source)
	return goclog.WithRequestID(ctx, requestID)
}

func WithTrafficLane(ctx context.Context, lane string) context.Context {
	return context.WithValue(ctx, ContextKeyTrafficLane, lane)
}

func SinceMS(started time.Time) slog.Attr {
	return slog.Float64("latency_ms", float64(time.Since(started).Microseconds())/1000)
}
