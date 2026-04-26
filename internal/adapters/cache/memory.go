package cache

import (
	"sync"

	"caching-proxy/internal/domain"
)

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]*domain.CacheEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]*domain.CacheEntry)}
}

func (s *MemoryStore) Get(key string) (*domain.CacheEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.items[key]
	if !ok {
		return nil, domain.ErrCacheMiss
	}
	return cloneEntry(entry), nil
}

func (s *MemoryStore) Set(key string, entry *domain.CacheEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = cloneEntry(entry)
	return nil
}

func (s *MemoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = make(map[string]*domain.CacheEntry)
	return nil
}

func cloneEntry(entry *domain.CacheEntry) *domain.CacheEntry {
	if entry == nil {
		return nil
	}
	cloned := *entry
	cloned.Headers = cloneHeaders(entry.Headers)
	cloned.Body = cloneBytes(entry.Body)
	return &cloned
}

func cloneHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}
	cloned := make(map[string][]string, len(headers))
	for k, v := range headers {
		cp := make([]string, len(v))
		copy(cp, v)
		cloned[k] = cp
	}
	return cloned
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}
