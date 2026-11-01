package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
)

// latestFile names the hash of the most recent setup approval.
const latestFile = "latest"

// Store holds the approved copies of a repository's setups in
// approved/<sha256>/ (ADR 0011).
type Store struct {
	Dir string
}

// Path returns the directory of an approved copy.
func (st Store) Path(hash string) string { return filepath.Join(st.Dir, hash) }

// Has reports whether the content with this hash was approved.
func (st Store) Has(hash string) (bool, error) {
	info, err := os.Lstat(st.Path(hash))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", st.Path(hash))
	}
	return true, nil
}

// Get reads an approved copy and checks its hash.
func (st Store) Get(hash string) (Setup, error) {
	s, err := ReadDir(st.Path(hash))
	if err != nil {
		return nil, err
	}
	if got := s.Hash(); got != hash {
		return nil, fmt.Errorf("approved copy %s was changed (content hash %s)", st.Path(hash), got)
	}
	return s, nil
}

// Add stores a setup as an approved copy and returns its hash. Adding a
// content twice is harmless.
func (st Store) Add(s Setup) (string, error) {
	hash := s.Hash()
	ok, err := st.Has(hash)
	if err != nil || ok {
		return hash, err
	}
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(st.Dir, ".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "copy")
	if err := s.Write(dir); err != nil {
		return "", err
	}
	if err := os.Rename(dir, st.Path(hash)); err != nil {
		// Another crannog process stored the same content first.
		if ok, _ := st.Has(hash); ok {
			return hash, nil
		}
		return "", err
	}
	return hash, nil
}

// Latest returns the hash of the most recent approval, or "" if there is
// none.
func (st Store) Latest() (string, error) {
	data, err := os.ReadFile(filepath.Join(st.Dir, latestFile))
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
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		return err
	}
	return renameio.WriteFile(filepath.Join(st.Dir, latestFile), []byte(hash+"\n"), 0o644)
}
