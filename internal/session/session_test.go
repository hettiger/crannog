package session

import (
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

func TestTryLock(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	l, err := TryLock(dir, "lock")
	require.NoError(t, err)

	// flock locks belong to the open file, so a second open in the same
	// process conflicts like another process would.
	_, err = TryLock(dir, "lock")
	assert.ErrorIs(t, err, ErrLocked)

	require.NoError(t, l.Unlock())
	l2, err := TryLock(dir, "lock")
	require.NoError(t, err)
	require.NoError(t, l2.Unlock())
	assert.NoError(t, (*Lock)(nil).Unlock())
}

func TestLockWait(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	l, err := TryLock(dir, "lock")
	require.NoError(t, err)

	done := make(chan *Lock)
	waited := make(chan bool, 1)
	go func() {
		l2, err := LockWait(dir, "lock", func() { waited <- true })
		assert.NoError(t, err)
		done <- l2
	}()
	assert.True(t, <-waited)
	require.NoError(t, l.Unlock())
	l2 := <-done
	require.NoError(t, l2.Unlock())
}

func TestLockRefusesSymlinks(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	testfs.WriteFile(t, dir, "target", "", 0o644)
	require.NoError(t, dir.Symlink("target", "lock"))
	_, err := TryLock(dir, "lock")
	assert.ErrorContains(t, err, "not a regular file")
	require.NoError(t, dir.Mkdir("dir", 0o755))
	_, err = TryLock(dir, "dir")
	assert.Error(t, err)
}

func TestRegisterAndCount(t *testing.T) {
	dir, _ := testfs.TempRoot(t)

	n, err := Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "empty dir")

	l, name, err := Register(dir)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(os.Getpid()), name)
	assert.True(t, testfs.Exists(t, dir, name))

	// A session that ended without cleaning up.
	testfs.WriteFile(t, dir, "99999999", "", 0o644)
	// Files not named like a process id are no session files.
	testfs.WriteFile(t, dir, "notes.txt", "", 0o644)
	testfs.WriteFile(t, dir, "0123", "", 0o644)

	n, err = Count(dir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, testfs.Exists(t, dir, "99999999"), "kept without prune")

	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.False(t, testfs.Exists(t, dir, "99999999"), "pruned")
	assert.True(t, testfs.Exists(t, dir, name), "running session kept")
	assert.True(t, testfs.Exists(t, dir, "notes.txt"), "no session file, kept")
	assert.True(t, testfs.Exists(t, dir, "0123"), "no session file, kept")

	require.NoError(t, l.Unlock())
	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// A child process that inherits the session file keeps the session
// running after the parent released its descriptor (ADR 0025).
func TestChildKeepsTheSession(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
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
