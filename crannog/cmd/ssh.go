package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var (
	sshHost bool
	sshPath bool
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
code.

--host prints the crannog's SSH host instead, for other tools; --path
prints <host>:<path of the repository in the crannog>, as scp, rsync and
sftp take it. The crannog need not run for them.`,
	Example: `  crannog ssh
  crannog ssh feature/x
  crannog ssh -- git status
  crannog ssh feature/x -- npm test
  crannog ssh --host
  scp notes.txt "$(crannog ssh --path)/"`,
	Args:              branchAndCommandArgs,
	ValidArgsFunction: completeFirstArg(startBranches),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch, command := splitBranchAndCommand(cmd, args)
		if (sshHost || sshPath) && len(command) > 0 {
			return errors.New("--host and --path take no command after --")
		}
		m, err := crannog.New(newUI(false))
		if err != nil {
			return err
		}
		if sshHost || sshPath {
			get := m.SSHHost
			if sshPath {
				get = m.SSHPath
			}
			s, err := get(branch)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), s)
			return err
		}
		return m.SSH(branch, command)
	},
}

func init() {
	sshCmd.Flags().BoolVar(&sshHost, "host", false, "print the crannog's SSH host")
	sshCmd.Flags().BoolVar(&sshPath, "path", false, "print <host>:<path of the repository in the crannog>")
	sshCmd.MarkFlagsMutuallyExclusive("host", "path")
	rootCmd.AddCommand(sshCmd)
}
