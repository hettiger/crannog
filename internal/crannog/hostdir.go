package crannog

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
)

// HostDir is a crannog's directory on the host,
// ~/.crannog/repositories/<id>/crannogs/<slug>/ (ADR 0014, ADR 0025).
type HostDir struct {
	Dir string
}

func (d HostDir) branchFile() string { return filepath.Join(d.Dir, "branch") }
func (d HostDir) hashFile() string   { return filepath.Join(d.Dir, "crannog") }

// LockPath is held while the crannog is started or stopped.
func (d HostDir) LockPath() string { return filepath.Join(d.Dir, "lock") }

// SessionsDir holds one locked file per running session.
func (d HostDir) SessionsDir() string { return filepath.Join(d.Dir, "sessions") }

// ProxyDir is mounted into the proxy: CA, secrets, configuration.
func (d HostDir) ProxyDir() string { return filepath.Join(d.Dir, "proxy") }

// CADir is mounted into the crannog: ca.crt only.
func (d HostDir) CADir() string { return filepath.Join(d.Dir, "ca") }

// Branch returns the start branch, or "" if it was not recorded.
func (d HostDir) Branch() (string, error) { return readValue(d.branchFile()) }

// Hash returns the hash of the approved copy the crannog uses, or "" if the
// crannog was never completely built.
func (d HostDir) Hash() (string, error) { return readValue(d.hashFile()) }

func (d HostDir) WriteBranch(branch string) error { return writeValue(d.branchFile(), branch) }

func (d HostDir) WriteHash(hash string) error { return writeValue(d.hashFile(), hash) }

func readValue(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

func writeValue(path, value string) error {
	return renameio.WriteFile(path, []byte(value+"\n"), 0o644)
}

// clear removes everything in the directory except the lock, which the
// caller may hold.
func (d HostDir) clear() error {
	entries, err := os.ReadDir(d.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if e.Name() == "lock" {
			continue
		}
		errs = append(errs, os.RemoveAll(filepath.Join(d.Dir, e.Name())))
	}
	return errors.Join(errs...)
}
