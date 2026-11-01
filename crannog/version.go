package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of crannog",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
