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
	return &MemoryStore{
		items: make(map[string]*domain.CacheEntry),
	}
}

func (s *MemoryStore) Get(key string) (*domain.CacheEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.items[key]
	if !ok {
		return nil, false
	}

	return cloneEntry(entry), true
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

	return &domain.CacheEntry{
		StatusCode: entry.StatusCode,
		Headers:    cloneHeaders(entry.Headers),
		Body:       cloneBytes(entry.Body),
		CachedAt:   entry.CachedAt,
	}
}

func cloneHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}

	cloned := make(map[string][]string, len(headers))
	for key, values := range headers {
		cloned[key] = cloneStringSlice(values)
	}
	return cloned
}

func cloneBytes(values []byte) []byte {
	if values == nil {
		return nil
	}

	cloned := make([]byte, len(values))
	copy(cloned, values)
	return cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
