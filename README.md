# Caching Proxy Server

Go implementation of the [roadmap.sh Caching Proxy project](https://roadmap.sh/projects/caching-server).

The application starts an HTTP proxy server, forwards incoming requests to an
origin server, caches repeated `GET` responses, and marks each response with
`X-Cache: HIT` or `X-Cache: MISS`.

## Features

- Roadmap-compatible CLI:
  - `caching-proxy --port <number> --origin <url>`
  - `caching-proxy --clear-cache`
- Equivalent [cobra](https://github.com/spf13/cobra)-based `start` and
  `clear-cache` subcommands for the same operations.
- Two interchangeable HTTP delivery adapters:
  - `gin` — routing via [gin-gonic/gin](https://github.com/gin-gonic/gin)
    (default).
  - `stdlib` — plain `net/http` handler.
- Two interchangeable cache backends:
  - `memory` — in-process map (default).
  - `redis` — [go-redis/redis](https://github.com/redis/go-redis) with TTL
    from configuration.
- Forwards method, headers, query, and body to the configured origin and
  returns the upstream status, headers, and body unchanged.
- Adds `X-Cache: MISS` for origin-served responses and `X-Cache: HIT` for
  cache-served ones.
- Maps internal errors to appropriate HTTP status codes
  (`502 Bad Gateway` on origin failures, `500` otherwise).
- Configuration via environment variables (`.env`).

## Requirements

- Go 1.25+
- A reachable origin URL
- Redis (only if `--cache redis` is used)

## Installation

```bash
go build -o caching-proxy ./cmd/caching-proxy
```

## Usage

### Start the proxy

```bash
caching-proxy --port 3000 --origin http://dummyjson.com
```

Flags:

| Flag       | Default      | Description                                   |
|------------|--------------|-----------------------------------------------|
| `--port`   | `3000`       | Port the proxy listens on                     |
| `--origin` | _(required)_ | Origin server URL                             |
| `--cache`  | `memory`     | Cache backend: `memory` or `redis`            |
| `--server` | `gin`        | HTTP server implementation: `gin` or `stdlib` |

Equivalent subcommand form:

```bash
caching-proxy start --port 3000 --origin http://dummyjson.com
```

Examples:

```bash
# Plain in-memory cache, gin server (default, roadmap-compatible form).
caching-proxy --port 3000 --origin http://dummyjson.com

# Redis cache, gin server.
caching-proxy --port 3000 --origin http://dummyjson.com --cache redis

# In-memory cache, stdlib server.
caching-proxy start --port 3000 --origin http://dummyjson.com --server stdlib
```

### Clear the cache

```bash
caching-proxy --clear-cache --cache redis
```

Equivalent subcommand form:

```bash
caching-proxy clear-cache --cache redis
```

For the Redis backend, `--clear-cache` removes stored entries. For the
in-memory backend, the cache lives inside the running process, so restarting
the server clears it.

### Running from source

```bash
go run ./cmd/caching-proxy --port 3000 --origin http://dummyjson.com
```

## Configuration

Redis settings are loaded from environment variables (or `.env`):

| Variable     | Default          | Description               |
|--------------|------------------|---------------------------|
| `REDIS_ADDR` | `127.0.0.1:6379` | Redis address `host:port` |
| `REDIS_PASS` | _(empty)_        | Redis password            |
| `REDIS_DB`   | `0`              | Redis logical DB number   |
| `CACHE_TTL`  | `3600`           | Cache TTL in seconds      |

## Verifying HIT and MISS

First request:

```bash
curl -i http://localhost:3000/products/1
```

Response header:

```text
X-Cache: MISS
```

Second identical `GET` request:

```bash
curl -i http://localhost:3000/products/1
```

Response header:

```text
X-Cache: HIT
```

Requests with different paths or query strings use different cache keys:

```bash
curl -i "http://localhost:3000/products/search?q=phone"
curl -i "http://localhost:3000/products/search?q=phone"
```

## Requirement mapping

The implementation matches the core roadmap.sh requirements:

- Start the proxy with top-level flags:
  `caching-proxy --port <number> --origin <url>`.
- Forward requests to the configured origin and return upstream status,
  headers, and body.
- Cache repeated requests and mark responses with `X-Cache: MISS` and
  `X-Cache: HIT`.
- Clear Redis-backed cache with `caching-proxy --clear-cache`.

It also adds optional extras beyond the roadmap scope: Redis support, an
alternate `stdlib` server, environment-based configuration, and equivalent
`start` / `clear-cache` subcommands.

## Architecture

The project follows a small clean-architecture split:

- `internal/domain` — entities (`CacheEntry`), the `CacheStore` port, and
  domain-level sentinel errors (`ErrCacheMiss`).
- `internal/usecase` — proxy business logic, DTOs (`ProxyRequest`,
  `ProxyResponse`, `OriginResponse`), and the `ErrOrigin` sentinel.
- `internal/adapters/cache` — `memory` and `redis` implementations of
  `CacheStore`.
- `internal/adapters/origin` — HTTP origin client.
- `internal/delivery/http` — both `gin` and `stdlib` HTTP delivery adapters
  in a single file. They share request/response mapping and a single
  error-to-status function.
- `internal/cli` — roadmap-compatible top-level flags plus equivalent cobra
  subcommands wiring everything together.
- `internal/config` — environment-based configuration.
- `cmd/caching-proxy/main.go` — entry point that hands control to
  `cli.Execute()`.

The delivery layer never decides when to cache. It only adapts the HTTP
request to a `ProxyRequest`, calls the use case, and writes the resulting
`ProxyResponse` back.

## Project structure

See [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) for the full file tree and
source listing.

## Verification

```bash
go build ./...
go vet ./...
```
