package shell

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoin(t *testing.T) {
	assert.Equal(t, `claude -p 'say hi'`, Join([]string{"claude", "-p", "say hi"}))
	assert.Equal(t, `'' 'it'\''s' '$HOME' 'a;b' --x=1`, Join([]string{"", "it's", "$HOME", "a;b", "--x=1"}))
}

// The shell receives exactly the original arguments.
func TestJoinRoundTrip(t *testing.T) {
	args := []string{"say hi", "it's", "$HOME", "`id`", "a\nb", "*", "", "ü", `\`, `"`}
	script := `for a in "$@"; do printf '%s\0' "$a"; done`
	out, err := exec.Command("sh", "-c", "set -- "+Join(args)+"; "+script).Output()
	require.NoError(t, err)
	var got []string
	start := 0
	for i, b := range out {
		if b == 0 {
			got = append(got, string(out[start:i]))
			start = i + 1
		}
	}
	assert.Equal(t, args, got)
}
