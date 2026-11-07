package instructions

import (
	"bytes"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// A crannog replaces the agent home by a symlink to a host directory while
// Write reads the instruction file. inotify schedules the replacement right
// after the open.
func TestWriteRacesWithAReplacedAgentHome(t *testing.T) {
	tmp, agents := agentsDir(t)
	require.NoError(t, EnsureHomes(agents))
	// Large enough to read slower than the replacement.
	require.NoError(t, agents.WriteFile("claude/CLAUDE.md", bytes.Repeat([]byte("x"), maxFileSize-len(ManagedBlock)-3), 0o600))

	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	require.NoError(t, err)
	defer syscall.Close(fd)
	_, err = syscall.InotifyAddWatch(fd, filepath.Join(agents.Name(), "claude", "CLAUDE.md"), syscall.IN_OPEN)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		if _, err := syscall.Read(fd, buf); err != nil {
			done <- err
			return
		}
		if err := agents.Rename("claude", "claude-moved"); err != nil {
			done <- err
			return
		}
		done <- agents.Symlink(filepath.Join(tmp.Name(), "host"), "claude")
	}()

	require.NoError(t, Write(agents))
	require.NoError(t, <-done)
	hostUntouched(t, tmp)
	assert.Contains(t, testfs.ReadFile(t, agents, "claude-moved/CLAUDE.md"), ManagedBlock, "written into the agent home it opened")
}
