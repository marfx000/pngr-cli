package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/cliclient"
	"pngr.dev/cli/internal/output"
	"pngr.dev/cli/pkg/client"
)

func init() {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authWhoamiCmd, authTokenCmd)
	rootCmd.AddCommand(authCmd)
}

var authLoginToken string

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store a personal access token from /settings/api-tokens",
	Long: `Authenticate the CLI by pasting a PAT minted in the web UI.

The CLI is purely a credential cache: it stores the token, sends it as
Authorization: Bearer pngr_pat_… on every request, and clears it on
logout. It does not mint, refresh, or revoke anything — those operations
live at https://pngr.dev/settings/api-tokens.

  pngr auth login --token pngr_pat_…

Or, for CI / MCP configs, export PNGR_TOKEN=pngr_pat_… in the
environment — it overrides whatever is in the config file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if authLoginToken == "" {
			return fmt.Errorf("--token is required; mint one at https://pngr.dev/settings/api-tokens")
		}
		if !strings.HasPrefix(authLoginToken, client.APITokenPrefix) {
			return fmt.Errorf("token must start with %q — paste the value shown by /settings/api-tokens", client.APITokenPrefix)
		}

		globalConfig.Token = authLoginToken
		if err := globalConfig.Save(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "token saved")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&authLoginToken, "token", "", "PAT from /settings/api-tokens (required)")
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear the locally-stored PAT (does not revoke it server-side)",
	Long: `Remove the stored PAT from ~/.config/pngr/config.yaml. Makes no
network call — the token remains valid for anyone else holding a copy.
Revoke it explicitly at https://pngr.dev/settings/api-tokens.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		globalConfig.Token = ""
		if err := globalConfig.Save(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "logged out (local only; revoke at /settings/api-tokens)")
		return nil
	},
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated user and active org",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := cliclient.New(globalConfig)
		if err != nil {
			return err
		}
		me, err := c.Me.Get(context.Background())
		if err != nil {
			return err
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, me)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "user:        %s\n", me.Email)
		fmt.Fprintf(cmd.OutOrStdout(), "id:          %s\n", me.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "verified:    %t\n", me.EmailVerified)
		if globalConfig.ActiveOrgID != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "active org:  %s\n", globalConfig.ActiveOrgID)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "active org:  (none — run `pngr org use <slug>`)")
		}
		return nil
	},
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the saved API token (useful for CI: `export PNGR_TOKEN=$(pngr auth token)`)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if globalConfig.Token == "" {
			return cliclient.ErrNotAuthenticated
		}
		fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(globalConfig.Token))
		return nil
	},
}
