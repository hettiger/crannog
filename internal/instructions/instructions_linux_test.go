package instructions

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A crannog replaces the agent home by a symlink to a host directory while
// Write reads the instruction file. inotify schedules the replacement right
// after the open.
func TestWriteRacesWithAReplacedAgentHome(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agents")
	require.NoError(t, EnsureHomes(dir))
	claude := filepath.Join(dir, "claude")
	// Large enough to read slower than the replacement.
	require.NoError(t, os.WriteFile(filepath.Join(claude, "CLAUDE.md"), bytes.Repeat([]byte("x"), maxFileSize-len(ManagedBlock)-3), 0o600))
	host := t.TempDir()
	hostFile := filepath.Join(host, "CLAUDE.md")
	require.NoError(t, os.WriteFile(hostFile, []byte("host"), 0o600))

	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	require.NoError(t, err)
	defer syscall.Close(fd)
	_, err = syscall.InotifyAddWatch(fd, filepath.Join(claude, "CLAUDE.md"), syscall.IN_OPEN)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		if _, err := syscall.Read(fd, buf); err != nil {
			done <- err
			return
		}
		if err := os.Rename(claude, claude+"-moved"); err != nil {
			done <- err
			return
		}
		done <- os.Symlink(host, claude)
	}()

	require.NoError(t, Write(dir))
	require.NoError(t, <-done)
	data, err := os.ReadFile(hostFile)
	require.NoError(t, err)
	assert.Equal(t, "host", string(data), "host file untouched")
	moved, err := os.ReadFile(filepath.Join(claude+"-moved", "CLAUDE.md"))
	require.NoError(t, err)
	assert.Contains(t, string(moved), ManagedBlock, "written into the agent home it opened")
}
