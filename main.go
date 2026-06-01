package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"pngr.dev/cli/internal/cliconfig"
)

// globalConfig is loaded once in PersistentPreRun and shared across all
// commands. Mutations during a command (e.g. `pngr auth login` updating the
// token) are saved back to disk before the command returns.
var globalConfig *cliconfig.Config

// outputOverride is set by the --output flag and wins over config.Output.
var outputOverride string

// orgOverride is set by --org and wins over config.ActiveOrgID. Accepts a
// UUID or a slug; commands resolve slugs by listing orgs.
var orgOverride string

var rootCmd = &cobra.Command{
	Use:   "pngr",
	Short: "pngr.dev CLI — manage URL monitors from the terminal",
	Long: `pngr is a command-line interface for pngr.dev.

Manage monitors, notification channels, and alerting rules from your
terminal or CI pipeline. Run 'pngr auth login' to get started.

Output format can be table (default), json, or yaml — set via --output,
PNGR_OUTPUT env var, or 'pngr config set output=json'.`,
	SilenceUsage:  true, // already shown in command help; don't repeat after errors
	SilenceErrors: true, // we print them ourselves with a clearer prefix
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cliconfig.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if outputOverride != "" {
			cfg.Output = outputOverride
		}
		if orgOverride != "" {
			cfg.ActiveOrgID = orgOverride
		}
		globalConfig = cfg
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputOverride, "output", "o", "",
		"output format: table | json | yaml")
	rootCmd.PersistentFlags().StringVar(&orgOverride, "org", "",
		"organization ID to operate against (overrides config)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "pngr: "+renderError(err))
		os.Exit(1)
	}
}
