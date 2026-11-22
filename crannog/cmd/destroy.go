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

Before that, crannog fetches the crannog's branches and shows what would be
lost: branches with commits that are not in your repository and worktrees
with uncommitted changes. It asks for confirmation.`,
	Args: cobra.MaximumNArgs(1),
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
