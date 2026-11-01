// Package session implements the crannog lock and the session files with
// flock (ADR 0014, ADR 0025). The kernel releases a lock when the last
// process holding its file descriptor ends, so locks never stay behind.
package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Lock is an exclusive flock on a file.
type Lock struct {
	f *os.File
}

// ErrLocked is returned by TryLock if another process holds the lock.
var ErrLocked = errors.New("locked by another process")

func open(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o644)
}

// TryLock takes the lock on path without waiting. It creates the file.
func TryLock(path string) (*Lock, error) {
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &Lock{f: f}, nil
}

// LockWait takes the lock on path, waiting for other processes. waiting is
// called once if the lock is held by another process.
func LockWait(path string, waiting func()) (*Lock, error) {
	l, err := TryLock(path)
	if !errors.Is(err, ErrLocked) {
		return l, err
	}
	if waiting != nil {
		waiting()
	}
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	if err := flockRetry(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
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

// Register creates sessions/<pid> for the current process and locks it.
// The caller must hold the crannog lock.
func Register(dir string) (*Lock, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, strconv.Itoa(os.Getpid()))
	l, err := TryLock(path)
	if err != nil {
		return nil, "", fmt.Errorf("register session: %w", err)
	}
	return l, path, nil
}

// Count returns the number of running sessions in dir: files in it that
// another process holds locked. With prune, files of ended sessions are
// deleted; the caller must then hold the crannog lock, so it cannot delete
// a session that is being registered.
func Count(dir string, prune bool) (int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		l, err := TryLock(path)
		if errors.Is(err, ErrLocked) {
			n++
			continue
		}
		if err != nil {
			return 0, err
		}
		if prune {
			_ = os.Remove(path)
		}
		l.Unlock()
	}
	return n, nil
}
