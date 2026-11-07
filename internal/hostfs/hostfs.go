// Package hostfs anchors crannog's file system access on the host
// (ADR 0032). There are three anchors: ~/.crannog, .crannog/ in a worktree
// and temporary directories crannog creates. This package opens them as an
// os.Root; all other code works within such a root, so a path can never
// lead outside of an anchor.
package hostfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// OpenAnchor opens the directory dir as a root. With create, dir is
// created if it is missing; its parent must exist.
func OpenAnchor(dir string, create bool) (*os.Root, error) {
	if create {
		if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return os.OpenRoot(dir)
}

// OpenDir opens the directory name within root as a new root. name is
// slash-separated and relative; no component of it may be a symlink. With
// create, missing directories are created.
func OpenDir(root *os.Root, name string, create bool) (*os.Root, error) {
	if name == "." {
		return root.OpenRoot(".")
	}
	cur := root
	for _, c := range strings.Split(name, "/") {
		next, err := openComponent(cur, c, create)
		if cur != root {
			cur.Close()
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		cur = next
	}
	return cur, nil
}

func openComponent(dir *os.Root, name string, create bool) (*os.Root, error) {
	if name == "" || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid path component %q", name)
	}
	info, err := dir.Lstat(name)
	if create && errors.Is(err, fs.ErrNotExist) {
		if err := dir.Mkdir(name, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		info, err = dir.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", name)
	}
	beforeOpen(dir, name)
	// OpenRoot follows a symlink within dir, and name can be replaced by
	// one after the Lstat. The opened directory must be the one checked.
	r, err := dir.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := r.Stat(".")
	if err != nil {
		r.Close()
		return nil, err
	}
	if !os.SameFile(info, opened) {
		r.Close()
		return nil, fmt.Errorf("%s was replaced while opening it", name)
	}
	return r, nil
}

// beforeOpen lets tests replace a directory between the check and the open.
var beforeOpen = func(dir *os.Root, name string) {}

// MkdirAll creates the directory name within root, see OpenDir.
func MkdirAll(root *os.Root, name string) error {
	r, err := OpenDir(root, name, true)
	if err != nil {
		return err
	}
	return r.Close()
}

// MkdirTemp creates a new directory with a random name that starts with
// prefix in root and returns its name.
func MkdirTemp(root *os.Root, prefix string) (string, error) {
	for range 10 {
		name := prefix + randomSuffix()
		err := root.Mkdir(name, 0o700)
		if err == nil {
			return name, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("cannot create a temporary directory %s* in %s", prefix, root.Name())
}

func randomSuffix() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// TempDir is a temporary directory crannog created in the user's
// temporary directory ($TMPDIR on macOS).
type TempDir struct {
	*os.Root
	// Dir is the directory's absolute path, e.g. to run a command in it.
	Dir string
	// parent is the user's temporary directory, opened once, so Remove
	// deletes in the directory the temporary directory was created in.
	parent *os.Root
	name   string
}

// NewTempDir creates a temporary directory named crannog-<name>-<random>.
func NewTempDir(name string) (*TempDir, error) {
	parent, err := OpenAnchor(os.TempDir(), false)
	if err != nil {
		return nil, err
	}
	dir, err := MkdirTemp(parent, "crannog-"+name+"-")
	if err != nil {
		parent.Close()
		return nil, err
	}
	root, err := OpenDir(parent, dir, false)
	if err != nil {
		_ = parent.Remove(dir)
		parent.Close()
		return nil, err
	}
	return &TempDir{Root: root, Dir: filepath.Join(parent.Name(), dir), parent: parent, name: dir}, nil
}

// Remove closes and deletes the temporary directory.
func (t *TempDir) Remove() error {
	return errors.Join(t.Root.Close(), t.parent.RemoveAll(t.name), t.parent.Close())
}
