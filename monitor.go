package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"pngr.dev/cli/internal/output"
	"pngr.dev/cli/pkg/client"
)

func init() {
	monitorCmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"mon"},
		Short:   "Manage monitors",
	}
	monitorCmd.AddCommand(
		monitorListCmd, monitorShowCmd, monitorCreateCmd, monitorEditCmd,
		monitorDeleteCmd, monitorPauseCmd, monitorResumeCmd,
		monitorCheckCmd, monitorWatchCmd,
	)
	rootCmd.AddCommand(monitorCmd)
}

var monitorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List monitors in the active org",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		monitors, err := c.Monitors.List(context.Background(), orgID)
		if err != nil {
			return err
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, monitors)
		}

		rows := make([][]string, len(monitors))
		for i, m := range monitors {
			rows[i] = []string{
				m.ID,
				output.Truncate(m.Name, 24),
				m.Status,
				output.Truncate(m.URL, 40),
				strconv.Itoa(m.IntervalSeconds) + "s",
				formatTimePtr(m.LastCheckedAt),
			}
		}
		return output.Table(cmd.OutOrStdout(),
			[]string{"ID", "NAME", "STATUS", "URL", "INTERVAL", "LAST CHECKED"}, rows)
	},
}

// monitorShowView bundles everything `pngr monitor show` displays.
// Returned as the JSON / YAML payload so callers can pipe it into jq.
type monitorShowView struct {
	Monitor      *client.Monitor      `json:"monitor"`
	Uptime       *client.UptimeStats  `json:"uptime"`
	RecentChecks []client.CheckResult `json:"recent_checks"`
}

var monitorShowCmd = &cobra.Command{
	Use:   "show <id-or-name>",
	Short: "Show a monitor's details, recent uptime %, and last check results",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		m, err := resolveMonitor(c, orgID, args[0])
		if err != nil {
			return err
		}

		// Best-effort fetch of uptime + recent checks. Failures here don't
		// abort — the monitor card is still useful even without these two.
		ctx := context.Background()
		uptime, _ := c.Monitors.Uptime(ctx, orgID, m.ID, "24h")
		recent, _ := c.Monitors.ListResults(ctx, orgID, m.ID, 10, 0)
		view := monitorShowView{Monitor: m, Uptime: uptime, RecentChecks: recent}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, view)
		}
		return renderMonitorCard(cmd.OutOrStdout(), view)
	},
}

func renderMonitorCard(w io.Writer, v monitorShowView) error {
	m := v.Monitor
	fmt.Fprintf(w, "─── %s ───\n", m.Name)
	fmt.Fprintf(w, "id:           %s\n", m.ID)
	fmt.Fprintf(w, "url:          %s\n", m.URL)
	fmt.Fprintf(w, "method:       %s  http/%s\n", m.Method, m.HTTPVersion)
	fmt.Fprintf(w, "status:       %s\n", statusIcon(m.Status))
	fmt.Fprintf(w, "interval:     %ds\n", m.IntervalSeconds)
	fmt.Fprintf(w, "timeout:      %ds\n", m.TimeoutSeconds)
	fmt.Fprintf(w, "last checked: %s\n", formatTimePtr(m.LastCheckedAt))
	fmt.Fprintf(w, "next check:   %s\n", formatTimePtr(m.NextCheckAt))

	if len(m.Conditions) > 0 {
		fmt.Fprintln(w, "\nCONDITIONS:")
		for _, c := range m.Conditions {
			cfg, _ := json.Marshal(c.Config)
			fmt.Fprintf(w, "  %-12s %s\n", c.Type, string(cfg))
		}
	}

	if v.Uptime != nil {
		fmt.Fprintf(w, "\nUPTIME (24h): %.1f%%  (%d samples)\n",
			v.Uptime.UptimePct, v.Uptime.SampleCount)
	}

	if len(v.RecentChecks) > 0 {
		fmt.Fprintln(w, "\nRECENT CHECKS:")
		rows := make([][]string, len(v.RecentChecks))
		for i, r := range v.RecentChecks {
			result := "✓ pass"
			if !r.Passed {
				result = "✗ fail"
			}
			code := "-"
			if r.StatusCode != nil {
				code = strconv.Itoa(int(*r.StatusCode))
			}
			latency := "-"
			if r.LatencyMs != nil {
				latency = strconv.FormatInt(*r.LatencyMs, 10) + " ms"
			}
			rows[i] = []string{
				r.CheckedAt.Format("2006-01-02 15:04:05"),
				result, code, latency,
				output.Truncate(strings.Join(r.FailureReasons, "; "), 50),
			}
		}
		return output.Table(w, []string{"TIME", "RESULT", "CODE", "LATENCY", "REASONS"}, rows)
	}
	return nil
}

// monitor create flags
var (
	createName            string
	createURL             string
	createMethod          string
	createHTTPVer         string
	createInterval        int
	createTimeout         int
	createVerifyTLS       bool
	createFollowRedirects bool
	// --condition "status_code=200" or "latency=2000" or "body_contains=OK"
	createConditions []string
)

var monitorCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a monitor (flag-driven for now; interactive prompts in a follow-up)",
	Long: `Create a monitor with flags. Conditions accept these shorthand forms:

  --condition "status=200"          → status code 200
  --condition "status=200-299"      → status code in range
  --condition "latency=2000"        → max latency 2000 ms
  --condition "contains=OK"         → response body must contain "OK"
  --condition "not_contains=ERROR"  → response body must NOT contain "ERROR"
  --condition "regex=^pong$"        → response body must match regex

Repeat --condition for multiple checks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}

		conds, err := parseConditions(createConditions)
		if err != nil {
			return err
		}

		m, err := c.Monitors.Create(context.Background(), orgID, client.CreateMonitorRequest{
			Name:            createName,
			URL:             createURL,
			Method:          createMethod,
			HTTPVersion:     createHTTPVer,
			IntervalSeconds: createInterval,
			TimeoutSeconds:  createTimeout,
			VerifyTLS:       &createVerifyTLS,
			FollowRedirects: &createFollowRedirects,
			Conditions:      conds,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "created monitor %s (%s)\n", m.ID, m.Name)
		return nil
	},
}

func init() {
	f := monitorCreateCmd.Flags()
	f.StringVar(&createName, "name", "", "monitor name (required)")
	f.StringVar(&createURL, "url", "", "URL to monitor (required)")
	f.StringVar(&createMethod, "method", "GET", "HTTP method")
	f.StringVar(&createHTTPVer, "http-version", "", `set to "h3" to force HTTP/3 (QUIC). Empty (default) auto-negotiates h1/h2 via ALPN.`)
	f.IntVar(&createInterval, "interval", 300, "check interval in seconds (min 60)")
	f.IntVar(&createTimeout, "timeout", 10, "per-request timeout in seconds (max 30)")
	f.BoolVar(&createVerifyTLS, "verify-tls", true, "verify TLS chain (--verify-tls=false to skip; for internal services behind private CAs)")
	f.BoolVar(&createFollowRedirects, "follow-redirects", true, "follow 3xx up to 10 hops (--follow-redirects=false to return the 3xx as-is)")
	f.StringSliceVar(&createConditions, "condition", nil, "check condition (repeatable; see --help)")
	_ = monitorCreateCmd.MarkFlagRequired("name")
	_ = monitorCreateCmd.MarkFlagRequired("url")
}

var monitorDeleteCmd = &cobra.Command{
	Use:   "delete <id-or-name>",
	Short: "Delete a monitor (soft delete)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		m, err := resolveMonitor(c, orgID, args[0])
		if err != nil {
			return err
		}
		if err := c.Monitors.Delete(context.Background(), orgID, m.ID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", m.Name)
		return nil
	},
}

var monitorPauseCmd = &cobra.Command{
	Use:   "pause <id-or-name>",
	Short: "Pause checks for a monitor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		m, err := resolveMonitor(c, orgID, args[0])
		if err != nil {
			return err
		}
		if err := c.Monitors.Pause(context.Background(), orgID, m.ID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "paused %s\n", m.Name)
		return nil
	},
}

var monitorResumeCmd = &cobra.Command{
	Use:   "resume <id-or-name>",
	Short: "Resume a paused monitor",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		m, err := resolveMonitor(c, orgID, args[0])
		if err != nil {
			return err
		}
		if err := c.Monitors.Resume(context.Background(), orgID, m.ID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "resumed %s\n", m.Name)
		return nil
	},
}

var monitorCheckCmd = &cobra.Command{
	Use:   "check <id-or-name>",
	Short: "Run an immediate check and print the result",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		m, err := resolveMonitor(c, orgID, args[0])
		if err != nil {
			return err
		}
		result, err := c.Monitors.Check(context.Background(), orgID, m.ID)
		if err != nil {
			return err
		}

		if globalConfig.Output != output.FormatTable {
			return output.Render(cmd.OutOrStdout(), globalConfig.Output, result)
		}

		status := "✓ passed"
		if !result.Passed {
			status = "✗ failed"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", status)
		if result.StatusCode != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "status:  %d\n", *result.StatusCode)
		}
		if result.LatencyMs != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "latency: %d ms\n", *result.LatencyMs)
		}
		for _, r := range result.FailureReasons {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", r)
		}
		return nil
	},
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// resolveMonitor accepts either a UUID or a name. Lists all monitors when a
// name is given and matches case-sensitively. Errors on ambiguous matches.
func resolveMonitor(c *client.Client, orgID, idOrName string) (*client.Monitor, error) {
	if looksLikeUUID(idOrName) {
		return c.Monitors.Get(context.Background(), orgID, idOrName)
	}
	monitors, err := c.Monitors.List(context.Background(), orgID)
	if err != nil {
		return nil, err
	}
	var match *client.Monitor
	for i := range monitors {
		if monitors[i].Name == idOrName {
			if match != nil {
				return nil, fmt.Errorf("multiple monitors named %q; use the ID instead", idOrName)
			}
			match = &monitors[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no monitor matching %q", idOrName)
	}
	return match, nil
}

func looksLikeUUID(s string) bool {
	// Crude but sufficient: 8-4-4-4-12 hex pattern is unambiguous in the wild.
	return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

// parseConditions converts shorthand strings ("status=200", "latency=2000",
// "contains=OK") into client.CheckCondition values matching the API schema.
func parseConditions(raw []string) ([]client.CheckCondition, error) {
	out := make([]client.CheckCondition, 0, len(raw))
	for _, c := range raw {
		k, v, ok := strings.Cut(c, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --condition %q (want key=value)", c)
		}
		switch k {
		case "status", "status_code":
			out = append(out, client.CheckCondition{
				Type: "status_code", Config: map[string]any{"expected": v},
			})
		case "latency":
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmt.Errorf("latency must be a number: %w", err)
			}
			out = append(out, client.CheckCondition{
				Type: "latency", Config: map[string]any{"max_ms": n},
			})
		case "contains":
			out = append(out, client.CheckCondition{
				Type: "body_match", Config: map[string]any{"mode": "contains", "value": v},
			})
		case "not_contains":
			out = append(out, client.CheckCondition{
				Type: "body_match", Config: map[string]any{"mode": "not_contains", "value": v},
			})
		case "regex":
			out = append(out, client.CheckCondition{
				Type: "body_match", Config: map[string]any{"mode": "matches_regex", "value": v},
			})
		default:
			return nil, fmt.Errorf("unknown condition key %q (see `pngr monitor create --help`)", k)
		}
	}
	return out, nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// ─── edit ─────────────────────────────────────────────────────────────────────

var (
	editName            string
	editURL             string
	editMethod          string
	editHTTPVer         string
	editInterval        int
	editTimeout         int
	editVerifyTLS       bool
	editFollowRedirects bool
	editConditions      []string
)

var monitorEditCmd = &cobra.Command{
	Use:   "edit <id-or-name>",
	Short: "Update fields on an existing monitor",
	Long: `Edit a monitor. Only flags you set on the command line are sent to the
server — everything else is left untouched. To replace the full condition
list, pass --condition one or more times (same shorthand as create).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}
		m, err := resolveMonitor(c, orgID, args[0])
		if err != nil {
			return err
		}

		// Only include fields the user explicitly passed. cobra's Flag.Changed
		// is true even for flags set to their default value, which is the
		// semantic we want for a PATCH ("the user said this on purpose").
		req := client.UpdateMonitorRequest{}
		if cmd.Flags().Changed("name") {
			req.Name = &editName
		}
		if cmd.Flags().Changed("url") {
			req.URL = &editURL
		}
		if cmd.Flags().Changed("method") {
			req.Method = &editMethod
		}
		if cmd.Flags().Changed("http-version") {
			req.HTTPVersion = &editHTTPVer
		}
		if cmd.Flags().Changed("interval") {
			req.IntervalSeconds = &editInterval
		}
		if cmd.Flags().Changed("timeout") {
			req.TimeoutSeconds = &editTimeout
		}
		if cmd.Flags().Changed("verify-tls") {
			req.VerifyTLS = &editVerifyTLS
		}
		if cmd.Flags().Changed("follow-redirects") {
			req.FollowRedirects = &editFollowRedirects
		}
		if cmd.Flags().Changed("condition") {
			conds, err := parseConditions(editConditions)
			if err != nil {
				return err
			}
			req.Conditions = conds
		}

		updated, err := c.Monitors.Update(context.Background(), orgID, m.ID, req)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "updated monitor %s\n", updated.Name)
		return nil
	},
}

func init() {
	f := monitorEditCmd.Flags()
	f.StringVar(&editName, "name", "", "new name")
	f.StringVar(&editURL, "url", "", "new URL")
	f.StringVar(&editMethod, "method", "", "HTTP method")
	f.StringVar(&editHTTPVer, "http-version", "", `set to "h3" to force HTTP/3 (QUIC); empty to auto-negotiate h1/h2`)
	f.IntVar(&editInterval, "interval", 0, "check interval in seconds (min 60)")
	f.IntVar(&editTimeout, "timeout", 0, "per-request timeout in seconds (max 30)")
	f.BoolVar(&editVerifyTLS, "verify-tls", true, "verify TLS chain (--verify-tls=false to skip)")
	f.BoolVar(&editFollowRedirects, "follow-redirects", true, "follow 3xx (--follow-redirects=false to return 3xx as-is)")
	f.StringSliceVar(&editConditions, "condition", nil,
		"replacement check conditions (repeatable; same shorthand as create)")
}

// ─── watch ────────────────────────────────────────────────────────────────────

var watchInterval int

var monitorWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Live monitor status dashboard (Ctrl+C to exit)",
	Long: `Refresh-on-tick dashboard of every monitor in the active org.

Use --interval to change the refresh period (default 5s). The display
uses ANSI cursor positioning if stdout is a TTY; piped output falls back
to plain re-printing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, orgID, err := newClientWithOrg()
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		isTTY := term.IsTerminal(int(os.Stdout.Fd()))

		if isTTY {
			// Clear once up front; subsequent ticks just rewind the cursor.
			fmt.Fprint(cmd.OutOrStdout(), "\033[2J")
		}

		ticker := time.NewTicker(time.Duration(watchInterval) * time.Second)
		defer ticker.Stop()

		drawOnce := func() {
			if isTTY {
				fmt.Fprint(cmd.OutOrStdout(), "\033[H\033[J") // home + erase below
			}
			ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			monitors, err := c.Monitors.List(ctx2, orgID)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "fetch error: %s\n", err)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pngr watch  %s  (Ctrl+C to exit)\n\n",
				time.Now().Format("15:04:05"))
			rows := make([][]string, len(monitors))
			for i, m := range monitors {
				rows[i] = []string{
					statusIcon(m.Status),
					output.Truncate(m.Name, 28),
					output.Truncate(m.URL, 40),
					formatTimePtr(m.LastCheckedAt),
					fmt.Sprintf("%ds", m.IntervalSeconds),
				}
			}
			_ = output.Table(cmd.OutOrStdout(),
				[]string{"STATUS", "NAME", "URL", "LAST CHECK", "INTERVAL"}, rows)
		}

		drawOnce()
		for {
			select {
			case <-ticker.C:
				drawOnce()
			case <-ctx.Done():
				fmt.Fprintln(cmd.OutOrStdout())
				return nil
			}
		}
	},
}

func init() {
	monitorWatchCmd.Flags().IntVar(&watchInterval, "interval", 5,
		"refresh interval in seconds")
}

func statusIcon(status string) string {
	switch status {
	case "up":
		return "✓ up"
	case "down":
		return "✗ down"
	case "paused":
		return "⏸ paused"
	case "pending":
		return "? pending"
	default:
		return status
	}
}
