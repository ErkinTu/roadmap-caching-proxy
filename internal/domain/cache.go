package domain

import "time"

type CacheEntry struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	CachedAt   time.Time
}

type CacheStore interface {
	Get(key string) (*CacheEntry, bool)
	Set(key string, entry *CacheEntry) error
	Clear() error
}
