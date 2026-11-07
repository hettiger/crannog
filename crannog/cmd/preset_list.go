package cmd

import (
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/preset"
)

var presetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all presets, bundled and user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		h, err := home.Default()
		if err != nil {
			return err
		}
		list, err := preset.List(h)
		if err != nil {
			return err
		}
		t := newTable("NAME", "SOURCE", "PATH")
		for _, p := range list {
			t.Row(p.Name, p.Source(), p.Dir)
		}
		_, err = lipgloss.Fprintln(cmd.OutOrStdout(), t)
		return err
	},
}

func init() {
	presetCmd.AddCommand(presetListCmd)
}
