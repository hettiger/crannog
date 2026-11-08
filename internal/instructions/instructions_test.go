package instructions

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/untrusted"
)

func TestUpdate(t *testing.T) {
	b := Begin + "\nnew\n" + End
	tests := map[string]string{
		"":         b + "\n",
		"mine":     "mine\n\n" + b + "\n",
		"mine\n":   "mine\n\n" + b + "\n",
		"mine\n\n": "mine\n\n" + b + "\n",
		"a\n" + Begin + "\nold\n" + End + "\nb\n": "a\n" + b + "\nb\n",
		Begin + "\nold\n" + End:                   b,
		// An unterminated block is left alone, a new one is appended.
		"a\n" + Begin + "\nold\n": "a\n" + Begin + "\nold\n\n" + b + "\n",
	}
	for in, want := range tests {
		assert.Equal(t, want, Update(in, b), "%q", in)
	}
	assert.Equal(t, Update("x", b), Update(Update("x", b), b), "idempotent")
}

func TestBlockIsMarked(t *testing.T) {
	assert.True(t, strings.HasPrefix(ManagedBlock, Begin+"\n"))
	assert.True(t, strings.HasSuffix(ManagedBlock, "\n"+End))
	assert.Equal(t, 1, strings.Count(ManagedBlock, Begin))
}

// agentsDir returns a temporary directory with an agents directory and a
// host directory next to it, standing for files outside ~/.crannog.
func agentsDir(t *testing.T) (tmp, agents *os.Root) {
	tmp, _ = testfs.TempRoot(t)
	require.NoError(t, tmp.Mkdir("agents", 0o755))
	require.NoError(t, tmp.Mkdir("host", 0o755))
	testfs.WriteFile(t, tmp, "host/CLAUDE.md", "host", 0o600)
	agents, err := tmp.OpenRoot("agents")
	require.NoError(t, err)
	t.Cleanup(func() { agents.Close() })
	return tmp, agents
}

// hostUntouched checks that the host directory holds its file only.
func hostUntouched(t *testing.T, tmp *os.Root) {
	assert.Equal(t, "host", testfs.ReadFile(t, tmp, "host/CLAUDE.md"), "host file untouched")
	entries, err := fs.ReadDir(tmp.FS(), "host")
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file on the host")
}

func TestWrite(t *testing.T) {
	_, agents := agentsDir(t)
	testfs.WriteFile(t, agents, "claude/CLAUDE.md", "# My rules\n", 0o600)

	require.NoError(t, Write(agents))
	for _, a := range Agents {
		assert.Contains(t, testfs.ReadFile(t, agents, filepath.Join(a.Name, a.File)), ManagedBlock, a.Name)
	}
	data := testfs.ReadFile(t, agents, "claude/CLAUDE.md")
	assert.True(t, strings.HasPrefix(data, "# My rules\n\n"+Begin))
	info, err := agents.Stat("claude/CLAUDE.md")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "mode kept")

	require.NoError(t, Write(agents), "second write")
	assert.Equal(t, data, testfs.ReadFile(t, agents, "claude/CLAUDE.md"))
}

func TestWriteRefusesSymlinks(t *testing.T) {
	tmp, agents := agentsDir(t)
	require.NoError(t, EnsureHomes(agents))
	require.NoError(t, agents.Symlink("../../host/CLAUDE.md", "codex/AGENTS.md"))
	assert.ErrorContains(t, Write(agents), "not a regular file")
	hostUntouched(t, tmp)

	// A symlinked agent home.
	tmp, agents = agentsDir(t)
	require.NoError(t, agents.Symlink(filepath.Join(tmp.Name(), "host"), "pi"))
	assert.ErrorContains(t, EnsureHomes(agents), "not a directory")
	hostUntouched(t, tmp)
}

// A crannog can replace an agent home or an instruction file with a symlink
// after EnsureHomes and Lstat checked them. The os.Root must still refuse
// to leave the agent home.
func TestWriteStaysInTheAgentHome(t *testing.T) {
	tmp, agents := agentsDir(t)
	require.NoError(t, EnsureHomes(agents))
	claude := Agents[0]

	// The agent home replaced by a symlink to a host directory.
	require.NoError(t, agents.Remove("claude"))
	require.NoError(t, agents.Symlink(filepath.Join(tmp.Name(), "host"), "claude"))
	assert.Error(t, write(agents, claude))

	// The instruction file replaced by a symlink to a host file.
	require.NoError(t, agents.Remove("claude"))
	require.NoError(t, agents.Mkdir("claude", 0o755))
	home, err := agents.OpenRoot("claude")
	require.NoError(t, err)
	defer home.Close()
	require.NoError(t, home.Symlink(filepath.Join(tmp.Name(), "host", "CLAUDE.md"), "CLAUDE.md"))
	_, _, err = untrusted.ReadFile(home, "CLAUDE.md", maxFileSize)
	assert.Error(t, err, "the read after Lstat")

	hostUntouched(t, tmp)
}

func TestWriteRefusesHugeFiles(t *testing.T) {
	_, agents := agentsDir(t)
	require.NoError(t, EnsureHomes(agents))
	require.NoError(t, agents.WriteFile("pi/AGENTS.md", make([]byte, maxFileSize+1), 0o644))
	assert.ErrorContains(t, Write(agents), "larger than")
}
