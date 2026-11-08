package session

import (
	"os"
	"os/exec"
	"path/filepath"
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

// setAfterLock replaces the lock file in the test hook between the flock
// and the check.
func setAfterLock(t *testing.T, f func(dir *os.Root, name string)) {
	t.Cleanup(func() { afterLock = func(*os.Root, string) {} })
	afterLock = f
}

func TestLockRefusesARemovedOrReplacedFile(t *testing.T) {
	for _, recreate := range []bool{false, true} {
		dir, _ := testfs.TempRoot(t)
		setAfterLock(t, func(dir *os.Root, name string) {
			require.NoError(t, dir.Remove(name))
			if recreate {
				testfs.WriteFile(t, dir, name, "", 0o644)
			}
		})
		_, err := TryLock(dir, "lock")
		assert.ErrorIs(t, err, ErrReplaced, "recreate %v", recreate)

		// The stale lock was released; the current file can be locked.
		afterLock = func(*os.Root, string) {}
		l, err := TryLock(dir, "lock")
		require.NoError(t, err)
		require.NoError(t, l.Unlock())
	}
}

// A process waiting for the lock opened the lock file before destroy
// removed the crannog's directory with it. Another process then locks the
// recreated file. The waiter must not hold the lock on the old file too.
func TestLockWaiterAfterDirectoryRemoval(t *testing.T) {
	parent, _ := testfs.TempRoot(t)
	require.NoError(t, parent.Mkdir("crannog", 0o755))
	old := testfs.Root(t, filepath.Join(parent.Name(), "crannog"))
	holder, err := TryLock(old, "lock")
	require.NoError(t, err)
	waiter, err := open(old, "lock", true)
	require.NoError(t, err)

	require.NoError(t, parent.RemoveAll("crannog"))
	require.NoError(t, holder.Unlock())
	require.NoError(t, parent.Mkdir("crannog", 0o755))
	current := testfs.Root(t, filepath.Join(parent.Name(), "crannog"))
	other, err := TryLock(current, "lock")
	require.NoError(t, err)
	defer other.Unlock()

	// flock succeeds on the removed file although another process holds
	// the lock on the current one.
	require.NoError(t, flockRetry(waiter))
	assert.ErrorIs(t, checkLocked(old, "lock", waiter), ErrReplaced)
	assert.ErrorIs(t, waiter.Close(), os.ErrClosed, "closed by the check")
}

func TestCountSkipsRemovedSessionFiles(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	testfs.WriteFile(t, dir, "99999999", "", 0o644)
	// The session ends and removes its file while Count looks at it.
	setAfterLock(t, func(dir *os.Root, name string) { require.NoError(t, dir.Remove(name)) })
	n, err := Count(dir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, testfs.Exists(t, dir, "99999999"))

	// A session file removed before Count opens it is not recreated.
	_, err = tryLock(dir, "99999999", false)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.False(t, testfs.Exists(t, dir, "99999999"))
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
// running after the parent released its descriptor (ADR 0019).
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
