package crannog

import (
	"fmt"
)

// Apply applies changes to .crannog/ to the crannog of a start branch
// without opening a session (ADR 0027). A rebuilt crannog stays stopped
// until the next session.
func (m *Manager) Apply(branchArg string) error {
	branch, err := m.Repo.StartBranch(branchArg)
	if err != nil {
		return err
	}
	t, err := m.target(branch)
	if err != nil {
		return err
	}
	m.SelfRepair()

	hash, err := t.dir.Hash()
	if err != nil {
		return err
	}
	if hash == "" {
		return fmt.Errorf("there is no crannog for %s; start one with crannog checkout %s", branch, branch)
	}
	src, err := m.readSource(branch)
	if err != nil {
		return err
	}
	lock, err := m.lock(t)
	if err != nil {
		return err
	}
	defer lock.Unlock()

	if hash, err = t.dir.Hash(); err != nil {
		return err
	}
	current, err := m.store().Get(hash)
	if err != nil {
		return err
	}
	sessions, err := t.dir.sessions(true)
	if err != nil {
		return err
	}
	c, next, err := m.review(t, src, current, sessions, false)
	if err != nil {
		return err
	}
	switch c {
	case changeNone:
		m.UI.Printf("The crannog of %s is up to date.\n", branch)
		return nil
	case changeReload:
		return m.reload(t, next)
	default:
		return m.rebuild(t, next, false)
	}
}
