package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/byx-darwin/ncgo/internal/assets"
	"github.com/byx-darwin/ncgo/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server for AI agents",
	}
	cmd.AddCommand(newMCPServeCmd())
	return cmd
}

func newMCPServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Serve ncgo tools over MCP stdio",
		Long: "Start a minimal MCP stdio server that supports initialize, tools/list, " +
			"and tools/call for selected ncgo operations.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcp.New(Version, assets.Version())
			if err := server.Serve(cmd.Context(), os.Stdin, os.Stdout); err != nil {
				return fmt.Errorf("mcp serve: %w", err)
			}
			return nil
		},
	}
}
