package crannog

import "fmt"

// Destroy destroys the crannog of a start branch after asking for
// confirmation (ADR 0015, ADR 0027). The branch does not need to exist
// anymore.
func (m *Manager) Destroy(branchArg string) error {
	t, err := m.crannogOf(branchArg)
	if err != nil {
		return err
	}
	m.SelfRepair()

	lock, err := m.lock(t)
	if err != nil {
		return err
	}
	defer lock.Unlock()

	sessions, err := t.dir.sessions(true)
	if err != nil {
		return err
	}
	hash, err := t.dir.Hash()
	if err != nil {
		return err
	}
	o, err := m.objects(t)
	if err != nil {
		return err
	}
	// Work is in the work volume of a crannog that was built. Without
	// either, nothing can be lost and nothing is asked (ADR 0015).
	if sessions > 0 || (hash != "" && o.work) {
		if err := m.confirmDestroy(t, sessions); err != nil {
			return err
		}
	}

	if err := m.endSessions(t); err != nil {
		return err
	}
	m.UI.Printf("Destroying the crannog of %s\n", t.branch)
	if err := m.remove(t); err != nil {
		return err
	}
	return t.dir.removeAll()
}

// crannogOf returns the crannog of a start branch, by default the
// checked-out one. The crannog must exist, the branch does not need to
// anymore.
func (m *Manager) crannogOf(branchArg string) (*target, error) {
	branch := branchArg
	if branch == "" {
		b, err := m.Repo.CurrentBranch()
		if err != nil {
			return nil, err
		}
		branch = b
	}
	t, err := m.target(branch)
	if err != nil {
		return nil, err
	}
	exists, err := t.dir.exists()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("there is no crannog for %s", branch)
	}
	return t, nil
}

// confirmDestroy says what destroying the crannog deletes and asks for
// confirmation. It does not look into the crannog (ADR 0015).
func (m *Manager) confirmDestroy(t *target, sessions int) error {
	m.UI.Printf("Commits that are not in your repository and uncommitted changes in the crannog of %s are lost.\n", t.branch)
	if sessions > 0 {
		m.UI.Printf("%s running in this crannog end.\n", plural(sessions, "session", "sessions"))
	}
	return m.UI.AskConfirmation(fmt.Sprintf("Destroy the crannog of %s?", t.branch))
}
