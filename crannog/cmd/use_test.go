package cmd

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// crannog use names its tools when the tool is unknown.
func TestUseUnknownTool(t *testing.T) {
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"use", "foo"})
	assert.EqualError(t, rootCmd.Execute(), `unknown tool "foo"; crannog use supports: zed`)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"use"})
	assert.NoError(t, rootCmd.Execute())
	assert.Contains(t, out.String(), "Include ~/.crannog/ssh/config", "without a tool, the help")
}
