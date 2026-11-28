package cmd

import (
	"io"
	"testing"
)

// execute runs crannog with args and returns its error. Flags keep their
// values between runs, so it resets those of open and ssh afterwards.
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
