package crannog

import (
	"errors"
	"strings"

	"github.com/hettiger/crannog/internal/git"
	"github.com/hettiger/crannog/internal/naming"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/session"
)

// SelfRepair stops the repository's running crannogs that have no session
// and removes crannog remotes whose crannog is not running (ADR 0019).
// Crannogs whose lock is held are skipped. Problems are printed as
// warnings.
func (m *Manager) SelfRepair() {
	if err := m.selfRepair(); err != nil {
		m.UI.Printf("Warning: self-repair: %v\n", err)
	}
}

func (m *Manager) selfRepair() error {
	containers, err := m.list(naming.Prefix(m.Repo.ID8()))
	if err != nil {
		return err
	}
	running := map[string]bool{}
	for _, c := range containers {
		running[c.Name] = c.State == runtime.StateRunning
	}
	remotes, err := m.crannogRemotes()
	if err != nil {
		return err
	}

	slugs := map[string]bool{}
	for s := range remotes {
		slugs[s] = true
	}
	dirs, err := m.hostDirSlugs()
	if err != nil {
		return err
	}
	for _, s := range dirs {
		slugs[s] = true
	}

	var errs []error
	for s := range slugs {
		n := naming.New(m.Repo.ID8(), s)
		d := newHostDir(m.CrannogHome, m.Repo.ID, s)
		ok, err := d.exists()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !ok {
			// A remote without a crannog directory.
			if remotes[s] && !running[n.Container] {
				errs = append(errs, m.removeRemote(n))
			}
			continue
		}
		lock, err := d.tryLock()
		if errors.Is(err, session.ErrLocked) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, m.repairOne(n, d, running, remotes[s]))
		lock.Unlock()
	}
	return errors.Join(errs...)
}

// repairOne repairs one crannog. The caller holds its lock.
func (m *Manager) repairOne(n naming.Names, d HostDir, running map[string]bool, hasRemote bool) error {
	sessions, err := d.sessions(true)
	if err != nil {
		return err
	}
	if sessions == 0 && (running[n.Container] || running[n.Proxy]) {
		branch, _ := d.Branch()
		t := &target{branch: branch, names: n, dir: d}
		m.UI.Printf("The crannog of %s runs without a session.\n", branch)
		return m.stop(t)
	}
	if hasRemote && !running[n.Container] {
		return m.removeRemote(n)
	}
	return nil
}

// crannogRemotes returns the slugs of the remotes crannog-<slug> with the
// URL crannog::<slug>. Other remotes are the user's.
func (m *Manager) crannogRemotes() (map[string]bool, error) {
	out, err := m.Repo.Git("config", "--get-regexp", `^remote\.crannog-.*\.url$`)
	if git.ExitCode(err) == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	slugs := map[string]bool{}
	for _, l := range lines(out) {
		key, url, ok := strings.Cut(l, " ")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		s, ok := naming.SlugFromURL(url)
		if ok && name == "crannog-"+s {
			slugs[s] = true
		}
	}
	return slugs, nil
}
