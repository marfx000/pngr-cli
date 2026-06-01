package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/cliclient"
	"pngr.dev/cli/internal/output"
	"pngr.dev/cli/pkg/client"
)

func init() {
	orgCmd := &cobra.Command{
		Use:   "org",
		Short: "Manage organizations",
	}
	orgCmd.AddCommand(orgListCmd, orgUseCmd, orgShowCmd, orgMembersCmd, orgInviteCmd)
	rootCmd.AddCommand(orgCmd)
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations you belong to",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := cliclient.New(globalConfig)
		if err != nil {
			return err
		}
		orgs, err := c.Orgs.List(context.Background())
		if err != nil {
			return err
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, orgs)
		}

		rows := make([][]string, len(orgs))
		for i, o := range orgs {
			marker := " "
			if o.ID == globalConfig.ActiveOrgID {
				marker = "*"
			}
			rows[i] = []string{marker, o.ID, o.Name, o.Slug}
		}
		return output.Table(cmd.OutOrStdout(), []string{"", "ID", "NAME", "SLUG"}, rows)
	},
}

var orgUseCmd = &cobra.Command{
	Use:   "use <slug-or-id>",
	Short: "Set the active organization (used by subsequent commands)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := cliclient.New(globalConfig)
		if err != nil {
			return err
		}
		orgs, err := c.Orgs.List(context.Background())
		if err != nil {
			return err
		}

		target := args[0]
		var match *client.Org
		for i := range orgs {
			if orgs[i].ID == target || orgs[i].Slug == target {
				match = &orgs[i]
				break
			}
		}
		if match == nil {
			return fmt.Errorf("no organization matching %q (try `pngr org list`)", target)
		}

		globalConfig.ActiveOrgID = match.ID
		if err := globalConfig.Save(); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "active org set to %s (%s)\n", match.Name, match.Slug)
		return nil
	},
}

var orgShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the active organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		org, err := c.Orgs.Get(context.Background(), orgID)
		if err != nil {
			return err
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, org)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "id:         %s\n", org.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "name:       %s\n", org.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "slug:       %s\n", org.Slug)
		fmt.Fprintf(cmd.OutOrStdout(), "logo_url:   %s\n", org.LogoURL)
		fmt.Fprintf(cmd.OutOrStdout(), "created_at: %s\n", org.CreatedAt.Format("2006-01-02 15:04:05"))
		return nil
	},
}

var orgMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "List members of the active org",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		members, err := c.Orgs.ListMembers(context.Background(), orgID)
		if err != nil {
			return err
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, members)
		}

		rows := make([][]string, len(members))
		for i, m := range members {
			rows[i] = []string{m.Email, m.DisplayName, m.Role, m.JoinedAt.Format("2006-01-02")}
		}
		return output.Table(cmd.OutOrStdout(), []string{"EMAIL", "NAME", "ROLE", "JOINED"}, rows)
	},
}

var inviteRole string

var orgInviteCmd = &cobra.Command{
	Use:   "invite <email>",
	Short: "Invite a user to the active org",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		inv, err := c.Orgs.Invite(context.Background(), orgID, client.InviteRequest{
			Email: args[0], Role: inviteRole,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "invited %s as %s\n", inv.Email, inv.Role)
		// TODO: once Phase 3 email delivery is live in prod, drop this hint.
		fmt.Fprintf(cmd.OutOrStdout(), "  invitation token (give to invitee): %s\n", inv.Token)
		return nil
	},
}

func init() {
	orgInviteCmd.Flags().StringVar(&inviteRole, "role", "member", "role: admin | member | viewer")
}

// newClientWithOrg builds the client and returns the active org ID, returning
// ErrNoActiveOrg with a hint if none is configured. Used by every command that
// operates on an org's resources.
func newClientWithOrg() (*client.Client, string, error) {
	c, err := cliclient.New(globalConfig)
	if err != nil {
		return nil, "", err
	}
	if globalConfig.ActiveOrgID == "" {
		return nil, "", cliclient.ErrNoActiveOrg
	}
	return c, globalConfig.ActiveOrgID, nil
}
