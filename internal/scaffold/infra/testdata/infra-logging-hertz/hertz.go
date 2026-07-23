// Optional Hertz logging middleware for internal/base/logging.

package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	HeaderRequestID            = "X-Request-ID"
	HeaderTrafficLane          = "X-Traffic-Lane"
	HertzContextKeyRequestID   = "request_id"
	HertzContextKeyTrafficLane = "traffic_lane"
)

func HertzRequestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID, source := hertzRequestID(ctx, c)
		lane := c.Request.Header.Get(HeaderTrafficLane)
		ctx = WithRequestID(ctx, requestID, source)
		if lane != "" {
			ctx = WithTrafficLane(ctx, lane)
			c.Set(HertzContextKeyTrafficLane, lane)
		}
		c.Set(HertzContextKeyRequestID, requestID)
		c.Response.Header.Set(HeaderRequestID, requestID)
		c.Next(ctx)
	}
}

func HertzAccessLog() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		started := time.Now()
		c.Next(ctx)
		status := c.Response.StatusCode()
		if status == 0 {
			status = consts.StatusOK
		}
		L().Info(ctx, CategoryAccess, "hertz access",
			slog.String("http.method", string(c.Method())),
			slog.String("http.path", string(c.Path())),
			slog.Int("http.status_code", status),
			SinceMS(started),
		)
	}
}

func HertzRecovery() app.HandlerFunc {
	return recovery.Recovery(recovery.WithRecoveryHandler(func(ctx context.Context, c *app.RequestContext, recovered interface{}, stack []byte) {
		err := goerror.In("hertz.recovery").
			Tags("panic", "hertz").
			Code("panic_recovered").
			With("panic", fmt.Sprint(recovered)).
			With("stack", string(stack)).
			New("hertz panic recovered")
		L().Error(ctx, CategoryPanic, "hertz panic recovered", err)
		c.Response.SetStatusCode(consts.StatusInternalServerError)
		c.Abort()
	}))
}

func HertzRequestIDFromContext(c *app.RequestContext) string {
	value, ok := c.Get(HertzContextKeyRequestID)
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

func hertzRequestID(ctx context.Context, c *app.RequestContext) (string, string) {
	if requestID := c.Request.Header.Get(HeaderRequestID); requestID != "" {
		return requestID, "header"
	}
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String(), "trace_id"
	}
	return newHertzRequestID(), "generated"
}

func newHertzRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
