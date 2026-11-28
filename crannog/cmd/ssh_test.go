package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Flags of ssh that do not go together fail before anything runs.
func TestSSHFlags(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
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
