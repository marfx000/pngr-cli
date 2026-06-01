package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags="-X main.version=v1.2.3"
// when GoReleaser cuts a release. Otherwise stays "dev" so local builds
// don't masquerade as a tagged version.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the pngr CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "pngr %s (%s/%s, %s)\n",
			version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
