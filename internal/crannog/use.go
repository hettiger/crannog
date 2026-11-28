package crannog

// UseZed opens the repository in the running crannog of a start branch in
// Zed, over the user's ssh like crannog ssh.
func (m *Manager) UseZed(branchArg string) error {
	t, err := m.sshTarget(branchArg)
	if err != nil {
		return err
	}
	return m.Zed.OpenSSH(t.names.Container, m.Repo.PathInCrannog())
}
