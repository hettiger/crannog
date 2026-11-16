package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var checkoutYes bool

var checkoutCmd = &cobra.Command{
	Use:   "checkout [branch] [-- command...]",
	Short: "Open a session in the crannog of a branch",
	Long: `Opens a session in the crannog of a branch, by default the checked-out one.

crannog builds the crannog if it does not exist and starts it if it is
stopped. If .crannog/ differs from the copy the crannog uses, crannog shows
the difference and asks for approval before it rebuilds the crannog or
reloads its proxy. When the last session ends, the crannog stops.

The session runs session.command from .crannog/config.yml, or the command
after --. The session's exit code becomes crannog's exit code.`,
	Example: `  crannog checkout
  crannog checkout feature/x
  crannog checkout feature/x -- zsh
  crannog checkout -- claude -p "Fix the failing test"`,
	Args: func(cmd *cobra.Command, args []string) error {
		if branchArgs(cmd, args) > 1 {
			return errors.New("takes at most one branch; put the command after --")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		branch, command := "", []string(nil)
		n := branchArgs(cmd, args)
		if n == 1 {
			branch = args[0]
		}
		if cmd.ArgsLenAtDash() >= 0 {
			command = args[n:]
		}
		m, err := crannog.New(newUI(checkoutYes))
		if err != nil {
			return err
		}
		exitCode, err = m.Checkout(branch, command)
		return err
	},
}

// branchArgs returns the number of arguments before --.
func branchArgs(cmd *cobra.Command, args []string) int {
	if dash := cmd.ArgsLenAtDash(); dash >= 0 {
		return dash
	}
	return len(args)
}

func init() {
	addYesFlag(checkoutCmd, &checkoutYes)
	rootCmd.AddCommand(checkoutCmd)
}
