package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/google/renameio/v2"

	"github.com/hettiger/crannog/internal/hostfs"
)

// latestFile names the hash of the most recent setup approval.
const latestFile = "latest"

// Store holds a repository's approved setups in
// approved/<sha256>/ (ADR 0011).
type Store struct {
	// Root is the root the store's directory is in.
	Root *os.Root
	// Dir is the store's directory within Root, slash-separated.
	Dir string
}

// open opens the store's directory. Without create, a missing directory
// gives a nil root and no error.
func (st Store) open(create bool) (*os.Root, error) {
	r, err := hostfs.OpenDir(st.Root, st.Dir, create)
	if !create && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return r, err
}

// Has reports whether the content with this hash was approved.
func (st Store) Has(hash string) (bool, error) {
	r, err := st.open(false)
	if r == nil {
		return false, err
	}
	defer r.Close()
	return has(r, hash)
}

func has(r *os.Root, hash string) (bool, error) {
	if err := checkHash(hash); err != nil {
		return false, err
	}
	info, err := r.Lstat(hash)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("approved setup %s is not a directory", hash)
	}
	return true, nil
}

// Get reads an approved setup and checks its setup hash.
func (st Store) Get(hash string) (Setup, error) {
	if err := checkHash(hash); err != nil {
		return nil, err
	}
	s, err := ReadDir(st.Root, st.Dir+"/"+hash)
	if err != nil {
		return nil, err
	}
	if got := s.Hash(); got != hash {
		return nil, fmt.Errorf("approved setup %s was changed (setup hash %s)", hash, got)
	}
	return s, nil
}

// checkHash checks that hash looks like a setup hash, so it is one path
// component.
func checkHash(hash string) error {
	if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		return fmt.Errorf("invalid setup hash %q", hash)
	}
	return nil
}

// Add stores a setup as an approved setup and returns its hash. Adding a
// content twice is harmless.
func (st Store) Add(s Setup) (string, error) {
	hash := s.Hash()
	r, err := st.open(true)
	if err != nil {
		return "", err
	}
	defer r.Close()
	if ok, err := has(r, hash); err != nil || ok {
		return hash, err
	}
	tmp, err := hostfs.MkdirTemp(r, ".tmp-")
	if err != nil {
		return "", err
	}
	defer r.RemoveAll(tmp)
	if err := s.Write(r, tmp+"/copy"); err != nil {
		return "", err
	}
	if err := r.Rename(tmp+"/copy", hash); err != nil {
		// Another crannog process stored the same content first.
		if ok, _ := has(r, hash); ok {
			return hash, nil
		}
		return "", err
	}
	return hash, nil
}

// Latest returns the hash of the most recent approval, or "" if there is
// none.
func (st Store) Latest() (string, error) {
	r, err := st.open(false)
	if r == nil {
		return "", err
	}
	defer r.Close()
	data, err := r.ReadFile(latestFile)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetLatest records the hash of the most recent approval.
func (st Store) SetLatest(hash string) error {
	r, err := st.open(true)
	if err != nil {
		return err
	}
	defer r.Close()
	return renameio.WriteFile(latestFile, []byte(hash+"\n"), 0o644, renameio.WithRoot(r))
}
