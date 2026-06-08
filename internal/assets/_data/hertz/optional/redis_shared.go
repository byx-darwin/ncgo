package data

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"{{.GoModule}}/internal/base/conf"
)

type RedisConfig = conf.RedisConfig
type Config = conf.Config

var (
	sharedRedisClientsMu sync.Mutex
	sharedRedisClients   = map[string]redis.UniversalClient{}
)

func SharedRedisClient(cfg RedisConfig) redis.UniversalClient {
	key := redisClientKey(cfg)
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	if client := sharedRedisClients[key]; client != nil {
		return client
	}
	client := redis.NewUniversalClient(RedisUniversalOptions(cfg))
	sharedRedisClients[key] = client
	return client
}

func CloseSharedRedisClient(cfg RedisConfig) {
	key := redisClientKey(cfg)
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	if client := sharedRedisClients[key]; client != nil {
		_ = client.Close()
		delete(sharedRedisClients, key)
	}
}

func RedisUniversalOptions(cfg RedisConfig) *redis.UniversalOptions {
	return &redis.UniversalOptions{
		Addrs:                 cfg.Addrs,
		ClientName:            cfg.ClientName,
		DB:                    cfg.DB,
		Protocol:              cfg.Protocol,
		Username:              cfg.Username,
		Password:              cfg.Password,
		SentinelUsername:      cfg.SentinelUsername,
		SentinelPassword:      cfg.SentinelPassword,
		MaxRetries:            cfg.MaxRetries,
		MinRetryBackoff:       durationMilliseconds(cfg.MinRetryBackoffMilliseconds),
		MaxRetryBackoff:       durationMilliseconds(cfg.MaxRetryBackoffMilliseconds),
		DialTimeout:           durationSeconds(cfg.DialTimeoutSeconds),
		DialerRetries:         cfg.DialerRetries,
		DialerRetryTimeout:    durationMilliseconds(cfg.DialerRetryTimeoutMilliseconds),
		ReadTimeout:           durationSeconds(cfg.ReadTimeoutSeconds),
		WriteTimeout:          durationSeconds(cfg.WriteTimeoutSeconds),
		ContextTimeoutEnabled: cfg.ContextTimeoutEnabled,
		ReadBufferSize:        cfg.ReadBufferSize,
		WriteBufferSize:       cfg.WriteBufferSize,
		PoolFIFO:              cfg.PoolFIFO,
		PoolSize:              cfg.PoolSize,
		MaxConcurrentDials:    cfg.MaxConcurrentDials,
		PoolTimeout:           durationSeconds(cfg.PoolTimeoutSeconds),
		MinIdleConns:          cfg.MinIdleConns,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxActiveConns:        cfg.MaxActiveConns,
		ConnMaxIdleTime:       durationSeconds(cfg.ConnMaxIdleTimeSeconds),
		ConnMaxLifetime:       durationSeconds(cfg.ConnMaxLifetimeSeconds),
		ConnMaxLifetimeJitter: durationSeconds(cfg.ConnMaxLifetimeJitterSeconds),
		MaxRedirects:          cfg.MaxRedirects,
		ReadOnly:              cfg.ReadOnly,
		RouteByLatency:        cfg.RouteByLatency,
		RouteRandomly:         cfg.RouteRandomly,
		MasterName:            cfg.MasterName,
		DisableIdentity:       cfg.DisableIdentity,
		IdentitySuffix:        cfg.IdentitySuffix,
		UnstableResp3:         cfg.UnstableResp3,
		FailingTimeoutSeconds: cfg.FailingTimeoutSeconds,
	}
}

func CloseSharedRedisClients() {
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	for key, client := range sharedRedisClients {
		_ = client.Close()
		delete(sharedRedisClients, key)
	}
}

func redisClientKey(cfg RedisConfig) string {
	// json.Marshal on a concrete struct with basic types cannot fail.
	payload, _ := json.Marshal(cfg)
	return string(payload)
}

func durationSeconds(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}

func durationMilliseconds(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}
