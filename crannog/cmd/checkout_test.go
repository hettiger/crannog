package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parse parses args like Cobra does for checkout.
func parse(t *testing.T, args ...string) (*cobra.Command, []string) {
	c := &cobra.Command{Use: "checkout"}
	c.Flags().AddFlagSet(checkoutCmd.Flags())
	require.NoError(t, c.ParseFlags(args))
	return c, c.Flags().Args()
}

func TestCheckoutArgs(t *testing.T) {
	tests := []struct {
		args     []string
		branches int
		valid    bool
	}{
		{nil, 0, true},
		{[]string{"feature/x"}, 1, true},
		{[]string{"feature/x", "--", "claude", "-p", "say hi"}, 1, true},
		{[]string{"--", "claude", "-p", "say hi"}, 0, true},
		{[]string{"-y", "--", "zsh"}, 0, true},
		{[]string{"-d"}, 0, true},
		{[]string{"-d", "feature/x", "--", "npm", "run", "dev"}, 1, true},
		{[]string{"a", "b"}, 2, false},
	}
	for _, tt := range tests {
		c, args := parse(t, tt.args...)
		assert.Equal(t, tt.branches, branchArgs(c, args), tt.args)
		assert.Equal(t, tt.valid, checkoutCmd.Args(c, args) == nil, tt.args)
	}

	c, args := parse(t, "feature/x", "--", "claude", "-p", "say hi")
	assert.Equal(t, []string{"claude", "-p", "say hi"}, args[branchArgs(c, args):], "flags after -- belong to the command")
	branch, command := splitBranchAndCommand(c, args)
	assert.Equal(t, "feature/x", branch)
	assert.Equal(t, []string{"claude", "-p", "say hi"}, command)

	c, args = parse(t)
	branch, command = splitBranchAndCommand(c, args)
	assert.Empty(t, branch)
	assert.Nil(t, command, "no command without --")

	c, args = parse(t, "--")
	_, command = splitBranchAndCommand(c, args)
	assert.Equal(t, []string{}, command, "-- without a command")
}

// crannog ssh takes its arguments like checkout.
func TestSSHArgs(t *testing.T) {
	c := &cobra.Command{Use: "ssh"}
	require.NoError(t, c.ParseFlags([]string{"feature/x", "--", "ls", "-la"}))
	assert.NoError(t, sshCmd.Args(c, c.Flags().Args()))
	branch, command := splitBranchAndCommand(c, c.Flags().Args())
	assert.Equal(t, "feature/x", branch)
	assert.Equal(t, []string{"ls", "-la"}, command)

	c = &cobra.Command{Use: "ssh"}
	require.NoError(t, c.ParseFlags([]string{"a", "b"}))
	assert.Error(t, sshCmd.Args(c, c.Flags().Args()))
}
