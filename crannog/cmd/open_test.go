package cmd

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// execute runs crannog with args and returns its error. Flags keep their
// values between runs, so each case sets what it needs.
func execute(t *testing.T, args ...string) error {
	t.Helper()
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		openZed, openInsecure, openPort = false, false, 0
		sshHost, sshPath = false, false
		for _, c := range []string{"zed", "insecure", "port"} {
			openCmd.Flags().Lookup(c).Changed = false
		}
		for _, c := range []string{"host", "path"} {
			sshCmd.Flags().Lookup(c).Changed = false
		}
	})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// Flags that do not go together fail before anything runs.
func TestFlagsThatDoNotGoTogether(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"open", "--zed", "--insecure"}, "[insecure zed] were all set"},
		{[]string{"open", "--zed", "--port", "8080"}, "[port zed] were all set"},
		{[]string{"open", "--port=0"}, "--port must be between 1 and 65535, not 0"},
		{[]string{"open", "--port", "65536"}, "--port must be between 1 and 65535, not 65536"},
		{[]string{"ssh", "--host", "--path"}, "[host path] were all set"},
		{[]string{"ssh", "--host", "--", "ls"}, "--host and --path take no command after --"},
		{[]string{"ssh", "main", "--path", "--", "ls"}, "--host and --path take no command after --"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.ErrorContains(t, execute(t, tt.args...), tt.want)
		})
	}
}
