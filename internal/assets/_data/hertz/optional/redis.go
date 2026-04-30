// Optional Redis add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/redis.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, &redis.UniversalOptions{
//	    Addrs:           []string{"redis-1:6379", "redis-2:6379"},
//	    DB:              0,
//	    Username:        "default",
//	    Password:        "secret",
//	    MasterName:      "",  // sentinel mode
//	    SentinelAddrs:   nil, // sentinel mode
//	    DialTimeout:     5 * time.Second,
//	    ReadTimeout:     3 * time.Second,
//	    WriteTimeout:    3 * time.Second,
//	    PoolSize:        10,
//	    MinIdleConns:    2,
//	    PoolTimeout:     4 * time.Second,
//	    ConnMaxIdleTime: 5 * time.Minute,
//	    ConnMaxLifetime: 30 * time.Minute,
//	    MaxRetries:      3,
//	    MinRetryBackoff: 8 * time.Millisecond,
//	    MaxRetryBackoff: 512 * time.Millisecond,
//	    TLSConfig:       nil, // *tls.Config
//	    Protocol:        3,
//	})
//	do.Provide(injector, data.NewRedis)
//
// UniversalOptions auto-selects between single / cluster / sentinel based on
// the fields you set. See: https://pkg.go.dev/github.com/redis/go-redis/v9#UniversalOptions
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

// NewRedis creates a Redis client from the full UniversalOptions struct,
// validates connectivity with the injected startup context, and returns a cleanup function for samber/do.
func NewRedis(ctx context.Context, opts *redis.UniversalOptions) (*Redis, func(), error) {
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
