package main

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
git, a network proxy and, optionally, volume mounts.`,
}

// execute runs the root command and exits with 1 on errors.
func execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
