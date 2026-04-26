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
