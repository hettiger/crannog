package crannog

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/runtime"
)

// crannog open opens the address of a running crannog in the browser,
// only if something answers there.
func TestOpenWeb(t *testing.T) {
	f := newFixture(t)
	assert.EqualError(t, f.m.OpenWeb("", WebOptions{}), "there is no crannog for main")

	tg := f.existing("main", defaultSetup(t))
	assert.EqualError(t, f.m.OpenWeb("", WebOptions{}), "the crannog of main is not running; start it with: crannog checkout -d main")
	assert.Empty(t, f.rt.callsWith("start"), "never starts a crannog")

	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	ip := f.rt.containers[tg.names.Container].IPv4On(tg.names.Network)
	require.NotEmpty(t, ip)
	assert.EqualError(t, f.m.OpenWeb("", WebOptions{}), "nothing answers on https://"+ip)
	assert.EqualError(t, f.m.OpenWeb("", WebOptions{Insecure: true, Port: 8080}), "nothing answers on http://"+ip+":8080")
	assert.Empty(t, f.opener.opened)

	f.answers = true
	require.NoError(t, f.m.OpenWeb("main", WebOptions{}))
	require.NoError(t, f.m.OpenWeb("main", WebOptions{Insecure: true}))
	require.NoError(t, f.m.OpenWeb("main", WebOptions{Port: 8080}))
	a := netip.MustParseAddr(ip)
	assert.Equal(t, []netip.AddrPort{
		netip.AddrPortFrom(a, 443), netip.AddrPortFrom(a, 8080),
		netip.AddrPortFrom(a, 443), netip.AddrPortFrom(a, 80), netip.AddrPortFrom(a, 8080),
	}, f.probed, "the default port of the scheme")
	assert.Equal(t, []string{"https://" + ip, "http://" + ip, "https://" + ip + ":8080"}, f.opener.opened)
	assert.Empty(t, f.out.String())
}

// crannog open --zed opens the repository in a running crannog, after the
// same checks as crannog ssh.
func TestOpenZed(t *testing.T) {
	f := newFixture(t)
	assert.EqualError(t, f.m.OpenZed(""), "there is no crannog for main")

	tg := f.existing("main", defaultSetup(t))
	assert.EqualError(t, f.m.OpenZed(""), "the crannog of main is not running; start it with: crannog checkout -d main")

	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	require.NoError(t, f.m.prepareSSH(tg))
	assert.ErrorContains(t, f.m.OpenZed(""), "Include ~/.crannog/ssh/config", "without the Include")
	assert.Empty(t, f.opener.opened)

	f.ssh.config["proxycommand"] = []string{"/usr/local/bin/crannog ssh-proxy --container-cli /usr/local/bin/container " + f.m.Repo.ID + " " + tg.names.Slug}
	require.NoError(t, f.m.OpenZed("main"))
	assert.Equal(t, []string{"zed " + tg.names.Container + " " + f.m.Repo.PathInCrannog()}, f.opener.opened)
	assert.Empty(t, f.probed, "no server check for Zed")
}

// crannog ssh --host and --path print, for a crannog that need not run.
func TestSSHHostAndPath(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.SSHHost("")
	assert.EqualError(t, err, "there is no crannog for main")
	_, err = f.m.SSHPath("")
	assert.EqualError(t, err, "there is no crannog for main")

	tg := f.existing("main", defaultSetup(t))
	host, err := f.m.SSHHost("")
	require.NoError(t, err)
	assert.Equal(t, tg.names.Container, host)
	p, err := f.m.SSHPath("main")
	require.NoError(t, err)
	assert.Equal(t, tg.names.Container+":"+f.m.Repo.PathInCrannog(), p)
	assert.Empty(t, f.rt.calls, "asks nothing of the runtime")
}
