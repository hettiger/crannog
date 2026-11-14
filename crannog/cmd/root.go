package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "crannog",
	Short: "Let coding agents work autonomously in sandboxes",
	Long: `crannog lets coding agents work autonomously in sandboxes.

Like its namesake, an island dwelling in a lake, a crannog is isolated from
its surroundings. The only ways in and out are causeways you control:
git, a network proxy and, optionally, volume mounts.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// exitCode is crannog's exit code if the command succeeded, e.g. the exit
// code of a session.
var exitCode int

// Execute runs the root command. Errors exit with 1.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, ui.ErrDeclined) {
			fmt.Fprintln(os.Stderr, "Aborted.")
		} else {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		os.Exit(1)
	}
	os.Exit(exitCode)
}

// newUI returns the UI for a command; yes answers confirmations.
func newUI(yes bool) *ui.UI {
	return &ui.UI{Prompter: ui.Terminal{}, Out: os.Stderr, Yes: yes}
}

// addYesFlag adds --yes to a command that asks confirmations.
func addYesFlag(c *cobra.Command, yes *bool) {
	c.Flags().BoolVarP(yes, "yes", "y", false, "answer confirmations with yes (never approves a setup)")
}
