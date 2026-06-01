package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/output"
	"pngr.dev/cli/pkg/client"
)

func init() {
	ruleCmd := &cobra.Command{
		Use:     "rule",
		Aliases: []string{"rules"},
		Short:   "Manage alerting rules",
	}
	ruleCmd.AddCommand(ruleListCmd, ruleCreateCmd, ruleEditCmd, ruleDeleteCmd)
	rootCmd.AddCommand(ruleCmd)
}

var ruleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List alerting rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		rules, err := c.Rules.List(context.Background(), orgID)
		if err != nil {
			return err
		}
		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, rules)
		}
		rows := make([][]string, len(rules))
		for i, r := range rules {
			scope := "all monitors"
			if !r.AppliesToAll {
				scope = strconv.Itoa(len(r.MonitorIDs)) + " monitor(s)"
			}
			rows[i] = []string{r.ID, r.Name, strings.Join(r.Triggers, ","), strconv.Itoa(r.FailureThreshold), scope}
		}
		return output.Table(cmd.OutOrStdout(),
			[]string{"ID", "NAME", "TRIGGERS", "THRESHOLD", "SCOPE"}, rows)
	},
}

var (
	ruleCreateName       string
	ruleCreateTriggers   []string
	ruleCreateThreshold  int
	ruleCreateAll        bool
	ruleCreateMonitorIDs []string
	ruleCreateChannelIDs []string
)

var ruleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an alerting rule",
	Long: `Create an alerting rule that dispatches notifications when monitors
transition state. --triggers is a comma-separated subset of:
on_warning | on_failure | on_recovery. Multi-trigger rules fire
once per matched trigger per incident.

Examples:

  pngr rule create --name 'Critical down alert' \
    --triggers on_failure --threshold 1 --all \
    --channel-ids slack-id,email-id

  pngr rule create --name 'Full lifecycle to ops' \
    --triggers on_warning,on_failure,on_recovery \
    --monitor-ids mon-1,mon-2 --channel-ids slack-id`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		r, err := c.Rules.Create(context.Background(), orgID, client.CreateRuleRequest{
			Name:             ruleCreateName,
			Triggers:         ruleCreateTriggers,
			FailureThreshold: ruleCreateThreshold,
			AppliesToAll:     ruleCreateAll,
			MonitorIDs:       ruleCreateMonitorIDs,
			ChannelIDs:       ruleCreateChannelIDs,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "created rule %s (%s)\n", r.ID, r.Name)
		return nil
	},
}

func init() {
	f := ruleCreateCmd.Flags()
	f.StringVar(&ruleCreateName, "name", "", "rule name (required)")
	// Default is the most common single-trigger workflow; users with
	// multi-trigger needs override with e.g. `--triggers on_warning,on_failure`.
	f.StringSliceVar(&ruleCreateTriggers, "triggers", []string{"on_failure"},
		"comma-separated triggers: on_warning | on_failure | on_recovery (multi-select; at least one)")
	f.IntVar(&ruleCreateThreshold, "threshold", 1, "consecutive failures before firing")
	f.BoolVar(&ruleCreateAll, "all", false, "apply to all monitors in the org")
	f.StringSliceVar(&ruleCreateMonitorIDs, "monitor-ids", nil, "specific monitor IDs (ignored if --all)")
	f.StringSliceVar(&ruleCreateChannelIDs, "channel-ids", nil, "channels to notify (required for the rule to do anything useful)")
	_ = ruleCreateCmd.MarkFlagRequired("name")
}

var (
	ruleEditName       string
	ruleEditTriggers   []string
	ruleEditThreshold  int
	ruleEditRenotify   int
	ruleEditMonitorIDs []string
	ruleEditChannelIDs []string
)

var ruleEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Update fields on an existing rule",
	Long: `Edit a rule. Only flags you set are sent to the server. Pass
--monitor-ids or --channel-ids to replace those lists in full.

Note: applies_to_all currently cannot be toggled via edit — delete and
recreate the rule to switch scope.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		r, err := resolveRule(c, orgID, args[0])
		if err != nil {
			return err
		}

		req := client.UpdateRuleRequest{}
		if cmd.Flags().Changed("name") {
			req.Name = &ruleEditName
		}
		if cmd.Flags().Changed("triggers") {
			req.Triggers = ruleEditTriggers
		}
		if cmd.Flags().Changed("threshold") {
			req.FailureThreshold = &ruleEditThreshold
		}
		if cmd.Flags().Changed("renotify") {
			req.RenotifyMinutes = &ruleEditRenotify
		}
		if cmd.Flags().Changed("monitor-ids") {
			req.MonitorIDs = ruleEditMonitorIDs
		}
		if cmd.Flags().Changed("channel-ids") {
			req.ChannelIDs = ruleEditChannelIDs
		}

		updated, err := c.Rules.Update(context.Background(), orgID, r.ID, req)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "updated rule %s\n", updated.Name)
		return nil
	},
}

func init() {
	f := ruleEditCmd.Flags()
	f.StringVar(&ruleEditName, "name", "", "new name")
	f.StringSliceVar(&ruleEditTriggers, "triggers", nil,
		"comma-separated replacement triggers: on_warning | on_failure | on_recovery")
	f.IntVar(&ruleEditThreshold, "threshold", 0, "consecutive failures before firing")
	f.IntVar(&ruleEditRenotify, "renotify", 0, "renotify every N minutes while down (0 disables)")
	f.StringSliceVar(&ruleEditMonitorIDs, "monitor-ids", nil, "replacement monitor list")
	f.StringSliceVar(&ruleEditChannelIDs, "channel-ids", nil, "replacement channel list")
}

var ruleDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete an alerting rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		r, err := resolveRule(c, orgID, args[0])
		if err != nil {
			return err
		}
		if err := c.Rules.Delete(context.Background(), orgID, r.ID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", r.Name)
		return nil
	},
}

// resolveRule accepts a UUID or a name. Rules.Get exists but we still need
// to do a list traversal for name-resolution, so keep the same approach
// as resolveChannel for consistency.
func resolveRule(c *client.Client, orgID, idOrName string) (*client.Rule, error) {
	rules, err := c.Rules.List(context.Background(), orgID)
	if err != nil {
		return nil, err
	}
	if looksLikeUUID(idOrName) {
		for i := range rules {
			if rules[i].ID == idOrName {
				return &rules[i], nil
			}
		}
		return nil, fmt.Errorf("no rule with ID %q", idOrName)
	}
	var match *client.Rule
	for i := range rules {
		if rules[i].Name == idOrName {
			if match != nil {
				return nil, fmt.Errorf("multiple rules named %q; use the ID instead", idOrName)
			}
			match = &rules[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no rule matching %q", idOrName)
	}
	return match, nil
}
