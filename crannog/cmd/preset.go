package cmd

import (
	"github.com/spf13/cobra"
)

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Manage presets for .crannog/",
	Long: `Presets are named templates for a repository's .crannog/. crannog ships the
preset default and uses it for repositories without .crannog/. Your own
presets live in ~/.crannog/presets/<name>/ and replace a bundled preset of
the same name.`,
}

func init() {
	rootCmd.AddCommand(presetCmd)
}
