package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var useZedCmd = &cobra.Command{
	Use:   "zed [branch]",
	Short: "Open the running crannog of a branch in Zed",
	Long: `Opens the repository in the running crannog of a branch, by default the
checked-out one, in Zed, as a remote project over SSH. It needs Zed's
command line tool (in Zed: Zed > Install CLI) and the Include of crannog
use's help.`,
	Example: `  crannog use zed
  crannog use zed feature/x`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeFirstArg(startBranches),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := crannog.New(newUI(false))
		if err != nil {
			return err
		}
		return m.UseZed(firstArg(args))
	},
}

func init() {
	useCmd.AddCommand(useZedCmd)
}
