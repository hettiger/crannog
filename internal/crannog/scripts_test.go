package crannog

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testgit"
)

// The scripts crannog runs in a crannog, run against a local repository.

func runScript(t *testing.T, script string, args ...string) string {
	out, err := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func TestAuthorScript(t *testing.T) {
	dir := testgit.Init(t)
	runScript(t, authorScript, dir, "set", "Ada 'Lovelace'", "set", "ada@example.com")
	assert.Equal(t, "Ada 'Lovelace'", testgit.Git(t, dir, "config", "user.name"))
	assert.Equal(t, "ada@example.com", testgit.Git(t, dir, "config", "user.email"))

	runScript(t, authorScript, dir, "unset", "", "set", "new@example.com")
	cmd := exec.Command("git", "-C", dir, "config", "user.name")
	assert.Error(t, cmd.Run(), "removed")
	assert.Equal(t, "new@example.com", testgit.Git(t, dir, "config", "user.email"))

	runScript(t, authorScript, dir, "unset", "", "unset", "")
}
