// Package home lays out crannog's state on the host in ~/.crannog
// (ADR 0008, 0009, 0011, 0014, 0015, 0025).
package home

import (
	"fmt"
	"os"
	"path/filepath"
)

// Home is crannog's directory on the host, ~/.crannog.
type Home struct {
	Dir string
}

// Default returns ~/.crannog of the current user.
func Default() (Home, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return Home{}, fmt.Errorf("cannot find the home directory: %w", err)
	}
	return Home{Dir: filepath.Join(h, ".crannog")}, nil
}

// AgentsDir holds the agent homes shared by all crannogs.
func (h Home) AgentsDir() string { return filepath.Join(h.Dir, "agents") }

// CacheDir holds the package caches shared by all crannogs.
func (h Home) CacheDir() string { return filepath.Join(h.Dir, "cache") }

// PresetsDir holds the user's presets.
func (h Home) PresetsDir() string { return filepath.Join(h.Dir, "presets") }

// RepositoryDir holds crannog's state about one repository.
func (h Home) RepositoryDir(id string) string {
	return filepath.Join(h.Dir, "repositories", id)
}

// CrannogsDir holds the host directories of a repository's crannogs.
func (h Home) CrannogsDir(id string) string {
	return filepath.Join(h.RepositoryDir(id), "crannogs")
}

// CrannogDir is the host directory of one crannog.
func (h Home) CrannogDir(id, slug string) string {
	return filepath.Join(h.CrannogsDir(id), slug)
}

// ApprovedDir holds the approved copies of a repository's .crannog/.
func (h Home) ApprovedDir(id string) string {
	return filepath.Join(h.RepositoryDir(id), "approved")
}
