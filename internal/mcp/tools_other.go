package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"pngr.dev/cli/pkg/client"
)

// ─── incidents ────────────────────────────────────────────────────────────────

func registerIncidentTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("incident_list",
			mcp.WithDescription("List recent incidents across the org. Optionally filter to a single monitor with monitor_id."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("monitor_id", mcp.Description("Optional: only return incidents for this monitor")),
			mcp.WithNumber("limit", mcp.Description("Max incidents to return (default 20)"), mcp.DefaultNumber(20)),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			limit := int(req.GetFloat("limit", 20))
			if mid := req.GetString("monitor_id", ""); mid != "" {
				incidents, err := c.Incidents.ListByMonitor(ctx, orgID, mid, limit, 0)
				if err != nil {
					return errResult(err), nil
				}
				return jsonResult(incidents), nil
			}
			incidents, err := c.Incidents.ListByOrg(ctx, orgID, limit, 0)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(incidents), nil
		},
	)

	s.AddTool(
		mcp.NewTool("incident_get",
			mcp.WithDescription("Get a single incident's full details: when it started, when it ended (if resolved), duration, and the resolved flag."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("incident_id", mcp.Required(), mcp.Description("Incident ID (UUID)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			iid, err := req.RequireString("incident_id")
			if err != nil {
				return errResult(err), nil
			}
			inc, err := c.Incidents.Get(ctx, orgID, iid)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(inc), nil
		},
	)
}

// ─── channels ─────────────────────────────────────────────────────────────────

func registerChannelTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("channel_list",
			mcp.WithDescription("List notification channels in the organization. Channel config (webhook URLs / tokens) is never returned in responses — it's write-only for security."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			channels, err := c.Channels.List(ctx, orgID)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(channels), nil
		},
	)

	s.AddTool(
		mcp.NewTool("channel_create",
			mcp.WithDescription("Create a notification channel. The 'config' object shape depends on type: slack/ms_teams require {webhook_url:\"...\"}; telegram requires {bot_token:\"...\", chat_id:\"...\"}; email requires {address:\"...\"}; webhook requires {url:\"...\"}; sms requires {to:\"+15551234567\"} (messaging provider configured server-side); pagerduty requires {routing_key:\"...\"} (Events API v2 integration key, trigger/resolve lifecycle); opsgenie requires {api_key:\"...\", region:\"us\"|\"eu\"} (open/close lifecycle). Config is encrypted at rest server-side."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable label")),
			mcp.WithString("type", mcp.Required(), mcp.Description("Channel type"), mcp.Enum("slack", "ms_teams", "telegram", "email", "webhook", "sms", "pagerduty", "opsgenie")),
			mcp.WithObject("config", mcp.Required(), mcp.Description("Type-specific credentials object")),
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
			typ, err := req.RequireString("type")
			if err != nil {
				return errResult(err), nil
			}
			cfg, _ := req.GetArguments()["config"].(map[string]any)
			if cfg == nil {
				return mcp.NewToolResultError("config is required and must be a JSON object"), nil
			}
			ch, err := c.Channels.Create(ctx, orgID, client.CreateChannelRequest{
				Name: name, Type: typ, Config: cfg,
			})
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(ch), nil
		},
	)

	s.AddTool(
		mcp.NewTool("channel_test",
			mcp.WithDescription("Send a sample test notification through the channel. Returns success or an error describing the dispatch failure (e.g. bad webhook URL)."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("channel_id", mcp.Required(), mcp.Description("Channel ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			cid, err := req.RequireString("channel_id")
			if err != nil {
				return errResult(err), nil
			}
			if err := c.Channels.Test(ctx, orgID, cid); err != nil {
				return errResult(err), nil
			}
			return mcp.NewToolResultText(`{"ok": true}`), nil
		},
	)

	s.AddTool(
		mcp.NewTool("channel_delete",
			mcp.WithDescription("Delete a notification channel. Cascades: any rules that referenced this channel lose the association."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("channel_id", mcp.Required(), mcp.Description("Channel ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			cid, err := req.RequireString("channel_id")
			if err != nil {
				return errResult(err), nil
			}
			if err := c.Channels.Delete(ctx, orgID, cid); err != nil {
				return errResult(err), nil
			}
			return mcp.NewToolResultText(`{"ok": true}`), nil
		},
	)
}

// ─── rules ────────────────────────────────────────────────────────────────────

func registerRuleTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("rule_list",
			mcp.WithDescription("List alerting rules in the organization. Each rule has a triggers array (subset of on_warning | on_failure | on_recovery), a failure_threshold, and a list of channels to notify."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			rules, err := c.Rules.List(ctx, orgID)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(rules), nil
		},
	)

	s.AddTool(
		mcp.NewTool("rule_create",
			mcp.WithDescription("Create an alerting rule. `triggers` is a non-empty array (subset of on_warning, on_failure, on_recovery) — multi-trigger rules fire once per matched trigger per incident. Set applies_to_all=true to fire for every monitor in the org, or pass specific monitor_ids. channel_ids is the list of channels to notify when the rule fires."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Rule name")),
			mcp.WithArray("triggers", mcp.Required(), mcp.Description("Subset of on_warning, on_failure, on_recovery (at least one)"), mcp.WithStringItems()),
			mcp.WithNumber("failure_threshold", mcp.Description("Consecutive failures before firing (default 1)"), mcp.DefaultNumber(1)),
			mcp.WithBoolean("applies_to_all", mcp.Description("If true, applies to every monitor in the org. Defaults to false."), mcp.DefaultBool(false)),
			mcp.WithArray("monitor_ids", mcp.Description("Specific monitor IDs (ignored if applies_to_all=true)"), mcp.WithStringItems()),
			mcp.WithArray("channel_ids", mcp.Description("Channels to notify when this rule fires"), mcp.WithStringItems()),
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
			threshold := int(req.GetFloat("failure_threshold", 1))
			all := req.GetBool("applies_to_all", false)

			args := req.GetArguments()
			r, err := c.Rules.Create(ctx, orgID, client.CreateRuleRequest{
				Name:             name,
				Triggers:         decodeStrings(args["triggers"]),
				FailureThreshold: threshold,
				AppliesToAll:     all,
				MonitorIDs:       decodeStrings(args["monitor_ids"]),
				ChannelIDs:       decodeStrings(args["channel_ids"]),
			})
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(r), nil
		},
	)

	s.AddTool(
		mcp.NewTool("rule_update",
			mcp.WithDescription("Update an alerting rule's fields. Only fields you pass are changed. Passing monitor_ids or channel_ids replaces those lists in full."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("rule_id", mcp.Required(), mcp.Description("Rule ID")),
			mcp.WithString("name", mcp.Description("New name")),
			mcp.WithArray("triggers", mcp.Description("Replacement triggers array (omit to leave existing set unchanged)"), mcp.WithStringItems()),
			mcp.WithNumber("failure_threshold", mcp.Description("New threshold")),
			mcp.WithArray("monitor_ids", mcp.Description("Replacement monitor list"), mcp.WithStringItems()),
			mcp.WithArray("channel_ids", mcp.Description("Replacement channel list"), mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			rid, err := req.RequireString("rule_id")
			if err != nil {
				return errResult(err), nil
			}

			args := req.GetArguments()
			update := client.UpdateRuleRequest{}
			if v, ok := args["name"].(string); ok && v != "" {
				update.Name = &v
			}
			if raw, ok := args["triggers"]; ok && raw != nil {
				update.Triggers = decodeStrings(raw)
			}
			if v, ok := args["failure_threshold"].(float64); ok {
				n := int(v)
				update.FailureThreshold = &n
			}
			if _, ok := args["monitor_ids"]; ok {
				update.MonitorIDs = decodeStrings(args["monitor_ids"])
			}
			if _, ok := args["channel_ids"]; ok {
				update.ChannelIDs = decodeStrings(args["channel_ids"])
			}

			r, err := c.Rules.Update(ctx, orgID, rid, update)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(r), nil
		},
	)

	s.AddTool(
		mcp.NewTool("rule_delete",
			mcp.WithDescription("Delete an alerting rule. Cascades to its escalations and rule/channel junctions."),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("rule_id", mcp.Required(), mcp.Description("Rule ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			rid, err := req.RequireString("rule_id")
			if err != nil {
				return errResult(err), nil
			}
			if err := c.Rules.Delete(ctx, orgID, rid); err != nil {
				return errResult(err), nil
			}
			return mcp.NewToolResultText(`{"ok": true}`), nil
		},
	)
}

// decodeStrings converts a JSON-array-of-strings argument to []string, tolerating
// missing or malformed entries.
func decodeStrings(raw any) []string {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
