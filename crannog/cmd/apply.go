package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var applyYes bool

var applyCmd = &cobra.Command{
	Use:   "apply [branch]",
	Short: "Apply changes to .crannog/ without opening a session",
	Long: `Applies changes to .crannog/ to the crannog of a branch, by default the
checked-out one, without opening a session.

New content needs your approval. If only iron-proxy.yaml changed, crannog
reloads the proxy and running sessions continue with the new rules.
Otherwise it rebuilds the crannog, which ends running sessions; it asks
before that. A rebuilt crannog starts with the next session.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := crannog.New(newUI(applyYes))
		if err != nil {
			return err
		}
		return m.Apply(firstArg(args))
	},
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func init() {
	addYesFlag(applyCmd, &applyYes)
	rootCmd.AddCommand(applyCmd)
}
