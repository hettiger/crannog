package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var (
	openZed      bool
	openInsecure bool
	openPort     int
)

var openCmd = &cobra.Command{
	Use:   "open [branch]",
	Short: "Open the running crannog of a branch in the browser or in Zed",
	Long: `Opens https://<address> of the running crannog of a branch, by default the
checked-out one, in the browser: its address on its network, which the Mac
reaches. --insecure opens http://, --port adds a port. crannog checks
first that something accepts connections there and fails otherwise; which
server answers is up to the crannog's setup.

--zed opens the repository in the crannog in Zed instead, as a remote
project over SSH. It needs this line at the top of ~/.ssh/config:

  Include ~/.crannog/ssh/config

crannog open never starts a crannog; start it e.g. with crannog checkout -d.`,
	Example: `  crannog open
  crannog open --insecure --port 8080
  crannog open feature/x --zed`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeFirstArg(startBranches),
	RunE: func(cmd *cobra.Command, args []string) error {
		port := 0
		if cmd.Flags().Changed("port") {
			if openPort < 1 || openPort > 65535 {
				return fmt.Errorf("--port must be between 1 and 65535, not %d", openPort)
			}
			port = openPort
		}
		m, err := crannog.New(newUI(false))
		if err != nil {
			return err
		}
		if openZed {
			return m.OpenZed(firstArg(args))
		}
		return m.OpenWeb(firstArg(args), crannog.WebOptions{Insecure: openInsecure, Port: uint16(port)})
	},
}

func init() {
	openCmd.Flags().BoolVar(&openZed, "zed", false, "open the repository in Zed over SSH instead of the browser")
	openCmd.Flags().BoolVar(&openInsecure, "insecure", false, "open http:// instead of https://")
	openCmd.Flags().IntVar(&openPort, "port", 0, "the port of the server in the crannog (default: the scheme's)")
	openCmd.MarkFlagsMutuallyExclusive("zed", "insecure")
	openCmd.MarkFlagsMutuallyExclusive("zed", "port")
	rootCmd.AddCommand(openCmd)
}
