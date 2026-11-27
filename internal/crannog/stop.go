package crannog

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/hettiger/crannog/internal/session"
)

// Stop ends the sessions in the crannog of a start branch, detached ones
// included, and stops the crannog (ADR 0019). It asks nothing: nothing is
// deleted. The branch does not need to exist anymore.
func (m *Manager) Stop(branchArg string) error {
	t, err := m.crannogOf(branchArg)
	if err != nil {
		return err
	}
	// The lock must not create the host directory again if destroy removed
	// it meanwhile. Self-repair comes after it and skips this crannog, so
	// that Stop itself says what it does to it.
	lock, err := t.dir.lockExisting(m.waiting(t))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, session.ErrReplaced) {
		return fmt.Errorf("there is no crannog for %s", t.branch)
	}
	if err != nil {
		return err
	}
	defer lock.Unlock()
	m.SelfRepair()

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
