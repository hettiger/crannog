package testgit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElsewhere(t *testing.T) {
	for _, args := range [][]string{
		{"-C", "/x", "status"},
		{"--git-dir=/x", "status"},
		{"--git-dir", "/x", "status"},
		{"--work-tree=/x", "status"},
		{"-c", "a=b", "--no-pager", "-C", "/x", "status"},
	} {
		assert.NotEmpty(t, elsewhere(args), "%v", args)
	}
	for _, args := range [][]string{
		{"status"},
		{"commit", "-C", "HEAD"},
		{"-c", "-C", "status"},
		{"-c", "a=b", "config", "--git-dir"},
		{"--no-pager", "log"},
	} {
		assert.Empty(t, elsewhere(args), "%v", args)
	}
}
