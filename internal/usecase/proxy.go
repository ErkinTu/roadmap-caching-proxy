package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"caching-proxy/internal/domain"
)

const (
	cacheHeader = "X-Cache"
	cacheHit    = "HIT"
	cacheMiss   = "MISS"
)

// ErrOrigin wraps any failure while talking to the upstream origin.
var ErrOrigin = errors.New("origin request failed")

// ProxyRequest is the inbound DTO consumed by the use case.
type ProxyRequest struct {
	Method   string
	Path     string
	RawQuery string
	Headers  map[string][]string
	Body     []byte
}

// ProxyResponse is the outbound DTO returned to the delivery layer.
type ProxyResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

// OriginResponse is the DTO returned by an OriginClient implementation.
type OriginResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

type OriginClient interface {
	Do(method, path, rawQuery string, headers map[string][]string, body []byte) (*OriginResponse, error)
}

type ProxyUseCase struct {
	cache  domain.CacheStore
	origin OriginClient
}

func NewProxyUseCase(cache domain.CacheStore, origin OriginClient) *ProxyUseCase {
	return &ProxyUseCase{cache: cache, origin: origin}
}

func (uc *ProxyUseCase) Handle(req ProxyRequest) (*ProxyResponse, error) {
	key := BuildCacheKey(req.Method, req.Path, req.RawQuery)

	if req.Method == "GET" {
		entry, err := uc.cache.Get(key)
		switch {
		case err == nil:
			return responseFromEntry(entry, cacheHit), nil
		case errors.Is(err, domain.ErrCacheMiss):
			// fall through to origin
		default:
			return nil, fmt.Errorf("cache lookup: %w", err)
		}
	}

	originResp, err := uc.origin.Do(req.Method, req.Path, req.RawQuery, req.Headers, req.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOrigin, err)
	}

	if req.Method == "GET" {
		entry := &domain.CacheEntry{
			StatusCode: originResp.StatusCode,
			Headers:    cloneHeaders(originResp.Headers),
			Body:       cloneBytes(originResp.Body),
			CachedAt:   time.Now(),
		}
		if err := uc.cache.Set(key, entry); err != nil {
			// Cache writes are best-effort: log and continue.
			log.Printf("cache write failed: %v", err)
		}
	}

	return &ProxyResponse{
		StatusCode: originResp.StatusCode,
		Headers:    withCacheHeader(originResp.Headers, cacheMiss),
		Body:       cloneBytes(originResp.Body),
	}, nil
}

func BuildCacheKey(method, path, rawQuery string) string {
	sum := sha256.Sum256([]byte(method + " " + path + "?" + rawQuery))
	return hex.EncodeToString(sum[:])
}

func responseFromEntry(entry *domain.CacheEntry, status string) *ProxyResponse {
	return &ProxyResponse{
		StatusCode: entry.StatusCode,
		Headers:    withCacheHeader(entry.Headers, status),
		Body:       cloneBytes(entry.Body),
	}
}

func withCacheHeader(headers map[string][]string, value string) map[string][]string {
	cloned := cloneHeaders(headers)
	if cloned == nil {
		cloned = make(map[string][]string)
	}
	cloned[cacheHeader] = []string{value}
	return cloned
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
