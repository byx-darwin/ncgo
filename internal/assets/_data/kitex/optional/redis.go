// Optional Redis add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/data/redis.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.Provide(injector, data.NewRedis)
//
// NewRedis reads cfg.Redis and creates a client via go-middleware/redis.
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common

package data

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	mwredis "github.com/byx-darwin/go-tools/go-middleware/redis"
	"github.com/redis/go-redis/v9"

	"{{.GoModule}}/internal/base/conf"
)

// Redis wraps a go-redis UniversalClient (single / cluster / sentinel).
type Redis struct {
	Client redis.UniversalClient
}

// NewRedis creates a Redis client from conf.RedisConfig via go-middleware/redis,
// validates connectivity, and returns a cleanup function for samber/do.
func NewRedis(ctx context.Context, cfg *conf.RedisConfig) (*Redis, func(), error) {
	if cfg == nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "configuration").
			Code("redis_config_missing").
			Public("redis configuration is invalid").
			New("conf.RedisConfig is nil")
	}
	client, closeFn, err := mwredis.NewUniversalClient(ctx, cfg.ToMiddlewareConfig())
	if err != nil {
		return nil, nil, goerror.
			In("redis").
			Tags("cache", "redis", "connection").
			Code("redis_ping_failed").
			Public("redis is unavailable").
			With("addrs_count", len(cfg.Addrs)).
			Wrapf(err, "redis connect")
	}
	return &Redis{Client: client}, closeFn, nil
}
