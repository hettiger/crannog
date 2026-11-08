package crannog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/hostfs"
	"github.com/hettiger/crannog/internal/session"
)

// Names in a crannog's host directory.
const (
	branchFile  = "branch"
	hashFile    = "crannog"
	lockFile    = "lock"
	sessionsDir = "sessions"
	proxyDir    = "proxy"
	caDir       = "ca"
)

// HostDir is a crannog's directory on the host,
// ~/.crannog/repositories/<id>/crannogs/<slug>/ (ADR 0014, ADR 0025). All
// access goes through the root of ~/.crannog; no directory on the way may
// be a symlink (ADR 0031).
type HostDir struct {
	home home.Home
	// crannogs is the repository's crannogs directory within ~/.crannog.
	crannogs string
	slug     string
	// path is the directory within ~/.crannog.
	path string
	// Dir is the absolute path, for mounts and messages.
	Dir string
}

func newHostDir(h home.Home, id, slug string) HostDir {
	p := home.CrannogPath(id, slug)
	return HostDir{home: h, crannogs: home.CrannogsPath(id), slug: slug, path: p, Dir: h.Abs(p)}
}

// open opens the directory or one below it. Without create, a missing
// directory is an error that matches fs.ErrNotExist.
func (d HostDir) open(sub string, create bool) (*os.Root, error) {
	return d.home.OpenDir(filepath.Join(d.path, sub), create)
}

// exists reports whether the directory exists.
func (d HostDir) exists() (bool, error) {
	r, err := d.open("", false)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, r.Close()
}

// ProxyDir is mounted into the proxy: CA, secrets, configuration.
func (d HostDir) ProxyDir() string { return d.home.Abs(filepath.Join(d.path, proxyDir)) }

// CADir is mounted into the crannog: ca.crt only.
func (d HostDir) CADir() string { return d.home.Abs(filepath.Join(d.path, caDir)) }

// Branch returns the start branch, or "" if it was not recorded.
func (d HostDir) Branch() (string, error) { return d.readValue(branchFile) }

// Hash returns the setup hash of the approved setup the crannog uses, or "" if the
// crannog was never completely built.
func (d HostDir) Hash() (string, error) { return d.readValue(hashFile) }

func (d HostDir) WriteBranch(branch string) error { return d.writeValue(branchFile, branch) }

func (d HostDir) WriteHash(hash string) error { return d.writeValue(hashFile, hash) }

func (d HostDir) readValue(name string) (string, error) {
	r, err := d.open("", false)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer r.Close()
	data, err := r.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

func (d HostDir) writeValue(name, value string) error {
	r, err := d.open("", true)
	if err != nil {
		return err
	}
	defer r.Close()
	return renameio.WriteFile(name, []byte(value+"\n"), 0o644, renameio.WithRoot(r))
}

// mkdirs creates the given directories below the directory.
func (d HostDir) mkdirs(subs ...string) error {
	for _, s := range subs {
		if err := hostfs.MkdirAll(d.home.Root, filepath.Join(d.path, s)); err != nil {
			return err
		}
	}
	return nil
}

// lock takes the crannog's lock, which is held while the crannog is
// started or stopped. It creates the directory.
func (d HostDir) lock(waiting func()) (*session.Lock, error) {
	r, err := d.open("", true)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return session.LockWait(r, lockFile, waiting)
}

// tryLock takes the crannog's lock without waiting.
func (d HostDir) tryLock() (*session.Lock, error) {
	r, err := d.open("", false)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return session.TryLock(r, lockFile)
}

// registerSession creates and locks the session file of this process and
// returns its name. The caller holds the crannog's lock.
func (d HostDir) registerSession() (*session.Lock, string, error) {
	r, err := d.open(sessionsDir, true)
	if err != nil {
		return nil, "", err
	}
	defer r.Close()
	return session.Register(r)
}

// removeSession deletes a session file registerSession created.
func (d HostDir) removeSession(name string) error {
	r, err := d.open(sessionsDir, false)
	if err != nil {
		return err
	}
	defer r.Close()
	return r.Remove(name)
}

// sessions counts the running sessions, see session.Count.
func (d HostDir) sessions(prune bool) (int, error) {
	r, err := d.open(sessionsDir, false)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer r.Close()
	return session.Count(r, prune)
}

// clear removes everything in the directory except the lock, which the
// caller may hold.
func (d HostDir) clear() error {
	r, err := d.open("", false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	entries, err := fs.ReadDir(r.FS(), ".")
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if e.Name() == lockFile {
			continue
		}
		errs = append(errs, r.RemoveAll(e.Name()))
	}
	return errors.Join(errs...)
}

// removeAll removes the directory with everything in it.
func (d HostDir) removeAll() error {
	r, err := d.home.OpenDir(d.crannogs, false)
	if err != nil {
		return err
	}
	defer r.Close()
	info, err := r.Lstat(d.slug)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", d.Dir)
	}
	return r.RemoveAll(d.slug)
}
