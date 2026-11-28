package crannog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/runtime"
)

// crannog use zed opens the repository in a running crannog, after the
// same checks as crannog ssh.
func TestUseZed(t *testing.T) {
	f := newFixture(t)
	assert.EqualError(t, f.m.UseZed(""), "there is no crannog for main")

	tg := f.existing("main", defaultSetup(t))
	assert.EqualError(t, f.m.UseZed(""), "the crannog of main is not running; start it with: crannog checkout -d main")
	assert.Empty(t, f.rt.callsWith("start"), "never starts a crannog")

	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	require.NoError(t, f.m.prepareSSH(tg))
	assert.ErrorContains(t, f.m.UseZed(""), "Include ~/.crannog/ssh/config", "without the Include")
	assert.Empty(t, f.zed.opened)

	f.ssh.config["proxycommand"] = []string{"/usr/local/bin/crannog ssh-proxy --container-cli /usr/local/bin/container " + f.m.Repo.ID + " " + tg.names.Slug}
	require.NoError(t, f.m.UseZed("main"))
	assert.Equal(t, [][2]string{{tg.names.Container, f.m.Repo.PathInCrannog()}}, f.zed.opened)
	assert.Empty(t, f.out.String())
}
