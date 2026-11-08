// Package home lays out crannog's state on the host in ~/.crannog
// (ADR 0021).
package home

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hettiger/crannog/internal/hostfs"
)

// Home is crannog's directory on the host, ~/.crannog.
type Home struct {
	// Dir is the absolute path, for mounts and messages.
	Dir string
	// Root is the anchor all access to ~/.crannog goes through (ADR 0013).
	Root *os.Root
}

// Default opens ~/.crannog of the current user, creating it if needed.
func Default() (Home, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return Home{}, fmt.Errorf("cannot find the home directory: %w", err)
	}
	return Open(filepath.Join(h, ".crannog"))
}

// Open opens dir as crannog's directory, creating it if needed.
func Open(dir string) (Home, error) {
	root, err := hostfs.OpenAnchor(dir, true)
	if err != nil {
		return Home{}, err
	}
	return Home{Dir: dir, Root: root}, nil
}

// Close closes the root.
func (h Home) Close() error { return h.Root.Close() }

// Paths within ~/.crannog.
const (
	// Agents holds the agent homes shared by all crannogs.
	Agents = "agents"
	// Cache holds the package caches shared by all crannogs.
	Cache = "cache"
	// Presets holds the user's presets.
	Presets = "presets"
)

// PathFile, in a repository's directory, holds the repository's git common
// dir, from which its id is derived.
const PathFile = "path"

// RepositoryPath holds crannog's state about one repository.
func RepositoryPath(id string) string { return filepath.Join("repositories", id) }

// CrannogsPath holds the host directories of a repository's crannogs.
func CrannogsPath(id string) string { return filepath.Join(RepositoryPath(id), "crannogs") }

// CrannogPath is the host directory of one crannog.
func CrannogPath(id, slug string) string { return filepath.Join(CrannogsPath(id), slug) }

// ApprovedPath holds a repository's approved setups.
func ApprovedPath(id string) string { return filepath.Join(RepositoryPath(id), "approved") }

// Abs returns the absolute path of a path within ~/.crannog.
func (h Home) Abs(p string) string { return filepath.Join(h.Dir, p) }

// OpenDir opens a directory within ~/.crannog as a root, see
// hostfs.OpenDir.
func (h Home) OpenDir(p string, create bool) (*os.Root, error) {
	return hostfs.OpenDir(h.Root, p, create)
}
