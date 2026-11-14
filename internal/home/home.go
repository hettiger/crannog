// Package home lays out the crannog home ~/.crannog on the host
// (ADR 0021).
package home

import (
	"os"
	"path/filepath"

	"github.com/hettiger/crannog/internal/hostfs"
)

// CrannogHome is the crannog home on the host, ~/.crannog.
type CrannogHome struct {
	// Dir is the absolute path, for mounts and messages.
	Dir string
	// Root is the anchor all access to ~/.crannog goes through (ADR 0013).
	Root *os.Root
}

// OpenCrannogHome opens the crannog home of the current user, creating it
// if needed.
func OpenCrannogHome() (CrannogHome, error) {
	root, err := hostfs.OpenCrannogHome()
	if err != nil {
		return CrannogHome{}, err
	}
	return CrannogHome{Dir: root.Name(), Root: root}, nil
}

// Close closes the root.
func (h CrannogHome) Close() error { return h.Root.Close() }

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
func (h CrannogHome) Abs(p string) string { return filepath.Join(h.Dir, p) }

// OpenDir opens a directory within ~/.crannog as a root, see
// hostfs.OpenDir.
func (h CrannogHome) OpenDir(p string, create bool) (*os.Root, error) {
	return hostfs.OpenDir(h.Root, p, create)
}
