# Структура проекта

```text
cmd
cmd/caching-proxy
cmd/caching-proxy/main.go
internal
internal/adapters
internal/adapters/cache
internal/adapters/cache/memory.go
internal/adapters/origin
internal/adapters/origin/client.go
internal/delivery
internal/delivery/http
internal/delivery/http/handler.go
internal/domain
internal/domain/cache.go
internal/usecase
internal/usecase/proxy.go
```

## Файлы

### cmd/caching-proxy/main.go

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	cacheadapter "caching-proxy/internal/adapters/cache"
	"caching-proxy/internal/adapters/origin"
	deliveryhttp "caching-proxy/internal/delivery/http"
	"caching-proxy/internal/usecase"
)

func main() {
	port := flag.Int("port", 3000, "port to listen on")
	originURL := flag.String("origin", "", "origin server URL")
	clearCache := flag.Bool("clear-cache", false, "clear in-memory cache and exit")
	flag.Parse()

	cacheStore := cacheadapter.NewMemoryStore()

	if *clearCache {
		if err := cacheStore.Clear(); err != nil {
			log.Fatalf("clear cache: %v", err)
		}
		fmt.Println("cache cleared")
		return
	}

	if *originURL == "" {
		fmt.Fprintln(os.Stderr, "origin is required: caching-proxy --port 3000 --origin http://dummyjson.com")
		os.Exit(1)
	}

	originClient, err := origin.NewClient(*originURL, http.DefaultClient)
	if err != nil {
		log.Fatalf("create origin client: %v", err)
	}

	proxyUseCase := usecase.NewProxyUseCase(cacheStore, originClient)
	handler := deliveryhttp.NewHandler(proxyUseCase)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("caching proxy listening on %s, origin %s", addr, *originURL)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

### internal/adapters/cache/memory.go

```go
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
```

### internal/adapters/origin/client.go

```go
package origin

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"caching-proxy/internal/usecase"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(rawBaseURL string, httpClient *http.Client) (*Client, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("origin must include scheme and host")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Do(method, path, rawQuery string, headers map[string][]string, body []byte) (*usecase.OriginResponse, error) {
	targetURL := c.buildURL(path, rawQuery)

	req, err := http.NewRequest(method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = cloneHeaders(headers)
	req.Host = c.baseURL.Host

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &usecase.OriginResponse{
		StatusCode: resp.StatusCode,
		Headers:    cloneHeaders(resp.Header),
		Body:       respBody,
	}, nil
}

func (c *Client) buildURL(path, rawQuery string) string {
	target := *c.baseURL
	target.Path = joinPath(c.baseURL.Path, path)
	target.RawQuery = rawQuery
	return target.String()
}

func joinPath(basePath, requestPath string) string {
	if basePath == "" || basePath == "/" {
		if requestPath == "" {
			return "/"
		}
		return requestPath
	}

	if requestPath == "" || requestPath == "/" {
		return basePath
	}

	return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(requestPath, "/")
}

func cloneHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}

	cloned := make(map[string][]string, len(headers))
	for key, values := range headers {
		copiedValues := make([]string, len(values))
		copy(copiedValues, values)
		cloned[key] = copiedValues
	}
	return cloned
}
```

### internal/delivery/http/handler.go

```go
package http

import (
	"io"
	stdhttp "net/http"

	"caching-proxy/internal/usecase"
)

type Proxy interface {
	Handle(req usecase.Request) (*usecase.Response, error)
}

type Handler struct {
	proxy Proxy
}

func NewHandler(proxy Proxy) *Handler {
	return &Handler{
		proxy: proxy,
	}
}

func (h *Handler) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		stdhttp.Error(w, "read request body", stdhttp.StatusBadRequest)
		return
	}

	resp, err := h.proxy.Handle(usecase.Request{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Headers:  cloneHeaders(r.Header),
		Body:     body,
	})
	if err != nil {
		stdhttp.Error(w, "proxy request failed", stdhttp.StatusBadGateway)
		return
	}

	writeHeaders(w.Header(), resp.Headers)
	w.WriteHeader(resp.StatusCode)

	if _, err := w.Write(resp.Body); err != nil {
		return
	}
}

func writeHeaders(dst stdhttp.Header, src map[string][]string) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func cloneHeaders(headers stdhttp.Header) map[string][]string {
	cloned := make(map[string][]string, len(headers))
	for key, values := range headers {
		copiedValues := make([]string, len(values))
		copy(copiedValues, values)
		cloned[key] = copiedValues
	}
	return cloned
}
```

### internal/domain/cache.go

```go
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
```

### internal/usecase/proxy.go

```go
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
```
