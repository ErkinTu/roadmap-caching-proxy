package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "caching-proxy",
		Short: "HTTP caching proxy",
	}
	root.AddCommand(newStartCommand())
	root.AddCommand(newClearCacheCommand())
	return root
}

func Execute() error {
	return NewRootCommand().Execute()
}
