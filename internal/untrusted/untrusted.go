// Package untrusted opens files in directories that a crannog or a proxy
// can write to (ADR 0031), and other files crannog reads but does not
// write, such as a worktree's .crannog/. Callers anchor every access in an
// os.Root, so symlinks never lead out of the directory, even if the guest
// replaces paths concurrently.
package untrusted

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// Open opens the regular file name in root for reading. Symlinks are
// followed only within root. O_NONBLOCK keeps a FIFO from blocking the open;
// anything but a regular file is refused.
func Open(root *os.Root, name string) (*os.File, fs.FileInfo, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, nil, fmt.Errorf("%s is not a regular file", name)
	}
	return f, info, nil
}

// ReadFile reads the regular file name in root, see Open. Files larger than
// max bytes are refused.
func ReadFile(root *os.Root, name string, max int64) ([]byte, fs.FileInfo, error) {
	f, info, err := Open(root, name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) > max {
		return nil, nil, fmt.Errorf("%s is larger than %d bytes", name, max)
	}
	return data, info, nil
}
