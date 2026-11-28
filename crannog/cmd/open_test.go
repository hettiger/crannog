package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Flags of open that do not go together, and invalid ports, fail before
// anything runs.
func TestOpenFlags(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"open", "--zed", "--insecure"}, "[insecure zed] were all set"},
		{[]string{"open", "--zed", "--port", "8080"}, "[port zed] were all set"},
		{[]string{"open", "--port=0"}, "--port must be between 1 and 65535, not 0"},
		{[]string{"open", "--port", "65536"}, "--port must be between 1 and 65535, not 65536"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.ErrorContains(t, execute(t, tt.args...), tt.want)
		})
	}
}
