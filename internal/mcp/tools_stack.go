package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"

	"pngr.dev/cli/pkg/client"
)

// registerStackTools wires the gitops surface from SPEC §2.11 + §8.5 into
// the MCP tool set. Two tools:
//
//	stack_dump  — read-only export of the org's current state as a Stack.
//	stack_apply — diff-first apply (re-call with confirm=true to execute).
//
// The confirm-on-second-call shape matches the §8.5 contract: agents see
// the changeset, can show it to their user, then re-invoke the tool with
// confirm=true to land the changes. This avoids the agent silently
// mutating shared infra.
func registerStackTools(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("stack_dump",
			mcp.WithDescription(
				"Export the active org as a Stack YAML document (SPEC §2.11). Channel secrets are emitted as ${SECRET_NAME} placeholders. Read-only — safe for any-member access.",
			),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("format", mcp.Description(`"yaml" (default — human-friendly) or "json"`), mcp.Enum("yaml", "json"), mcp.DefaultString("yaml")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			stack, err := c.Stack.Dump(ctx, orgID)
			if err != nil {
				return errResult(err), nil
			}
			format := req.GetString("format", "yaml")
			if format == "yaml" {
				out, err := yaml.Marshal(stack)
				if err != nil {
					return errResult(err), nil
				}
				return mcp.NewToolResultText(string(out)), nil
			}
			return jsonResult(stack), nil
		},
	)

	s.AddTool(
		mcp.NewTool("stack_apply",
			mcp.WithDescription(
				"Apply a Stack file (SPEC §2.11) to the active org. Two-step protocol: first call returns the changeset (no mutations). Re-call with confirm=true to execute. The file may be YAML or JSON — set format accordingly. Use prune to delete resources not in the file (comma list of channels|monitors|rules, or 'all'). Admin/Owner only.",
			),
			mcp.WithString("org_id", mcp.Required(), mcp.Description("Organization ID")),
			mcp.WithString("file", mcp.Required(), mcp.Description("Stack document (YAML or JSON, per format)")),
			mcp.WithString("format", mcp.Description(`"yaml" (default) or "json"`), mcp.Enum("yaml", "json"), mcp.DefaultString("yaml")),
			mcp.WithBoolean("confirm", mcp.Description("Set true on the SECOND call to actually execute the apply. First call (or false) returns the diff only."), mcp.DefaultBool(false)),
			mcp.WithString("prune", mcp.Description("Optional. Comma list of kinds to delete-when-missing (channels|monitors|rules), or 'all'. Empty/omitted = additive-by-default (no deletes).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgID, errRes := requireOrgID(req)
			if errRes != nil {
				return errRes, nil
			}
			rawFile, err := req.RequireString("file")
			if err != nil {
				return errResult(err), nil
			}
			format := req.GetString("format", "yaml")
			confirm := req.GetBool("confirm", false)
			pruneRaw := req.GetString("prune", "")

			var stack client.Stack
			switch format {
			case "json":
				if err := json.Unmarshal([]byte(rawFile), &stack); err != nil {
					return errResult(fmt.Errorf("parsing JSON file: %w", err)), nil
				}
			default:
				if err := yaml.Unmarshal([]byte(rawFile), &stack); err != nil {
					return errResult(fmt.Errorf("parsing YAML file: %w", err)), nil
				}
			}

			opts := client.ApplyOptions{
				DryRun: !confirm, // first call (or confirm=false) is always dry-run
				Prune:  parsePruneCSV(pruneRaw),
			}
			result, err := c.Stack.Apply(ctx, orgID, &stack, opts)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(result), nil
		},
	)
}

// parsePruneCSV is shared with the CLI in cmd/pngr/stack.go semantically
// but kept local to keep cmd/pngr/internal/mcp's import graph small.
func parsePruneCSV(raw string) []client.ChangeKind {
	if raw == "" {
		return nil
	}
	if raw == "all" {
		return []client.ChangeKind{client.KindChannel, client.KindMonitor, client.KindRule}
	}
	var out []client.ChangeKind
	for _, p := range splitCSV(raw) {
		switch p {
		case "channel", "channels":
			out = append(out, client.KindChannel)
		case "monitor", "monitors":
			out = append(out, client.KindMonitor)
		case "rule", "rules":
			out = append(out, client.KindRule)
		}
	}
	return out
}

func splitCSV(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		if r == ' ' || r == '\t' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
