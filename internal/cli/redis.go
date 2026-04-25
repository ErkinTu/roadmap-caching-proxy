package cli

import (
	"github.com/redis/go-redis/v9"

	"caching-proxy/internal/config"
)

func newRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       cfg.RedisDB,
	})
}
