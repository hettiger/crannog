package crannog

// Stop ends the sessions in the crannog of a start branch, detached ones
// included, and stops the crannog (ADR 0019). It asks nothing: nothing is
// deleted. The branch does not need to exist anymore.
func (m *Manager) Stop(branchArg string) error {
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

	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	if st.running() {
		if err := m.endSessions(t); err != nil {
			return err
		}
	} else if !st.proxyRunning() {
		m.UI.Printf("The crannog of %s is not running\n", t.branch)
	}
	return m.stop(t)
}
