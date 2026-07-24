// Optional Redis add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/redis.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.Provide(injector, data.NewRedis)
//
// NewRedis reads cfg.Redis and reuses the same shared UniversalClient used by
// redis-backed middleware.
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common
//	go get github.com/byx-darwin/go-tools/go-framework

package data

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	"github.com/redis/go-redis/v9"
)

// CodeCacheUnavailable is the project-segment error code (>=40100) for cache
// backend unavailability. go-framework has no equivalent code, so a project
// code is defined here and mapped to HTTP 503 at init time.
const CodeCacheUnavailable = 40504

func init() {
	goerror.RegisterHTTPStatuses(map[int]int{CodeCacheUnavailable: 503})
}

// Redis wraps a go-redis UniversalClient (single / cluster / sentinel).
type Redis struct {
	Client redis.UniversalClient
}

// NewRedis reuses the shared Redis client derived from cfg.Redis, validates
// connectivity with the injected startup context, and returns a cleanup
// function for samber/do.
func NewRedis(ctx context.Context, cfg *Config) (*Redis, func(), error) {
	if cfg == nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("data.Config is nil")
	}
	cli := SharedRedisClient(cfg.Redis)
	if cli == nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "connection").
			Code(CodeCacheUnavailable).
			Public("cache_unavailable").
			With("addrs_count", len(cfg.Redis.Addrs)).
			New("redis connection failed")
	}
	if err := cli.Ping(ctx).Err(); err != nil {
		CloseSharedRedisClient(cfg.Redis)
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "connection").
			Code(CodeCacheUnavailable).
			Public("cache_unavailable").
			With("addrs_count", len(cfg.Redis.Addrs)).
			Wrapf(err, "redis.Ping")
	}
	cleanup := func() { CloseSharedRedisClient(cfg.Redis) }
	return &Redis{Client: cli}, cleanup, nil
}
