package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/cliclient"
	"pngr.dev/cli/internal/mcp"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
}

var mcpTransport string

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server (stdio transport) for AI agents",
	Long: `Run pngr as a Model Context Protocol server.

Spawned as a subprocess by AI agents (Claude Desktop, Cursor, Windsurf) via
their MCP config. Exposes every pngr operation as a typed tool the agent can
discover and call by name.

Auth: reads PNGR_TOKEN from the environment (set it in the agent's MCP
config, NOT in ~/.config/pngr — the agent spawns this as its own process
with its own env). Same for PNGR_API_URL if pointing at a non-default
server.

Example Claude Desktop config (~/Library/Application Support/Claude/claude_desktop_config.json):

  {
    "mcpServers": {
      "pngr": {
        "command": "pngr",
        "args": ["mcp"],
        "env": {
          "PNGR_TOKEN": "your-api-token",
          "PNGR_API_URL": "https://api.pngr.dev/v1"
        }
      }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mcpTransport != "stdio" {
			return fmt.Errorf("transport %q not yet supported (only stdio for now; SSE is planned)", mcpTransport)
		}
		c, err := cliclient.New(globalConfig)
		if err != nil {
			return err
		}
		return mcp.Run(c)
	},
}

func init() {
	mcpCmd.Flags().StringVar(&mcpTransport, "transport", "stdio",
		"transport: stdio (default) | sse (planned)")
}
