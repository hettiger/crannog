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
	c.Flags().BoolP("yes", "y", false, "")
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
		{[]string{"a", "b"}, 2, false},
	}
	for _, tt := range tests {
		c, args := parse(t, tt.args...)
		assert.Equal(t, tt.branches, branchArgs(c, args), tt.args)
		assert.Equal(t, tt.valid, checkoutCmd.Args(c, args) == nil, tt.args)
	}

	c, args := parse(t, "feature/x", "--", "claude", "-p", "say hi")
	assert.Equal(t, []string{"claude", "-p", "say hi"}, args[branchArgs(c, args):], "flags after -- belong to the command")
}
