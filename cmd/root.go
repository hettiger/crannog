package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "crannog",
	Short: "Let coding agents work autonomously in sandboxes",
	Long: `crannog lets coding agents work autonomously in sandboxes.

Like its namesake, an island dwelling in a lake, a crannog is isolated from
its surroundings. The only ways in and out are causeways you control:
git, and optionally a network proxy and volume mounts.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
