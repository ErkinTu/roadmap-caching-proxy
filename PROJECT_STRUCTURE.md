# Структура проекта

```text
cmd
cmd/caching-proxy
cmd/caching-proxy/main.go
internal
internal/adapters
internal/adapters/cache
internal/adapters/cache/memory.go
internal/adapters/cache/redis.go
internal/adapters/origin
internal/adapters/origin/client.go
internal/cli
internal/cli/cli.go
internal/config
internal/config/config.go
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
	"log"

	"caching-proxy/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
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
```

### internal/adapters/cache/redis.go

```go
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
```

### internal/adapters/origin/client.go

```go
package origin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"caching-proxy/internal/usecase"
)

var ErrInvalidOrigin = errors.New("invalid origin URL")

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(rawBaseURL string, httpClient *http.Client) (*Client, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOrigin, err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("%w: must include scheme and host", ErrInvalidOrigin)
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{baseURL: baseURL, httpClient: httpClient}, nil
}

func (c *Client) Do(method, path, rawQuery string, headers map[string][]string, body []byte) (*usecase.OriginResponse, error) {
	target := c.buildURL(path, rawQuery)

	req, err := http.NewRequest(method, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build origin request: %w", err)
	}
	req.Header = cloneHeaders(headers)
	req.Host = c.baseURL.Host

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call origin: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read origin body: %w", err)
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
	for k, v := range headers {
		cp := make([]string, len(v))
		copy(cp, v)
		cloned[k] = cp
	}
	return cloned
}
```

### internal/cli/cli.go

```go
package cli

import (
	"fmt"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"

	cacheadapter "caching-proxy/internal/adapters/cache"
	"caching-proxy/internal/adapters/origin"
	"caching-proxy/internal/config"
	deliveryhttp "caching-proxy/internal/delivery/http"
	"caching-proxy/internal/domain"
	"caching-proxy/internal/usecase"
)

const (
	cacheMemory = "memory"
	cacheRedis  = "redis"

	serverGin    = "gin"
	serverStdlib = "stdlib"
)

type startOptions struct {
	port      int
	originURL string
	cacheKind string
	server    string
}

func Execute() error {
	var (
		opts       startOptions
		clearCache bool
	)

	root := &cobra.Command{
		Use:          "caching-proxy",
		Short:        "HTTP caching proxy",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if clearCache {
				if cmd.Flags().Changed("port") || cmd.Flags().Changed("origin") || cmd.Flags().Changed("server") {
					return fmt.Errorf("--clear-cache cannot be combined with --port, --origin, or --server")
				}
				return runClearCache(cmd, opts.cacheKind)
			}

			if hasStartFlags(cmd) {
				return runStart(cmd, opts)
			}

			return cmd.Help()
		},
	}

	addStartFlags(root, &opts)
	root.Flags().BoolVar(&clearCache, "clear-cache", false, "clear cached responses")
	root.AddCommand(startCommand(), clearCacheCommand())
	return root.Execute()
}

func startCommand() *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start the caching proxy server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd, opts)
		},
	}

	addStartFlags(cmd, &opts)
	return cmd
}

func clearCacheCommand() *cobra.Command {
	var cacheKind string

	cmd := &cobra.Command{
		Use:          "clear-cache",
		Short:        "Clear cached responses",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClearCache(cmd, cacheKind)
		},
	}

	cmd.Flags().StringVar(&cacheKind, "cache", cacheMemory, "cache backend: memory or redis")
	return cmd
}

func addStartFlags(cmd *cobra.Command, opts *startOptions) {
	cmd.Flags().IntVar(&opts.port, "port", 3000, "port to listen on")
	cmd.Flags().StringVar(&opts.originURL, "origin", "", "origin server URL")
	cmd.Flags().StringVar(&opts.cacheKind, "cache", cacheMemory, "cache backend: memory or redis")
	cmd.Flags().StringVar(&opts.server, "server", serverGin, "http server: gin or stdlib")
}

func hasStartFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("port") ||
		cmd.Flags().Changed("origin") ||
		cmd.Flags().Changed("cache") ||
		cmd.Flags().Changed("server")
}

func runStart(cmd *cobra.Command, opts startOptions) error {
	if opts.originURL == "" {
		return fmt.Errorf("--origin is required")
	}

	cfg := config.Load()

	store, err := buildCacheStore(opts.cacheKind, cfg)
	if err != nil {
		return err
	}

	originClient, err := origin.NewClient(opts.originURL, http.DefaultClient)
	if err != nil {
		return fmt.Errorf("create origin client: %w", err)
	}

	proxy := usecase.NewProxyUseCase(store, originClient)
	addr := fmt.Sprintf(":%d", opts.port)
	cmd.Printf("caching proxy listening on %s, origin %s, cache %s, server %s\n", addr, opts.originURL, opts.cacheKind, opts.server)

	switch opts.server {
	case serverGin, "":
		return deliveryhttp.NewGinEngine(proxy).Run(addr)
	case serverStdlib:
		return http.ListenAndServe(addr, deliveryhttp.NewStdHandler(proxy))
	default:
		return fmt.Errorf("unknown server %q", opts.server)
	}
}

func runClearCache(cmd *cobra.Command, cacheKind string) error {
	switch cacheKind {
	case "", cacheMemory:
		cmd.Println("memory cache is process-local; restart the server to clear it")
		return nil
	case cacheRedis:
		store, err := buildCacheStore(cacheKind, config.Load())
		if err != nil {
			return err
		}
		if err := store.Clear(); err != nil {
			return fmt.Errorf("clear cache: %w", err)
		}
		cmd.Println("cache cleared")
		return nil
	default:
		return fmt.Errorf("unknown cache backend %q", cacheKind)
	}
}

func buildCacheStore(kind string, cfg *config.Config) (domain.CacheStore, error) {
	switch kind {
	case cacheMemory, "":
		return cacheadapter.NewMemoryStore(), nil
	case cacheRedis:
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPass,
			DB:       cfg.RedisDB,
		})
		return cacheadapter.NewRedisStore(client, cfg.CacheTTL), nil
	default:
		return nil, fmt.Errorf("unknown cache backend %q", kind)
	}
}
```

### internal/config/config.go

```go
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
```

### internal/delivery/http/handler.go

```go
package http

import (
	"errors"
	"io"
	stdhttp "net/http"

	"github.com/gin-gonic/gin"

	"caching-proxy/internal/usecase"
)

// Proxy is the use-case contract the delivery layer depends on.
type Proxy interface {
	Handle(req usecase.ProxyRequest) (*usecase.ProxyResponse, error)
}

// NewStdHandler returns a stdlib http.Handler that proxies through the use case.
func NewStdHandler(proxy Proxy) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			stdhttp.Error(w, "read request body", stdhttp.StatusBadRequest)
			return
		}

		resp, err := proxy.Handle(buildProxyRequest(r, body))
		if err != nil {
			status, msg := errorToStatus(err)
			stdhttp.Error(w, msg, status)
			return
		}

		writeProxyResponse(w, resp)
	})
}

// NewGinEngine returns a gin engine that routes every request through the use case.
func NewGinEngine(proxy Proxy) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.NoRoute(func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(stdhttp.StatusBadRequest, "read request body")
			return
		}

		resp, err := proxy.Handle(buildProxyRequest(c.Request, body))
		if err != nil {
			status, msg := errorToStatus(err)
			c.String(status, msg)
			return
		}

		writeProxyResponse(c.Writer, resp)
	})
	return engine
}

func buildProxyRequest(r *stdhttp.Request, body []byte) usecase.ProxyRequest {
	return usecase.ProxyRequest{
		Method:   r.Method,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
		Headers:  cloneHeaders(r.Header),
		Body:     body,
	}
}

func writeProxyResponse(w stdhttp.ResponseWriter, resp *usecase.ProxyResponse) {
	for k, vs := range resp.Headers {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func errorToStatus(err error) (int, string) {
	if errors.Is(err, usecase.ErrOrigin) {
		return stdhttp.StatusBadGateway, "origin request failed"
	}
	return stdhttp.StatusInternalServerError, "proxy error"
}

func cloneHeaders(headers stdhttp.Header) map[string][]string {
	cloned := make(map[string][]string, len(headers))
	for k, v := range headers {
		cp := make([]string, len(v))
		copy(cp, v)
		cloned[k] = cp
	}
	return cloned
}
```

### internal/domain/cache.go

```go
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
```

### internal/usecase/proxy.go

```go
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
```
