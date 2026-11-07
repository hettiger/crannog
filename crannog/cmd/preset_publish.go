package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/preset"
	"github.com/hettiger/crannog/internal/repository"
	"github.com/hettiger/crannog/internal/setup"
)

var presetPublishYes bool

var presetPublishCmd = &cobra.Command{
	Use:   "publish [name]",
	Short: "Write a preset to .crannog/",
	Long: `Writes a preset to .crannog/ in the current worktree, so the repository
can change and version its own setup. Without a name, crannog lets you pick
one. It asks before replacing an existing .crannog/.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		u := newUI(presetPublishYes)
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		repo, err := repository.Open(cwd)
		if err != nil {
			return err
		}
		h, err := home.Default()
		if err != nil {
			return err
		}

		name := firstArg(args)
		if name == "" {
			list, err := preset.List(h)
			if err != nil {
				return err
			}
			var names []string
			for _, p := range list {
				names = append(names, p.Name)
			}
			if name, err = u.PickOne("Preset to publish", "the preset name", names); err != nil {
				return err
			}
		}
		p, err := preset.Find(h, name)
		if err != nil {
			return err
		}
		s, err := p.Load()
		if err != nil {
			return err
		}

		target := filepath.Join(repo.Dir, setup.Dir)
		present, err := setup.Present(repo.Dir)
		if err != nil {
			return err
		}
		if present {
			if err := u.AskConfirmation(fmt.Sprintf("Replace %s?", target)); err != nil {
				return err
			}
		}
		if err := preset.Publish(s, repo.Dir); err != nil {
			return err
		}
		u.Printf("Wrote preset %s to %s\n", name, target)
		return nil
	},
}

func init() {
	addYesFlag(presetPublishCmd, &presetPublishYes)
	presetCmd.AddCommand(presetPublishCmd)
}
