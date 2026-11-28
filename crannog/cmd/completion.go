package cmd

import (
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hettiger/crannog/internal/crannog"
	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/preset"
	"github.com/hettiger/crannog/internal/repository"
)

// completeFirstArg completes a command's first argument with the
// candidates of list that start with what is typed (ADR 0027). After the
// first argument and after -- it offers nothing, not even files; an error
// offers nothing either.
func completeFirstArg(list func() ([]string, error)) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 || dashTyped() {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		candidates, err := list()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var matches []cobra.Completion
		for _, c := range candidates {
			if strings.HasPrefix(c, toComplete) {
				matches = append(matches, c)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

// dashTyped reports whether -- was typed before the word being completed.
// During completion, ArgsLenAtDash cannot tell: Cobra first parses the
// arguments with a -- appended, and pflag keeps that position. Cobra's
// completion command gets the typed words as arguments, the word being
// completed last.
func dashTyped() bool {
	if len(os.Args) < 2 {
		return false
	}
	typed := os.Args[1:]
	return slices.Contains(typed[:len(typed)-1], "--")
}

// localBranches lists the repository's local branches, for checkout,
// which builds a missing crannog.
func localBranches() ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	repo, err := repository.Open(cwd)
	if err != nil {
		return nil, err
	}
	return repo.Branches()
}

// startBranches lists the start branches of the repository's crannogs,
// running or not, for the commands that need a crannog.
func startBranches() ([]string, error) {
	m, err := crannog.New(newUI(false))
	if err != nil {
		return nil, err
	}
	return m.StartBranches()
}

// presetNames lists the presets, bundled and user, as the picker of
// preset publish does.
func presetNames() ([]string, error) {
	h, err := home.OpenCrannogHome()
	if err != nil {
		return nil, err
	}
	defer h.Close()
	list, err := preset.List(h)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list))
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names, nil
}
