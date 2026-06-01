package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/cliconfig"
)

func init() {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and update the CLI config",
	}
	cfgCmd.AddCommand(configShowCmd, configSetCmd)
	rootCmd.AddCommand(cfgCmd)
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current CLI config (token is redacted)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := cliconfig.Path()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "config:        %s\n", path)
		fmt.Fprintf(cmd.OutOrStdout(), "api_url:       %s\n", globalConfig.APIURL)
		fmt.Fprintf(cmd.OutOrStdout(), "token:         %s\n", redactToken(globalConfig.Token))
		if globalConfig.ActiveOrgID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "active_org_id: %s\n", globalConfig.ActiveOrgID)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "active_org_id: (not set)")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "output:        %s\n", globalConfig.Output)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Update a single config field (api_url | token | active_org_id | output)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		switch key {
		case "api_url":
			globalConfig.APIURL = value
		case "token":
			globalConfig.Token = value
		case "active_org_id", "org":
			globalConfig.ActiveOrgID = value
		case "output":
			if value != "table" && value != "json" && value != "yaml" {
				return fmt.Errorf("output must be one of: table | json | yaml")
			}
			globalConfig.Output = value
		default:
			return fmt.Errorf("unknown key %q (try api_url | token | active_org_id | output)", key)
		}
		if err := globalConfig.Save(); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s saved\n", key)
		return nil
	},
}

// redactToken keeps the last 4 characters of the token for a "is this the right
// one?" sanity check, while hiding the rest.
func redactToken(t string) string {
	if t == "" {
		return "(not set)"
	}
	if len(t) <= 8 {
		return "****"
	}
	return "****" + t[len(t)-4:]
}
