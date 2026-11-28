package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var stopCmd = &cobra.Command{
	Use:   "stop [branch]",
	Short: "Stop the crannog of a branch",
	Long: `Stops the crannog of a branch, by default the checked-out one: ends its
sessions, detached ones included, then stops the crannog and its proxy and
removes its remote. Nothing is deleted; the next crannog checkout starts
the crannog again.

crannog does not ask for confirmation. It asks the programs of the sessions
to end and waits for them.`,
	Example: `  crannog stop
  crannog stop feature/x`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeFirstArg(startBranches),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := crannog.New(newUI(false))
		if err != nil {
			return err
		}
		return m.Stop(firstArg(args))
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
