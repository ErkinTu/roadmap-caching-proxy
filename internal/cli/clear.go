package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"caching-proxy/internal/config"
)

func newClearCacheCommand() *cobra.Command {
	var cacheKind string

	cmd := &cobra.Command{
		Use:   "clear-cache",
		Short: "Clear cached responses",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			store, err := buildCacheStore(cacheKind, cfg)
			if err != nil {
				return err
			}
			if err := store.Clear(); err != nil {
				return fmt.Errorf("clear cache: %w", err)
			}
			cmd.Println("cache cleared")
			return nil
		},
	}

	cmd.Flags().StringVar(&cacheKind, "cache", "memory", "cache backend: memory or redis")
	return cmd
}
