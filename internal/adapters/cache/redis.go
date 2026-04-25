package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"caching-proxy/internal/domain"
)

const redisKeyPrefix = "caching-proxy:"

type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
	ctx    context.Context
}

func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{
		client: client,
		ttl:    ttl,
		ctx:    context.Background(),
	}
}

func (s *RedisStore) Get(key string) (*domain.CacheEntry, bool) {
	raw, err := s.client.Get(s.ctx, redisKeyPrefix+key).Bytes()
	if err != nil {
		return nil, false
	}

	var entry domain.CacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false
	}
	return &entry, true
}

func (s *RedisStore) Set(key string, entry *domain.CacheEntry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.client.Set(s.ctx, redisKeyPrefix+key, raw, s.ttl).Err()
}

func (s *RedisStore) Clear() error {
	iter := s.client.Scan(s.ctx, 0, redisKeyPrefix+"*", 0).Iterator()
	for iter.Next(s.ctx) {
		if err := s.client.Del(s.ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}
