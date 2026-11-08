package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set at build time via
// -ldflags "-X github.com/hettiger/crannog/crannog/cmd.version=..."
var version = "dev"

// Version returns the version: the one set at build time, else the module
// version Go recorded (go install sets it, a plain go build does not),
// else "dev".
func Version() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of crannog",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
