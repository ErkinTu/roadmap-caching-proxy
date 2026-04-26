package domain

import (
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

type CacheEntry struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	CachedAt   time.Time
}

type CacheStore interface {
	Get(key string) (*CacheEntry, error)
	Set(key string, entry *CacheEntry) error
	Clear() error
}
