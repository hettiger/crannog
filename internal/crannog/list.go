package crannog

import (
	"errors"
	"io/fs"
	"sort"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/slug"
)

// Info describes a crannog for crannog list.
type Info struct {
	Branch   string
	Status   string
	Sessions int
}

// Statuses shown by crannog list besides the runtime's states.
const (
	StatusIncomplete = "incomplete"
	StatusMissing    = "containers missing"
)

// List returns the repository's crannogs, sorted by start branch.
func (m *Manager) List() ([]Info, error) {
	m.SelfRepair()
	containers, err := m.list(naming.Prefix(m.Repo.ID8()))
	if err != nil {
		return nil, err
	}
	byName := map[string]runtime.Container{}
	for _, c := range containers {
		byName[c.Name] = c
	}

	slugs, err := m.hostDirSlugs()
	if err != nil {
		return nil, err
	}
	var list []Info
	for _, s := range slugs {
		d := newHostDir(m.Home, m.Repo.ID, s)
		branch, err := d.Branch()
		if err != nil || branch == "" {
			continue
		}
		hash, err := d.Hash()
		if err != nil {
			return nil, err
		}
		n := naming.New(m.Repo.ID8(), s)
		info := Info{Branch: branch}
		c, ok := byName[n.Container]
		switch {
		case hash == "":
			info.Status = StatusIncomplete
		case !ok:
			info.Status = StatusMissing
		default:
			info.Status = c.State
		}
		if info.Sessions, err = d.sessions(false); err != nil {
			return nil, err
		}
		list = append(list, info)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Branch < list[j].Branch })
	return list, nil
}

// hostDirSlugs returns the branch slugs of the repository's crannog
// directories. Entries that are no directory or not named like a branch
// slug are not crannog directories.
func (m *Manager) hostDirSlugs() ([]string, error) {
	r, err := m.Home.OpenDir(home.CrannogsPath(m.Repo.ID), false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	entries, err := fs.ReadDir(r.FS(), ".")
	if err != nil {
		return nil, err
	}
	var slugs []string
	for _, e := range entries {
		if e.IsDir() && slug.Valid(e.Name()) {
			slugs = append(slugs, e.Name())
		}
	}
	return slugs, nil
}
