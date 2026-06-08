// Optional Redis add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/redis.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.Provide(injector, data.NewRedis)
//
// NewRedis reads cfg.Redis and reuses the same shared UniversalClient used by
// redis-backed middleware. For a dedicated client, call NewRedisWithOptions.
//
// Required dependency:
//
//	go get github.com/redis/go-redis/v9
//	go get github.com/samber/oops

package data

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/samber/oops"
)

// Redis wraps a go-redis UniversalClient (single / cluster / sentinel).
type Redis struct {
	Client redis.UniversalClient
}

// NewRedis reuses the shared Redis client derived from cfg.Redis, validates
// connectivity with the injected startup context, and returns a cleanup
// function for samber/do.
func NewRedis(ctx context.Context, cfg *Config) (*Redis, func(), error) {
	if cfg == nil {
		return nil, nil, oops.
			In("redis").
			Tags("cache", "redis", "configuration").
			Code(10308).
			Public("config_invalid").
			New("data.Config is nil")
	}
	cli := SharedRedisClient(cfg.Redis)
	if err := cli.Ping(ctx).Err(); err != nil {
		CloseSharedRedisClient(cfg.Redis)
		return nil, nil, oops.
			In("redis").
			Tags("cache", "redis", "connection").
			Code(10304).
			Public("cache_unavailable").
			With("addrs_count", len(cfg.Redis.Addrs)).
			Wrapf(err, "redis.Ping")
	}
	cleanup := func() { CloseSharedRedisClient(cfg.Redis) }
	return &Redis{Client: cli}, cleanup, nil
}

// NewRedisWithOptions creates a dedicated Redis client from raw UniversalOptions.
// Use it only when you intentionally want a different connection pool than cfg.Redis.
func NewRedisWithOptions(ctx context.Context, opts *redis.UniversalOptions) (*Redis, func(), error) {
	if opts == nil {
		return nil, nil, oops.
			In("redis").
			Tags("cache", "redis", "configuration").
			Code(10308).
			Public("config_invalid").
			New("redis.UniversalOptions is nil")
	}
	cli := redis.NewUniversalClient(opts)
	if err := cli.Ping(ctx).Err(); err != nil {
		_ = cli.Close()
		return nil, nil, oops.
			In("redis").
			Tags("cache", "redis", "connection").
			Code(10304).
			Public("cache_unavailable").
			With("addrs_count", len(opts.Addrs)).
			Wrapf(err, "redis.Ping")
	}
	cleanup := func() { _ = cli.Close() }
	return &Redis{Client: cli}, cleanup, nil
}
