package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"caching-proxy/internal/domain"
)

const (
	cacheHeaderName = "X-Cache"
	cacheHit        = "HIT"
	cacheMiss       = "MISS"
)

type OriginClient interface {
	Do(method, path, rawQuery string, headers map[string][]string, body []byte) (*OriginResponse, error)
}

type OriginResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

type Request struct {
	Method   string
	Path     string
	RawQuery string
	Headers  map[string][]string
	Body     []byte
}

type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

type ProxyUseCase struct {
	cache  domain.CacheStore
	origin OriginClient
}

func NewProxyUseCase(cache domain.CacheStore, origin OriginClient) *ProxyUseCase {
	return &ProxyUseCase{
		cache:  cache,
		origin: origin,
	}
}

func (uc *ProxyUseCase) Handle(req Request) (*Response, error) {
	cacheKey := BuildCacheKey(req.Method, req.Path, req.RawQuery)

	if req.Method == "GET" {
		if entry, ok := uc.cache.Get(cacheKey); ok {
			return &Response{
				StatusCode: entry.StatusCode,
				Headers:    withCacheHeader(entry.Headers, cacheHit),
				Body:       cloneBytes(entry.Body),
			}, nil
		}
	}

	originResp, err := uc.origin.Do(req.Method, req.Path, req.RawQuery, req.Headers, req.Body)
	if err != nil {
		return nil, err
	}

	if req.Method == "GET" {
		err = uc.cache.Set(cacheKey, &domain.CacheEntry{
			StatusCode: originResp.StatusCode,
			Headers:    cloneHeaders(originResp.Headers),
			Body:       cloneBytes(originResp.Body),
			CachedAt:   time.Now(),
		})
		if err != nil {
			return nil, err
		}
	}

	return &Response{
		StatusCode: originResp.StatusCode,
		Headers:    withCacheHeader(originResp.Headers, cacheMiss),
		Body:       cloneBytes(originResp.Body),
	}, nil
}

func BuildCacheKey(method, path, rawQuery string) string {
	sum := sha256.Sum256([]byte(method + " " + path + "?" + rawQuery))
	return hex.EncodeToString(sum[:])
}

func withCacheHeader(headers map[string][]string, value string) map[string][]string {
	cloned := cloneHeaders(headers)
	if cloned == nil {
		cloned = make(map[string][]string)
	}
	cloned[cacheHeaderName] = []string{value}
	return cloned
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
