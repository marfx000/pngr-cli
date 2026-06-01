// Package mcp implements the pngr Model Context Protocol server.
//
// Spawned by `pngr mcp` as a long-running stdio subprocess for AI agents
// (Claude Desktop, Cursor, Windsurf, etc). Every tool is a thin adapter over
// /pkg/client — same call sites as the CLI, different presentation layer.
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"pngr.dev/cli/pkg/client"
)

const (
	serverName    = "pngr"
	serverVersion = "0.1"
)

// Run builds the full tool set on top of c and serves it over stdio. Blocks
// until the parent process closes stdin (Claude Desktop sends EOF on quit).
func Run(c *client.Client) error {
	s := server.NewMCPServer(serverName, serverVersion)
	registerOrgTools(s, c)
	registerMonitorTools(s, c)
	registerIncidentTools(s, c)
	registerChannelTools(s, c)
	registerRuleTools(s, c)
	registerDashboardTools(s, c)
	registerStackTools(s, c)
	return server.ServeStdio(s)
}

// jsonResult marshals v as pretty JSON and wraps it as a successful tool result.
// LLMs parse JSON natively; pretty-printing helps when a human watches the
// MCP message log.
func jsonResult(v any) *mcp.CallToolResult {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal: %v", err))
	}
	return mcp.NewToolResultText(string(data))
}

// errResult formats client errors with the API error code first (the most
// LLM-actionable bit), then the human message.
func errResult(err error) *mcp.CallToolResult {
	var apiErr *client.APIError
	if asAPIErr(err, &apiErr) {
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", apiErr.Code, apiErr.Message))
	}
	return mcp.NewToolResultError(err.Error())
}

// asAPIErr is a tiny errors.As shim that avoids importing "errors" everywhere.
func asAPIErr(err error, target **client.APIError) bool {
	for err != nil {
		if e, ok := err.(*client.APIError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// requireOrgID extracts org_id with a friendly error message naming the param.
// Most tools start with this so we factor it out.
func requireOrgID(req mcp.CallToolRequest) (string, *mcp.CallToolResult) {
	id, err := req.RequireString("org_id")
	if err != nil {
		return "", mcp.NewToolResultError("org_id is required (UUID; call org_list to discover)")
	}
	return id, nil
}
