package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"caching-proxy/internal/domain"
)

const redisKeyPrefix = "caching-proxy:"

type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{client: client, ttl: ttl}
}

func (s *RedisStore) Get(key string) (*domain.CacheEntry, error) {
	ctx := context.Background()
	raw, err := s.client.Get(ctx, redisKeyPrefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var entry domain.CacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, fmt.Errorf("decode cache entry: %w", err)
	}
	return &entry, nil
}

func (s *RedisStore) Set(key string, entry *domain.CacheEntry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode cache entry: %w", err)
	}
	if err := s.client.Set(context.Background(), redisKeyPrefix+key, raw, s.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (s *RedisStore) Clear() error {
	ctx := context.Background()
	iter := s.client.Scan(ctx, 0, redisKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		if err := s.client.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("redis del: %w", err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("redis scan: %w", err)
	}
	return nil
}
