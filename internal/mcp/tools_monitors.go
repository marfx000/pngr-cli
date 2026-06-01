package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"pngr.dev/cli/pkg/client"
)

func registerMonitorTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("monitor_list",
			mcp.WithDescription("List all monitors in the organization with their current status (up | down | paused | pending), URL, check interval, and last-check timestamp."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			monitors, err := c.Monitors.List(ctx, orgID)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(monitors), nil
		},
	)

	s.AddTool(
		mcp.NewTool("monitor_get",
			mcp.WithDescription("Get a single monitor's full details including its configured check conditions (status_code / latency / body_match)."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID (UUID)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			mid, err := req.RequireString("monitor_id")
			if err != nil {
				return errResult(err), nil
			}
			m, err := c.Monitors.Get(ctx, orgID, mid)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(m), nil
		},
	)

	s.AddTool(
		mcp.NewTool("monitor_create",
			mcp.WithDescription("Create a new URL monitor. The 'conditions' array specifies what passes — each item is {type, config}. Types: 'status_code' with config={expected:\"200\"} or {expected:\"200-299\"}; 'latency' with config={max_ms:2000}; 'body_match' with config={mode:\"contains\"|\"not_contains\"|\"matches_regex\", value:\"...\"}. interval_seconds must be ≥60. Returns the created monitor including its ID."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable label")),
			mcp.WithString("url", mcp.Required(), mcp.Description("URL to monitor (http:// or https://)")),
			mcp.WithNumber("interval_seconds", mcp.Description("Check interval; minimum 60, default 300"), mcp.DefaultNumber(300)),
			mcp.WithNumber("timeout_seconds", mcp.Description("Per-request timeout; max 30, default 10"), mcp.DefaultNumber(10)),
			mcp.WithString("http_version", mcp.Description(`Set "h3" to force HTTP/3 (QUIC). Leave empty to auto-negotiate h1/h2 via ALPN — the right choice unless you specifically want to verify QUIC support.`), mcp.Enum("", "h3"), mcp.DefaultString("")),
			mcp.WithString("method", mcp.Description("HTTP method: GET | HEAD | POST"), mcp.Enum("GET", "HEAD", "POST"), mcp.DefaultString("GET")),
			mcp.WithBoolean("verify_tls", mcp.Description("Verify TLS chain. Default true. Set false for internal services behind private CAs or self-signed certs (HTTPS-only; ignored on http:// monitors)."), mcp.DefaultBool(true)),
			mcp.WithBoolean("follow_redirects", mcp.Description("Follow 3xx redirects up to 10 hops. Default true. Set false to return the 3xx response as-is (useful when asserting a redirect is in place)."), mcp.DefaultBool(true)),
			mcp.WithArray("conditions", mcp.Required(), mcp.Description("Check conditions; ALL must pass for the monitor to be up. Example: [{\"type\":\"status_code\",\"config\":{\"expected\":\"200\"}}]")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			name, err := req.RequireString("name")
			if err != nil {
				return errResult(err), nil
			}
			url, err := req.RequireString("url")
			if err != nil {
				return errResult(err), nil
			}
			interval := int(req.GetFloat("interval_seconds", 300))
			timeout := int(req.GetFloat("timeout_seconds", 10))
			httpVer := req.GetString("http_version", "")
			method := req.GetString("method", "GET")
			verifyTLS := req.GetBool("verify_tls", true)
			followRedirects := req.GetBool("follow_redirects", true)

			conds := decodeConditions(req.GetArguments()["conditions"])

			m, err := c.Monitors.Create(ctx, orgID, client.CreateMonitorRequest{
				Name: name, URL: url, HTTPVersion: httpVer, Method: method,
				IntervalSeconds: interval, TimeoutSeconds: timeout,
				VerifyTLS: &verifyTLS, FollowRedirects: &followRedirects,
				Conditions: conds,
			})
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(m), nil
		},
	)

	s.AddTool(
		mcp.NewTool("monitor_update",
			mcp.WithDescription("Update an existing monitor. Only provided fields are changed. Passing 'conditions' replaces the full condition list."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
			mcp.WithString("name", mcp.Description("New name")),
			mcp.WithString("url", mcp.Description("New URL")),
			mcp.WithNumber("interval_seconds", mcp.Description("New interval (min 60)")),
			mcp.WithNumber("timeout_seconds", mcp.Description("New timeout (max 30)")),
			mcp.WithArray("conditions", mcp.Description("Replacement condition list")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			mid, err := req.RequireString("monitor_id")
			if err != nil {
				return errResult(err), nil
			}

			args := req.GetArguments()
			update := client.UpdateMonitorRequest{}
			if v, ok := args["name"].(string); ok && v != "" {
				update.Name = &v
			}
			if v, ok := args["url"].(string); ok && v != "" {
				update.URL = &v
			}
			if v, ok := args["interval_seconds"].(float64); ok {
				n := int(v)
				update.IntervalSeconds = &n
			}
			if v, ok := args["timeout_seconds"].(float64); ok {
				n := int(v)
				update.TimeoutSeconds = &n
			}
			if _, ok := args["conditions"]; ok {
				update.Conditions = decodeConditions(args["conditions"])
			}

			m, err := c.Monitors.Update(ctx, orgID, mid, update)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(m), nil
		},
	)

	s.AddTool(
		mcp.NewTool("monitor_delete",
			mcp.WithDescription("Soft-delete a monitor. Existing check results and incidents are preserved."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
		),
		simpleMonitorAction(c.Monitors.Delete),
	)

	s.AddTool(
		mcp.NewTool("monitor_pause",
			mcp.WithDescription("Pause checks for a monitor. The scheduler will skip it until resumed."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
		),
		simpleMonitorAction(c.Monitors.Pause),
	)

	s.AddTool(
		mcp.NewTool("monitor_resume",
			mcp.WithDescription("Resume a paused monitor. Status goes to 'pending'; next check fires almost immediately."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
		),
		simpleMonitorAction(c.Monitors.Resume),
	)

	s.AddTool(
		mcp.NewTool("monitor_check",
			mcp.WithDescription("Trigger an immediate check and return the result synchronously. The result is also persisted asynchronously, so subsequent monitor_results calls will see it."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			mid, err := req.RequireString("monitor_id")
			if err != nil {
				return errResult(err), nil
			}
			result, err := c.Monitors.Check(ctx, orgID, mid)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(result), nil
		},
	)

	s.AddTool(
		mcp.NewTool("monitor_results",
			mcp.WithDescription("List recent check results for a monitor (newest first). Each result has passed/failed status, latency, status code, and any failure reasons."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
			mcp.WithNumber("limit", mcp.Description("Max results to return (default 20, max 200)"), mcp.DefaultNumber(20)),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			mid, err := req.RequireString("monitor_id")
			if err != nil {
				return errResult(err), nil
			}
			limit := int(req.GetFloat("limit", 20))
			results, err := c.Monitors.ListResults(ctx, orgID, mid, limit, 0)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(results), nil
		},
	)

	s.AddTool(
		mcp.NewTool("monitor_uptime",
			mcp.WithDescription("Get uptime percentage over the requested time range. Useful for SLA reporting."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Required(), mcp.Description("Monitor ID")),
			mcp.WithString("range", mcp.Description("Time range: 24h | 7d | 30d"), mcp.Enum("24h", "7d", "30d"), mcp.DefaultString("24h")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			mid, err := req.RequireString("monitor_id")
			if err != nil {
				return errResult(err), nil
			}
			rng := req.GetString("range", "24h")
			stats, err := c.Monitors.Uptime(ctx, orgID, mid, rng)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(stats), nil
		},
	)
}

// simpleMonitorAction wraps a (ctx, orgID, monitorID) → error client call as
// an MCP tool handler. Used for pause/resume/delete which share the same
// param shape and return only a success acknowledgement.
func simpleMonitorAction(action func(context.Context, string, string) error) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		orgID, errRes := requireOrgID(req)
		if errRes != nil {
			return errRes, nil
		}
		mid, err := req.RequireString("monitor_id")
		if err != nil {
			return errResult(err), nil
		}
		if err := action(ctx, orgID, mid); err != nil {
			return errResult(err), nil
		}
		return mcp.NewToolResultText(`{"ok": true}`), nil
	}
}

// decodeConditions accepts the raw 'conditions' argument (an arbitrary JSON
// array per the tool schema) and coerces it into the typed client struct.
// Tolerant of missing/malformed entries — the server-side check_conditions
// type constraint will reject anything truly bad.
func decodeConditions(raw any) []client.CheckCondition {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]client.CheckCondition, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		cfg, _ := m["config"].(map[string]any)
		out = append(out, client.CheckCondition{Type: t, Config: cfg})
	}
	return out
}
