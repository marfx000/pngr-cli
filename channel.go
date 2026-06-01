package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/output"
	"pngr.dev/cli/pkg/client"
)

func init() {
	channelCmd := &cobra.Command{
		Use:     "channel",
		Aliases: []string{"channels", "ch"},
		Short:   "Manage notification channels",
	}
	channelCmd.AddCommand(
		channelListCmd, channelCreateCmd, channelDeleteCmd, channelTestCmd,
	)
	rootCmd.AddCommand(channelCmd)
}

var channelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notification channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		channels, err := c.Channels.List(context.Background(), orgID)
		if err != nil {
			return err
		}
		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, channels)
		}
		rows := make([][]string, len(channels))
		for i, ch := range channels {
			enabled := "yes"
			if !ch.Enabled {
				enabled = "no"
			}
			rows[i] = []string{ch.ID, ch.Name, ch.Type, enabled}
		}
		return output.Table(cmd.OutOrStdout(), []string{"ID", "NAME", "TYPE", "ENABLED"}, rows)
	},
}

var (
	channelCreateName string
	channelCreateType string
	channelCreateCfg  string
)

var channelCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a notification channel",
	Long: `Create a channel. Config is a JSON object whose shape depends on type:

  slack / ms_teams:  {"webhook_url":"..."}
  telegram:          {"bot_token":"...","chat_id":"..."}
  email:             {"address":"..."}
  webhook:           {"url":"..."}
  sms:               {"to":"+15551234567"}   (provider account configured server-side)
  pagerduty:         {"routing_key":"..."}   (Events API v2 integration key)
  opsgenie:          {"api_key":"...","region":"us"}   (region: "us" | "eu", default "us")

Example:
  pngr channel create --name 'Ops Slack' --type slack \
    --config '{"webhook_url":"https://hooks.slack.com/..."}'

The config is encrypted at rest server-side and never returned in API responses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}

		var cfg map[string]any
		if err := json.Unmarshal([]byte(channelCreateCfg), &cfg); err != nil {
			return fmt.Errorf("--config must be a JSON object: %w", err)
		}
		ch, err := c.Channels.Create(context.Background(), orgID, client.CreateChannelRequest{
			Name: channelCreateName, Type: channelCreateType, Config: cfg,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "created channel %s (%s)\n", ch.ID, ch.Name)
		return nil
	},
}

func init() {
	f := channelCreateCmd.Flags()
	f.StringVar(&channelCreateName, "name", "", "channel name (required)")
	f.StringVar(&channelCreateType, "type", "", "type: slack | ms_teams | telegram | email | webhook | sms | pagerduty | opsgenie (required)")
	f.StringVar(&channelCreateCfg, "config", "{}", "JSON config object (see --help for shape)")
	_ = channelCreateCmd.MarkFlagRequired("name")
	_ = channelCreateCmd.MarkFlagRequired("type")
}

var channelDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete a notification channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		ch, err := resolveChannel(c, orgID, args[0])
		if err != nil {
			return err
		}
		if err := c.Channels.Delete(context.Background(), orgID, ch.ID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", ch.Name)
		return nil
	},
}

var channelTestCmd = &cobra.Command{
	Use:   "test <id-or-name>",
	Short: "Send a test notification through the channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		ch, err := resolveChannel(c, orgID, args[0])
		if err != nil {
			return err
		}
		if err := c.Channels.Test(context.Background(), orgID, ch.ID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "test notification sent via %s\n", ch.Name)
		return nil
	},
}

// resolveChannel accepts a UUID or a name. Channels don't have a single-get
// endpoint so this lists + filters. Ambiguous name → error.
func resolveChannel(c *client.Client, orgID, idOrName string) (*client.Channel, error) {
	channels, err := c.Channels.List(context.Background(), orgID)
	if err != nil {
		return nil, err
	}
	if looksLikeUUID(idOrName) {
		for i := range channels {
			if channels[i].ID == idOrName {
				return &channels[i], nil
			}
		}
		return nil, fmt.Errorf("no channel with ID %q", idOrName)
	}
	var match *client.Channel
	for i := range channels {
		if channels[i].Name == idOrName {
			if match != nil {
				return nil, fmt.Errorf("multiple channels named %q; use the ID instead", idOrName)
			}
			match = &channels[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no channel matching %q", idOrName)
	}
	return match, nil
}
