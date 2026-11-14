// Package hostfs anchors crannog's file system access on the host
// (ADR 0013). There are three anchors: the crannog home ~/.crannog,
// .crannog/ in a worktree and temporary directories crannog creates. Only
// this package decides where they are and opens them; all other code works
// within a root it gets from here, so a path can never lead outside of an
// anchor.
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

// SetupDir is the name of the setup directory in a worktree.
const SetupDir = ".crannog"

// OpenCrannogHome opens the crannog home ~/.crannog of the current user
// as a root, creating it readable by the user only if it is missing.
func OpenCrannogHome() (*os.Root, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot find the home directory: %w", err)
	}
	dir := filepath.Join(h, ".crannog")
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	return os.OpenRoot(dir)
}

// Worktree is a git worktree on the host. Only its setup directory and
// temporary siblings of it can be accessed.
type Worktree struct {
	root *os.Root
}

// OpenWorktree opens the worktree at dir. dir must be the top of a git
// worktree.
func OpenWorktree(dir string) (*Worktree, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	if _, err := root.Lstat(".git"); err != nil {
		root.Close()
		return nil, fmt.Errorf("%s is not the top of a git worktree: %w", dir, err)
	}
	return &Worktree{root: root}, nil
}

// Close closes the worktree.
func (w *Worktree) Close() error { return w.root.Close() }

// LstatSetup returns information about the setup directory without
// following a symlink.
func (w *Worktree) LstatSetup() (fs.FileInfo, error) { return w.root.Lstat(SetupDir) }

// OpenSetup opens the setup directory as a root, see OpenDir.
func (w *Worktree) OpenSetup() (*os.Root, error) { return OpenDir(w.root, SetupDir, false) }

// SetupTemp is a temporary sibling .crannog.tmp-* of the setup directory,
// used to replace it.
type SetupTemp struct {
	*os.Root
	// Dir is the directory's absolute path, for messages.
	Dir  string
	wt   *Worktree
	name string
}

// NewSetupTemp creates a temporary sibling of the setup directory.
func (w *Worktree) NewSetupTemp() (*SetupTemp, error) {
	name, err := MkdirTemp(w.root, SetupDir+".tmp-")
	if err != nil {
		return nil, err
	}
	root, err := OpenDir(w.root, name, false)
	if err != nil {
		_ = w.root.Remove(name)
		return nil, err
	}
	return &SetupTemp{Root: root, Dir: filepath.Join(w.root.Name(), name), wt: w, name: name}, nil
}

// Stash moves the setup directory into the temporary directory as name.
func (t *SetupTemp) Stash(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return t.wt.root.Rename(SetupDir, filepath.Join(t.name, name))
}

// Install moves name from the temporary directory to the setup directory.
func (t *SetupTemp) Install(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return t.wt.root.Rename(filepath.Join(t.name, name), SetupDir)
}

// Remove closes and deletes the temporary directory.
func (t *SetupTemp) Remove() error {
	return errors.Join(t.Root.Close(), t.wt.root.RemoveAll(t.name))
}

func checkName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}

// OpenDir opens the directory name within root as a new root. name is
// relative; no component of it may be a symlink. With
// create, missing directories are created.
func OpenDir(root *os.Root, name string, create bool) (*os.Root, error) {
	if name == "." {
		return root.OpenRoot(".")
	}
	cur := root
	for _, c := range strings.Split(name, string(filepath.Separator)) {
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
	if err := checkName(name); err != nil {
		return nil, err
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
	return dir.OpenRoot(name)
}

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
	parent, err := os.OpenRoot(os.TempDir())
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
