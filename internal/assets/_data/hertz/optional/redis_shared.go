package data

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"

	mwredis "github.com/byx-darwin/go-tools/go-middleware/redis"

	"{{.GoModule}}/internal/base/conf"
)

type RedisConfig = conf.RedisConfig
type Config = conf.Config

var (
	sharedRedisClientsMu sync.Mutex
	sharedRedisClients   = map[string]redis.UniversalClient{}
	sharedRedisCloseFns  = map[string]func(){}
)

func SharedRedisClient(cfg RedisConfig) redis.UniversalClient {
	key := redisClientKey(cfg)
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	if client := sharedRedisClients[key]; client != nil {
		return client
	}
	client, closeFn, err := mwredis.NewUniversalClient(context.Background(), cfg.ToMiddlewareConfig())
	if err != nil {
		return nil
	}
	sharedRedisClients[key] = client
	sharedRedisCloseFns[key] = closeFn
	return client
}

func CloseSharedRedisClient(cfg RedisConfig) {
	key := redisClientKey(cfg)
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	if closeFn := sharedRedisCloseFns[key]; closeFn != nil {
		closeFn()
		delete(sharedRedisCloseFns, key)
	}
	delete(sharedRedisClients, key)
}

func CloseSharedRedisClients() {
	sharedRedisClientsMu.Lock()
	defer sharedRedisClientsMu.Unlock()
	for key, closeFn := range sharedRedisCloseFns {
		if closeFn != nil {
			closeFn()
		}
		delete(sharedRedisClients, key)
		delete(sharedRedisCloseFns, key)
	}
}

func redisClientKey(cfg RedisConfig) string {
	// json.Marshal on a concrete struct with basic types cannot fail.
	payload, _ := json.Marshal(cfg)
	return string(payload)
}
