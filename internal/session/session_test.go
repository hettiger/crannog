package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	l, err := TryLock(path)
	require.NoError(t, err)

	// flock locks belong to the open file, so a second open in the same
	// process conflicts like another process would.
	_, err = TryLock(path)
	assert.ErrorIs(t, err, ErrLocked)

	require.NoError(t, l.Unlock())
	l2, err := TryLock(path)
	require.NoError(t, err)
	require.NoError(t, l2.Unlock())
	assert.NoError(t, (*Lock)(nil).Unlock())
}

func TestLockWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	l, err := TryLock(path)
	require.NoError(t, err)

	done := make(chan *Lock)
	waited := make(chan bool, 1)
	go func() {
		l2, err := LockWait(path, func() { waited <- true })
		assert.NoError(t, err)
		done <- l2
	}()
	assert.True(t, <-waited)
	require.NoError(t, l.Unlock())
	l2 := <-done
	require.NoError(t, l2.Unlock())
}

func TestLockRefusesSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, nil, 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "lock")))
	_, err := TryLock(filepath.Join(dir, "lock"))
	assert.Error(t, err)
}

func TestRegisterAndCount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")

	n, err := Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "missing dir")

	l, path, err := Register(dir)
	require.NoError(t, err)
	assert.FileExists(t, path)

	// A session that ended without cleaning up.
	stale := filepath.Join(dir, "99999999")
	require.NoError(t, os.WriteFile(stale, nil, 0o644))

	n, err = Count(dir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.FileExists(t, stale, "kept without prune")

	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.NoFileExists(t, stale, "pruned")
	assert.FileExists(t, path, "running session kept")

	require.NoError(t, l.Unlock())
	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// A child process that inherits the session file keeps the session
// running after the parent released its descriptor (ADR 0025).
func TestChildKeepsTheSession(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	l, _, err := Register(dir)
	require.NoError(t, err)

	cmd := exec.Command("cat")
	cmd.ExtraFiles = []*os.File{l.File()}
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	require.NoError(t, l.Unlock())

	n, err := Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the child holds the lock")

	require.NoError(t, stdin.Close())
	require.NoError(t, cmd.Wait())
	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
