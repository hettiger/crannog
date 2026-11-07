// Package session implements the crannog lock and the session files with
// flock (ADR 0014, ADR 0025). The kernel releases a lock when the last
// process holding its file descriptor ends, so locks never stay behind.
// Lock and session files are accessed within an os.Root (ADR 0032).
package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"syscall"
)

// Lock is an exclusive flock on a file.
type Lock struct {
	f *os.File
}

// ErrLocked is returned by TryLock if another process holds the lock.
var ErrLocked = errors.New("locked by another process")

// ErrReplaced is returned if the lock file was removed or replaced while
// the lock was taken, e.g. by destroy, which removes the crannog's
// directory with its lock. A lock on the old file would not exclude a
// process that locks the new one; the caller can try again.
var ErrReplaced = errors.New("the lock file was removed or replaced; try again")

// open opens the lock file name in dir, with create also creating it. It
// must be a regular file; a symlink is refused. O_NONBLOCK keeps a file
// replaced by a FIFO from blocking the open.
func open(dir *os.Root, name string, create bool) (*os.File, error) {
	info, err := dir.Lstat(name)
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if create {
		flags |= os.O_CREATE
	}
	f, err := dir.OpenFile(name, flags, 0o644)
	if err != nil {
		return nil, err
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	return f, nil
}

// checkLocked checks that the locked file f is still the file name in dir.
// Otherwise it closes f.
func checkLocked(dir *os.Root, name string, f *os.File) error {
	afterLock(dir, name)
	locked, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	current, err := dir.Lstat(name)
	if err != nil || !os.SameFile(locked, current) {
		f.Close()
		return fmt.Errorf("lock %s: %w", name, ErrReplaced)
	}
	return nil
}

// afterLock lets tests replace the lock file between the flock and the
// check.
var afterLock = func(dir *os.Root, name string) {}

// TryLock takes the lock on the file name in dir without waiting. It
// creates the file.
func TryLock(dir *os.Root, name string) (*Lock, error) {
	return tryLock(dir, name, true)
}

func tryLock(dir *os.Root, name string, create bool) (*Lock, error) {
	f, err := open(dir, name, create)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("lock %s: %w", name, err)
	}
	if err := checkLocked(dir, name, f); err != nil {
		return nil, err
	}
	return &Lock{f: f}, nil
}

// LockWait takes the lock on the file name in dir, waiting for other
// processes. waiting is called once if the lock is held by another process.
func LockWait(dir *os.Root, name string, waiting func()) (*Lock, error) {
	l, err := TryLock(dir, name)
	if !errors.Is(err, ErrLocked) {
		return l, err
	}
	if waiting != nil {
		waiting()
	}
	f, err := open(dir, name, true)
	if err != nil {
		return nil, err
	}
	if err := flockRetry(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", name, err)
	}
	if err := checkLocked(dir, name, f); err != nil {
		return nil, err
	}
	return &Lock{f: f}, nil
}

func flockRetry(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// File returns the locked file, e.g. to pass it to a child process that
// keeps the lock while it runs.
func (l *Lock) File() *os.File { return l.f }

// Unlock releases the lock. Child processes that inherited the file
// descriptor keep it.
func (l *Lock) Unlock() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// Register creates the session file <pid> in the sessions directory dir
// for the current process and locks it. It returns the lock and the file's
// name. The caller must hold the crannog lock.
func Register(dir *os.Root) (*Lock, string, error) {
	name := strconv.Itoa(os.Getpid())
	l, err := TryLock(dir, name)
	if err != nil {
		return nil, "", fmt.Errorf("register session: %w", err)
	}
	return l, name, nil
}

// Count returns the number of running sessions in the sessions directory
// dir: session files that another process holds locked. Only regular files
// named like a process id are session files. With prune, files of ended
// sessions are deleted; the caller must then hold the crannog lock, so it
// cannot delete a session that is being registered. A session file removed
// meanwhile, by its session when it ends, is skipped and not recreated.
func Count(dir *os.Root, prune bool) (int, error) {
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.Type().IsRegular() || !isPID(e.Name()) {
			continue
		}
		l, err := tryLock(dir, e.Name(), false)
		if errors.Is(err, ErrLocked) {
			n++
			continue
		}
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrReplaced) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if prune {
			_ = dir.Remove(e.Name())
		}
		l.Unlock()
	}
	return n, nil
}

// isPID reports whether name is a process id as Register writes it.
func isPID(name string) bool {
	pid, err := strconv.Atoi(name)
	return err == nil && pid > 0 && strconv.Itoa(pid) == name
}
