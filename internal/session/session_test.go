package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// ended is the name of the session file of a session that has ended.
const ended = "0193a5b2-58a0-7c5e-9d0b-3a1f0c2b7e11"

// upperCase is a UUID as Register never writes it. It differs from ended
// in more than its case: a Mac's file system would take them for the same
// file.
const upperCase = "0193A5B2-7C44-7D1E-8F3A-5B6C7D8E9F01"

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
	testfs.WriteFile(t, dir, ended, "", 0o644)
	// The session ends and removes its file while Count looks at it.
	setAfterLock(t, func(dir *os.Root, name string) { require.NoError(t, dir.Remove(name)) })
	n, err := Count(dir, false)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, testfs.Exists(t, dir, ended))

	// A session file removed before Count opens it is not recreated.
	_, err = tryLock(dir, ended, false, syscall.LOCK_SH)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.False(t, testfs.Exists(t, dir, ended))
}

// crannog list counts without the crannog lock. While it looks at the file
// of an ended session, another count must not take that for a running
// session.
func TestCountsDoNotSeeEachOther(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	testfs.WriteFile(t, dir, ended, "", 0o644)
	looking, err := tryLock(dir, ended, false, syscall.LOCK_SH)
	require.NoError(t, err)
	defer looking.Unlock()

	n, err := Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.False(t, testfs.Exists(t, dir, ended), "pruned")
}

// A count that looks at a session file while it is registered only delays
// the registration.
func TestRegisterWaitsForACount(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	name := ended
	newName = func() string { return name }
	t.Cleanup(func() { newName = func() string { return uuid.NewV7().String() } })
	testfs.WriteFile(t, dir, name, "", 0o644)
	looking, err := tryLock(dir, name, false, syscall.LOCK_SH)
	require.NoError(t, err)
	go func() {
		time.Sleep(2 * registerRetry)
		looking.Unlock()
	}()

	l, _, err := Register(dir, Info{})
	require.NoError(t, err)
	require.NoError(t, l.Unlock())
}

func TestRegisterAndCount(t *testing.T) {
	dir, _ := testfs.TempRoot(t)

	n, err := Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "empty dir")

	info := Info{Command: "npm run 'dev server'", Detached: true, StartedAt: time.Date(2026, 11, 23, 14, 2, 11, 0, time.FixedZone("", 3600))}
	l, name, err := Register(dir, info)
	require.NoError(t, err)
	assert.Equal(t, "command: npm run 'dev server'\ndetached: true\nstarted_at: 2026-11-23T14:02:11+01:00\n",
		testfs.ReadFile(t, dir, name), "the file says what the session runs")
	_, err = uuid.Parse(name)
	assert.NoError(t, err, "named with a UUID")
	second, other, err := Register(dir, Info{})
	require.NoError(t, err)
	assert.NotEqual(t, name, other, "one process can register several sessions")
	require.NoError(t, second.Unlock())
	require.NoError(t, dir.Remove(other))
	assert.True(t, testfs.Exists(t, dir, name))

	// A session that ended without cleaning up.
	testfs.WriteFile(t, dir, ended, "", 0o644)
	// Files not named like a session file are none.
	testfs.WriteFile(t, dir, "notes.txt", "", 0o644)
	testfs.WriteFile(t, dir, "0123", "", 0o644)
	testfs.WriteFile(t, dir, upperCase, "", 0o644)

	n, err = Count(dir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, testfs.Exists(t, dir, ended), "kept without prune")

	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.False(t, testfs.Exists(t, dir, ended), "pruned")
	assert.True(t, testfs.Exists(t, dir, name), "running session kept")
	assert.True(t, testfs.Exists(t, dir, "notes.txt"), "no session file, kept")
	assert.True(t, testfs.Exists(t, dir, "0123"), "no session file, kept")
	assert.True(t, testfs.Exists(t, dir, upperCase), "no session file, kept")

	require.NoError(t, l.Unlock())
	n, err = Count(dir, true)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// A child process that inherits the session file keeps the session
// running after the parent released its descriptor (ADR 0019).
func TestChildKeepsTheSession(t *testing.T) {
	dir, _ := testfs.TempRoot(t)
	l, _, err := Register(dir, Info{})
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
