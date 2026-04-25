package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisAddr string
	RedisPass string
	RedisDB   int
	CacheTTL  time.Duration
}

func Load() *Config {
	return &Config{
		RedisAddr: getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPass: getEnv("REDIS_PASS", ""),
		RedisDB:   getEnvAsInt("REDIS_DB", 0),
		CacheTTL:  time.Duration(getEnvAsInt("CACHE_TTL", 3600)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
