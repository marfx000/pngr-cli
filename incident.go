package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/output"
	"pngr.dev/cli/pkg/client"
)

func init() {
	incidentCmd := &cobra.Command{
		Use:     "incident",
		Aliases: []string{"incidents"},
		Short:   "View incidents",
	}
	incidentCmd.AddCommand(incidentListCmd, incidentShowCmd)
	rootCmd.AddCommand(incidentCmd)
}

var incidentShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show incident details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		inc, err := c.Incidents.Get(context.Background(), orgID, args[0])
		if err != nil {
			return err
		}
		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, inc)
		}
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "id:       %s\n", inc.ID)
		fmt.Fprintf(w, "monitor:  %s\n", inc.MonitorID)
		fmt.Fprintf(w, "status:   %s\n", resolvedStatus(inc.Resolved))
		fmt.Fprintf(w, "started:  %s\n", inc.StartedAt.Format("2006-01-02 15:04:05"))
		if inc.EndedAt != nil {
			fmt.Fprintf(w, "ended:    %s\n", inc.EndedAt.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Fprintln(w, "ended:    (ongoing)")
		}
		fmt.Fprintf(w, "duration: %s\n", formatDuration(inc.DurationSeconds))
		return nil
	},
}

var incidentListLimit int
var incidentListMonitor string

var incidentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent incidents (optionally filter by monitor)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}

		var incidents []client.Incident
		if incidentListMonitor != "" {
			m, err := resolveMonitor(c, orgID, incidentListMonitor)
			if err != nil {
				return err
			}
			incidents, err = c.Incidents.ListByMonitor(context.Background(), orgID, m.ID, incidentListLimit, 0)
			if err != nil {
				return err
			}
		} else {
			incidents, err = c.Incidents.ListByOrg(context.Background(), orgID, incidentListLimit, 0)
			if err != nil {
				return err
			}
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, incidents)
		}
		rows := make([][]string, len(incidents))
		for i, inc := range incidents {
			rows[i] = []string{
				inc.ID,
				inc.MonitorID,
				inc.StartedAt.Format("2006-01-02 15:04:05"),
				formatTimePtr(inc.EndedAt),
				formatDuration(inc.DurationSeconds),
				resolvedStatus(inc.Resolved),
			}
		}
		return output.Table(cmd.OutOrStdout(),
			[]string{"ID", "MONITOR", "STARTED", "ENDED", "DURATION", "STATUS"}, rows)
	},
}

func init() {
	incidentListCmd.Flags().IntVar(&incidentListLimit, "limit", 20, "max number of incidents to return")
	incidentListCmd.Flags().StringVar(&incidentListMonitor, "monitor", "", "filter by monitor (id or name)")
}

func formatDuration(seconds *int64) string {
	if seconds == nil {
		return "-"
	}
	return (time.Duration(*seconds) * time.Second).String()
}

func resolvedStatus(resolved bool) string {
	if resolved {
		return "resolved"
	}
	return "open"
}
