package cmd

import (
	"fmt"
	"strconv"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the repository's crannogs",
	Long:  `Lists the crannogs of the current repository: start branch, status and running sessions.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := crannog.New(newUI(false))
		if err != nil {
			return err
		}
		list, err := m.List()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "No crannogs in this repository. Start one with crannog checkout.")
			return nil
		}
		t := newTable("BRANCH", "STATUS", "SESSIONS")
		for _, c := range list {
			t.Row(c.Branch, c.Status, strconv.Itoa(c.Sessions))
		}
		_, err = lipgloss.Fprintln(cmd.OutOrStdout(), t)
		return err
	},
}

// newTable returns a borderless table with the given headers.
func newTable(headers ...string) *table.Table {
	return table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderHeader(false).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().PaddingRight(2)
			if row == table.HeaderRow {
				return s.Bold(true)
			}
			return s
		})
}

func init() {
	rootCmd.AddCommand(listCmd)
}
