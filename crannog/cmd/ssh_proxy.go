package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/runtime"
)

var sshProxyCmd = &cobra.Command{
	Use:   "ssh-proxy <repository-id> <branch-slug>",
	Short: "Connect ssh to a crannog (the ProxyCommand of its Host block)",
	Long: `Connects its standard input and output to sshd in a running crannog.
ssh runs it as the ProxyCommand of the Host blocks crannog writes to
~/.crannog/ssh; it is not meant to be run by hand. It never starts a
crannog.`,
	Args:   cobra.ExactArgs(2),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runtime.CheckCLI(); err != nil {
			return err
		}
		h, err := home.OpenCrannogHome()
		if err != nil {
			return err
		}
		defer h.Close()
		cli := runtime.CLI{}
		return crannog.SSHProxy(h, cli, cli.Replace, args[0], args[1])
	},
}

func init() {
	rootCmd.AddCommand(sshProxyCmd)
}
