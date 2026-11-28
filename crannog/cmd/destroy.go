package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var destroyYes bool

var destroyCmd = &cobra.Command{
	Use:   "destroy [branch]",
	Short: "Destroy the crannog of a branch",
	Long: `Destroys the crannog of a branch, by default the checked-out one: its
containers, network, volumes, remote and state on the host. The image stays.

Commits that are not in your repository and uncommitted changes in the
crannog are lost: fetch what you want to keep first. crannog asks for
confirmation.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeFirstArg(startBranches),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := crannog.New(newUI(destroyYes))
		if err != nil {
			return err
		}
		return m.Destroy(firstArg(args))
	},
}

func init() {
	addYesFlag(destroyCmd, &destroyYes)
	rootCmd.AddCommand(destroyCmd)
}
