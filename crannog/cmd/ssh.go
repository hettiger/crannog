package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var sshCmd = &cobra.Command{
	Use:   "ssh [branch] [-- command...]",
	Short: "Open an SSH shell in the crannog of a branch",
	Long: `Connects with ssh to the crannog of a branch, by default the checked-out
one, and opens the agent's login shell in the repository, or runs the
command after -- there, like a session.

The crannog must be running, e.g. after crannog checkout -d: crannog ssh
never starts it. ssh must use crannog's Host blocks; add this line once at
the top of ~/.ssh/config:

  Include ~/.crannog/ssh/config

An SSH connection is not a session: it does not keep the crannog running
and ends when the crannog stops. ssh's exit code becomes crannog's exit
code.`,
	Example: `  crannog ssh
  crannog ssh feature/x
  crannog ssh -- git status
  crannog ssh feature/x -- npm test`,
	Args: branchAndCommandArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		branch, command := splitBranchAndCommand(cmd, args)
		m, err := crannog.New(newUI(false))
		if err != nil {
			return err
		}
		return m.SSH(branch, command)
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}
