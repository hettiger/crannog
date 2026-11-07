package instructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestWrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "claude", "CLAUDE.md"), []byte("# My rules\n"), 0o600))

	require.NoError(t, Write(dir))
	for _, a := range Agents {
		data, err := os.ReadFile(filepath.Join(dir, a.Name, a.File))
		require.NoError(t, err, a.Name)
		assert.Contains(t, string(data), ManagedBlock, a.Name)
	}
	data, err := os.ReadFile(filepath.Join(dir, "claude", "CLAUDE.md"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(data), "# My rules\n\n"+Begin))
	info, err := os.Stat(filepath.Join(dir, "claude", "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "mode kept")

	require.NoError(t, Write(dir), "second write")
	again, err := os.ReadFile(filepath.Join(dir, "claude", "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, data, again)
}

func TestWriteRefusesSymlinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, EnsureHomes(dir))
	target := filepath.Join(t.TempDir(), "host-file")
	require.NoError(t, os.WriteFile(target, []byte("host"), 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "codex", "AGENTS.md")))

	assert.ErrorContains(t, Write(dir), "not a regular file")
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "host", string(data), "target untouched")

	// A symlinked agent home.
	dir2 := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, os.MkdirAll(dir2, 0o755))
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(dir2, "pi")))
	assert.ErrorContains(t, EnsureHomes(dir2), "not a directory")
}

// A crannog can replace an agent home or an instruction file with a symlink
// after EnsureHomes and Lstat checked them. The os.Root must still refuse
// to leave the agent home.
func TestWriteStaysInTheAgentHome(t *testing.T) {
	host := t.TempDir()
	hostFile := filepath.Join(host, "CLAUDE.md")
	require.NoError(t, os.WriteFile(hostFile, []byte("host"), 0o600))

	dir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, EnsureHomes(dir))
	agents, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer agents.Close()
	claude := Agents[0]

	// The agent home replaced by a symlink to a host directory.
	require.NoError(t, os.Remove(filepath.Join(dir, "claude")))
	require.NoError(t, os.Symlink(host, filepath.Join(dir, "claude")))
	assert.Error(t, write(agents, claude))

	// The instruction file replaced by a symlink to a host file.
	require.NoError(t, os.Remove(filepath.Join(dir, "claude")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "claude"), 0o755))
	home, err := agents.OpenRoot("claude")
	require.NoError(t, err)
	defer home.Close()
	require.NoError(t, os.Symlink(hostFile, filepath.Join(dir, "claude", "CLAUDE.md")))
	_, _, err = untrusted.ReadFile(home, "CLAUDE.md", maxFileSize)
	assert.Error(t, err, "the read after Lstat")

	data, err := os.ReadFile(hostFile)
	require.NoError(t, err)
	assert.Equal(t, "host", string(data), "host file untouched")
	entries, err := os.ReadDir(host)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file on the host")
}

func TestWriteRefusesHugeFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, EnsureHomes(dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pi", "AGENTS.md"), make([]byte, maxFileSize+1), 0o644))
	assert.ErrorContains(t, Write(dir), "larger than")
}
