package crannog

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/hettiger/crannog/internal/open"
)

// probeTimeout bounds the check that a server answers.
const probeTimeout = 2 * time.Second

// WebOptions are the options of crannog open for the browser.
type WebOptions struct {
	// Insecure opens http instead of https.
	Insecure bool
	// Port is the server's port; 0 is the scheme's default.
	Port uint16
}

// OpenWeb opens the running crannog of a start branch in the browser, at
// its address on its network, if something accepts connections there.
func (m *Manager) OpenWeb(branchArg string, o WebOptions) error {
	t, err := m.crannogOf(branchArg)
	if err != nil {
		return err
	}
	st, err := m.status(t.names)
	if err != nil {
		return err
	}
	if !st.running() {
		return errNotRunning(t.branch, "")
	}
	ip, err := netip.ParseAddr(st.crannog.IPv4On(t.names.Network))
	if err != nil {
		return fmt.Errorf("the crannog of %s has no address on its network %s", t.branch, t.names.Network)
	}
	port := o.Port
	if port == 0 {
		port = 443
		if o.Insecure {
			port = 80
		}
	}
	u := open.WebURL(o.Insecure, ip, o.Port)
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	if err := m.Probe(ctx, netip.AddrPortFrom(ip, port)); err != nil {
		return fmt.Errorf("nothing answers on %s", u)
	}
	return m.Opener.Web(o.Insecure, ip, o.Port)
}

// OpenZed opens the repository in the running crannog of a start branch
// in Zed, over the user's ssh like crannog ssh.
func (m *Manager) OpenZed(branchArg string) error {
	t, err := m.sshTarget(branchArg)
	if err != nil {
		return err
	}
	return m.Opener.ZedSSH(t.names.Container, m.Repo.PathInCrannog())
}

// SSHHost returns the SSH host of the crannog of a start branch. The
// crannog need not run.
func (m *Manager) SSHHost(branchArg string) (string, error) {
	t, err := m.crannogOf(branchArg)
	if err != nil {
		return "", err
	}
	return t.names.Container, nil
}

// SSHPath returns <host>:<path of the repository in the crannog>, as scp,
// rsync and sftp take it. The crannog need not run.
func (m *Manager) SSHPath(branchArg string) (string, error) {
	host, err := m.SSHHost(branchArg)
	if err != nil {
		return "", err
	}
	return host + ":" + m.Repo.PathInCrannog(), nil
}
