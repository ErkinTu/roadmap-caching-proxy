package cli

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	cacheadapter "caching-proxy/internal/adapters/cache"
	"caching-proxy/internal/adapters/origin"
	"caching-proxy/internal/config"
	deliveryhttp "caching-proxy/internal/delivery/http"
	"caching-proxy/internal/domain"
	"caching-proxy/internal/usecase"
)

func newStartCommand() *cobra.Command {
	var (
		port      int
		originURL string
		cacheKind string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the caching proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if originURL == "" {
				return fmt.Errorf("--origin is required")
			}

			cfg := config.Load()

			store, err := buildCacheStore(cacheKind, cfg)
			if err != nil {
				return err
			}

			originClient, err := origin.NewClient(originURL, http.DefaultClient)
			if err != nil {
				return fmt.Errorf("create origin client: %w", err)
			}

			proxy := usecase.NewProxyUseCase(store, originClient)
			engine := deliveryhttp.NewGinEngine(proxy)

			addr := fmt.Sprintf(":%d", port)
			cmd.Printf("caching proxy listening on %s, origin %s, cache %s\n", addr, originURL, cacheKind)
			return engine.Run(addr)
		},
	}

	cmd.Flags().IntVar(&port, "port", 3000, "port to listen on")
	cmd.Flags().StringVar(&originURL, "origin", "", "origin server URL")
	cmd.Flags().StringVar(&cacheKind, "cache", "memory", "cache backend: memory or redis")
	return cmd
}

func buildCacheStore(kind string, cfg *config.Config) (domain.CacheStore, error) {
	switch kind {
	case "memory", "":
		return cacheadapter.NewMemoryStore(), nil
	case "redis":
		return cacheadapter.NewRedisStore(newRedisClient(cfg), cfg.CacheTTL), nil
	default:
		return nil, fmt.Errorf("unknown cache backend %q", kind)
	}
}
