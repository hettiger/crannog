package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testgit"
)

// complete runs Cobra's hidden completion command for a command whose
// first argument completes from list, and returns the candidates.
func complete(t *testing.T, list func() ([]string, error), args ...string) []string {
	t.Helper()
	root := &cobra.Command{Use: "crannog"}
	c := &cobra.Command{
		Use:               "stop",
		Args:              branchAndCommandArgs,
		ValidArgsFunction: completeFirstArg(list),
		Run:               func(*cobra.Command, []string) {},
	}
	c.Flags().BoolP("yes", "y", false, "")
	root.AddCommand(c)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	// Like the shell scripts, which run crannog __complete <words...>.
	saved := os.Args
	os.Args = append([]string{"crannog", cobra.ShellCompRequestCmd, "stop"}, args...)
	t.Cleanup(func() { os.Args = saved })
	root.SetArgs(os.Args[1:])
	require.NoError(t, root.Execute())

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	directive := lines[len(lines)-1]
	assert.Equal(t, ":4", directive, "no file completion")
	return lines[:len(lines)-1]
}

func TestCompleteFirstArg(t *testing.T) {
	branches := func() ([]string, error) {
		return []string{"feature/a", "feature/b", "main"}, nil
	}
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{""}, []string{"feature/a", "feature/b", "main"}},
		{[]string{"fe"}, []string{"feature/a", "feature/b"}},
		{[]string{"feature/a"}, []string{"feature/a"}},
		{[]string{"x"}, []string{}},
		{[]string{"-y", ""}, []string{"feature/a", "feature/b", "main"}},
		{[]string{"main", ""}, []string{}},
		{[]string{"--", ""}, []string{}},
		{[]string{"main", "--", ""}, []string{}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, complete(t, branches, tt.args...), tt.args)
	}

	failing := func() ([]string, error) { return nil, errors.New("not a git repository") }
	assert.Equal(t, []string{}, complete(t, failing, ""), "an error offers nothing")
}

// The commands complete from the repository and the presets.
func TestCommandCompletion(t *testing.T) {
	dir := testgit.Init(t)
	testgit.Git(t, dir, "branch", "feature/x")
	t.Chdir(dir)
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	run := func(words ...string) []string {
		var out bytes.Buffer
		rootCmd.SetOut(&out)
		rootCmd.SetErr(io.Discard)
		saved := os.Args
		os.Args = append([]string{"crannog", cobra.ShellCompRequestCmd}, words...)
		defer func() { os.Args = saved }()
		rootCmd.SetArgs(os.Args[1:])
		require.NoError(t, rootCmd.Execute())
		lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
		return lines[:len(lines)-1]
	}
	assert.Equal(t, []string{"feature/x", "main"}, run("checkout", ""))
	assert.Equal(t, []string{"feature/x"}, run("checkout", "f"))
	assert.Contains(t, run("preset", "publish", ""), "default")
	assert.Contains(t, run("use", ""), "zed\tOpen the running crannog of a branch in Zed")
}
