package openssh

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// fakeSSH puts an ssh on PATH that runs script.
func fakeSSH(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	testfs.WriteFile(t, testfs.Root(t, dir), "ssh", "#!/bin/sh\n"+script+"\n", 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestConfig(t *testing.T) {
	fakeSSH(t, `[ "$*" = "-G crannog-1" ] || exit 99
printf '%s\n' 'user agent' 'proxycommand /usr/local/bin/crannog ssh-proxy a b' 'identityfile ~/.ssh/id_rsa' 'identityfile ~/.crannog/ssh/id_ed25519' 'batchmode'`)
	c, err := Client{}.Config(context.Background(), "crannog-1")
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{
		"user":         {"agent"},
		"proxycommand": {"/usr/local/bin/crannog ssh-proxy a b"},
		"identityfile": {"~/.ssh/id_rsa", "~/.crannog/ssh/id_ed25519"},
	}, c, "every value in order; a line without a value is skipped")

	fakeSSH(t, `echo "/Users/me/.ssh/config line 3: Bad configuration option: foo" >&2; exit 255`)
	_, err = Client{}.Config(context.Background(), "crannog-1")
	assert.EqualError(t, err, "ssh -G crannog-1: /Users/me/.ssh/config line 3: Bad configuration option: foo")

	fakeSSH(t, `exit 255`)
	_, err = Client{}.Config(context.Background(), "crannog-1")
	assert.ErrorContains(t, err, "ssh -G crannog-1: exit status 255")
}

func TestHostIsNoOption(t *testing.T) {
	fakeSSH(t, "exit 0")
	for _, host := range []string{"", "-oProxyCommand=sh", "-G"} {
		_, err := Client{}.Config(context.Background(), host)
		assert.ErrorContains(t, err, "invalid SSH host", host)
		assert.ErrorContains(t, Client{}.Replace(host, Options{}), "invalid SSH host", host)
	}
}

func TestMissingSSH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Client{}.Config(context.Background(), "crannog-1")
	assert.EqualError(t, err, "ssh is not in PATH")
	assert.EqualError(t, Client{}.Replace("crannog-1", Options{}), "ssh is not in PATH")
}

func TestConnectArgs(t *testing.T) {
	assert.Equal(t, []string{"ssh", "-o", "ControlPath=none", "-t", "crannog-1"},
		connectArgs("crannog-1", Options{Terminal: true}))
	assert.Equal(t, []string{"ssh", "-o", "ControlPath=none", "-T", "crannog-1", "cd /work && ls"},
		connectArgs("crannog-1", Options{Command: "cd /work && ls"}))
}
