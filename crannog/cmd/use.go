package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <tool> [branch]",
	Short: "Open a running crannog in a tool on the host",
	Long: `Opens the running crannog of a branch, by default the checked-out one, in
a tool on the host. The tools connect over SSH with your SSH configuration,
so it needs this line at the top of ~/.ssh/config:

  Include ~/.crannog/ssh/config

crannog use never starts a crannog; start it e.g. with crannog checkout -d.
Close the tool, or remove the crannog from it, in the tool itself.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown tool %q; crannog use supports: %s", args[0], strings.Join(tools(cmd), ", "))
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// tools returns the names of the subcommands of use.
func tools(cmd *cobra.Command) []string {
	var names []string
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			names = append(names, c.Name())
		}
	}
	return names
}

func init() {
	rootCmd.AddCommand(useCmd)
}
