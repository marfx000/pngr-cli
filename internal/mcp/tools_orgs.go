package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"pngr.dev/cli/pkg/client"
)

func registerOrgTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("org_list",
			mcp.WithDescription("List organizations the authenticated user belongs to. Returns each org's ID, name, and slug. Call this first to discover org_id values needed by other tools."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgs, err := c.Orgs.List(ctx)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(orgs), nil
		},
	)

	s.AddTool(
		mcp.NewTool("org_members_list",
			mcp.WithDescription("List members of an organization with their roles (owner | admin | member | viewer)."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			members, err := c.Orgs.ListMembers(ctx, orgID)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(members), nil
		},
	)
}

func registerDashboardTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("dashboard_get",
			mcp.WithDescription("Get organization dashboard: monitor counts by status (up/down/paused/pending/total) and the 10 most recent incidents."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			dash, err := c.Dashboard.Get(ctx, orgID)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(dash), nil
		},
	)
}
