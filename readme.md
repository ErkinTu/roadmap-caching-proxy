# Caching Proxy Server

Minimal Go implementation of the [roadmap.sh Caching Proxy project](https://roadmap.sh/projects/caching-server).

The application starts an HTTP proxy server, forwards incoming requests to an origin server, caches repeated `GET` responses in memory, and marks each response with `X-Cache: MISS` or `X-Cache: HIT`.

## Features

- CLI startup with `--port` and `--origin`.
- Proxies HTTP requests to the configured origin server.
- Caches only `GET` responses for the MVP.
- Stores cache entries in memory only.
- Returns origin status code, headers, and body to the client.
- Adds `X-Cache: MISS` when the response comes from origin.
- Adds `X-Cache: HIT` when the response comes from cache.
- Provides `--clear-cache` command.
- Uses only the Go standard library.

## Requirements

- Go installed.
- No external services are required.
- No third-party Go packages are used.

## Project Structure

```text
.
├── cmd/caching-proxy/main.go
├── go.mod
└── internal
    ├── adapters
    │   ├── cache/memory.go
    │   └── origin/client.go
    ├── delivery/http/handler.go
    ├── domain/cache.go
    └── usecase/proxy.go
```

## Architecture

The project keeps a small clean architecture split:

- `internal/domain` contains cache entities and interfaces.
- `internal/usecase` contains proxy and caching business logic.
- `internal/adapters/cache` contains the in-memory cache implementation.
- `internal/adapters/origin` contains the HTTP origin client.
- `internal/delivery/http` contains the `net/http` handler.
- `cmd/caching-proxy/main.go` wires dependencies and starts the server.

The delivery layer does not decide when to cache. It only reads the incoming HTTP request, passes it to the use case, and writes the returned response.

## Run

Start the proxy on port `3000` and forward requests to `http://dummyjson.com`:

```bash
go run ./cmd/caching-proxy --port 3000 --origin http://dummyjson.com
```

You can also build a binary:

```bash
go build -o caching-proxy ./cmd/caching-proxy
./caching-proxy --port 3000 --origin http://dummyjson.com
```

## Check HIT and MISS

First request:

```bash
curl -i http://localhost:3000/products/1
```

Expected header:

```text
X-Cache: MISS
```

Second identical `GET` request:

```bash
curl -i http://localhost:3000/products/1
```

Expected header:

```text
X-Cache: HIT
```

Requests with different paths or query strings use different cache keys:

```bash
curl -i "http://localhost:3000/products/search?q=phone"
curl -i "http://localhost:3000/products/search?q=phone"
```

## Clear Cache

Run:

```bash
go run ./cmd/caching-proxy --clear-cache
```

Or, if using the built binary:

```bash
./caching-proxy --clear-cache
```

For this MVP the cache is in memory, so stopping the running server also clears all cached data.

## What Is Not Included

The MVP intentionally does not include:

- Redis
- PostgreSQL
- TTL cleaner
- LRU eviction
- metrics
- authentication
- Docker
- third-party routers or frameworks

## Future Extensions

- Redis cache: add another implementation of `domain.CacheStore` in `internal/adapters/cache`.
- Gin delivery layer: add a new HTTP delivery adapter that calls the same use case.
- TTL or LRU: extend the cache adapter without moving cache decisions into the HTTP handler.

## Verification

Run:

```bash
go test ./...
```
