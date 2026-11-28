package zed

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// fakeZed puts a zed on PATH that runs script.
func fakeZed(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	testfs.WriteFile(t, testfs.Root(t, dir), "zed", "#!/bin/sh\n"+script+"\n", 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestOpenSSH(t *testing.T) {
	fakeZed(t, `[ "$*" = "ssh://crannog-1/work/Users/me/my%20app" ] || exit 99`)
	require.NoError(t, CLI{}.OpenSSH("crannog-1", "/work/Users/me/my app"))

	fakeZed(t, `echo "zed: cannot connect to Zed" >&2; exit 1`)
	assert.EqualError(t, CLI{}.OpenSSH("crannog-1", "/work/a"), "zed ssh://crannog-1/work/a: zed: cannot connect to Zed")

	fakeZed(t, `exit 2`)
	assert.EqualError(t, CLI{}.OpenSSH("crannog-1", "/work/a"), "zed ssh://crannog-1/work/a: exit status 2")
}

func TestOpenSSHRefusesInvalidArguments(t *testing.T) {
	fakeZed(t, "exit 0")
	for _, host := range []string{"", "user@crannog-1", "crannog-1:22", "a/b"} {
		assert.ErrorContains(t, CLI{}.OpenSSH(host, "/work"), "invalid SSH host", host)
	}
	assert.ErrorContains(t, CLI{}.OpenSSH("crannog-1", "work"), "not an absolute path")
}

func TestMissingZed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	assert.EqualError(t, CLI{}.OpenSSH("crannog-1", "/work"),
		"zed is not in PATH; install Zed's command line tool in Zed with Zed > Install CLI")
}

func TestSSHURL(t *testing.T) {
	assert.Equal(t, "ssh://crannog-1/work/a%23b/c%3Fd", sshURL("crannog-1", "/work/a#b/c?d"))
}
