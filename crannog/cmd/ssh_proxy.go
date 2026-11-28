package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/runtime"
)

var sshProxyCmd = &cobra.Command{
	Use:   "ssh-proxy [--container-cli <path>] <repository-id> <branch-slug>",
	Short: "Connect ssh to a crannog (the ProxyCommand of its Host block)",
	Long: `Connects its standard input and output to sshd in a running crannog.
ssh runs it as the ProxyCommand of the Host blocks crannog writes to
~/.crannog/ssh; it is not meant to be run by hand. It never starts a
crannog. --container-cli names the container CLI to run, for apps whose
PATH does not have it.`,
	Args:   cobra.ExactArgs(2),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cli := runtime.CLI{Path: sshProxyContainerCLI}
		if cli.Path == "" {
			if err := runtime.CheckCLI(); err != nil {
				return err
			}
		}
		h, err := home.OpenCrannogHome()
		if err != nil {
			return err
		}
		defer h.Close()
		return crannog.SSHProxy(h, cli, cli.Replace, args[0], args[1])
	},
}

var sshProxyContainerCLI string

func init() {
	sshProxyCmd.Flags().StringVar(&sshProxyContainerCLI, "container-cli", "", "path of the container CLI")
	rootCmd.AddCommand(sshProxyCmd)
}
