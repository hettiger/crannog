package crannog

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/session"
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
	containers, err := m.Runtime.List(naming.Prefix(m.Repo.ID8()))
	if err != nil {
		return nil, err
	}
	byName := map[string]runtime.Container{}
	for _, c := range containers {
		byName[c.Name] = c
	}

	entries, err := os.ReadDir(m.Home.CrannogsDir(m.Repo.ID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Info
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := HostDir{Dir: filepath.Join(m.Home.CrannogsDir(m.Repo.ID), e.Name())}
		branch, err := d.Branch()
		if err != nil || branch == "" {
			continue
		}
		hash, err := d.Hash()
		if err != nil {
			return nil, err
		}
		n := naming.New(m.Repo.ID8(), e.Name())
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
		if info.Sessions, err = session.Count(d.SessionsDir(), false); err != nil {
			return nil, err
		}
		list = append(list, info)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Branch < list[j].Branch })
	return list, nil
}
