package open

import (
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// fakeOpen makes open a script that runs script.
func fakeOpen(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	testfs.WriteFile(t, testfs.Root(t, dir), "open", "#!/bin/sh\n"+script+"\n", 0o755)
	saved := program
	program = filepath.Join(dir, "open")
	t.Cleanup(func() { program = saved })
}

func TestZedSSH(t *testing.T) {
	fakeOpen(t, `[ "$*" = "zed://ssh/crannog-1/work/Users/me/my%20app" ] || exit 99`)
	require.NoError(t, Opener{}.ZedSSH("crannog-1", "/work/Users/me/my app"))

	fakeOpen(t, `echo "No application knows how to open URL zed://ssh/crannog-1/work/a" >&2; exit 1`)
	assert.EqualError(t, Opener{}.ZedSSH("crannog-1", "/work/a"),
		"cannot open zed://ssh/crannog-1/work/a: No application knows how to open URL zed://ssh/crannog-1/work/a (is Zed installed?)")

	fakeOpen(t, `exit 2`)
	assert.EqualError(t, Opener{}.ZedSSH("crannog-1", "/work/a"),
		"cannot open zed://ssh/crannog-1/work/a: exit status 2 (is Zed installed?)")
}

func TestZedSSHRefusesInvalidArguments(t *testing.T) {
	fakeOpen(t, "exit 0")
	for _, host := range []string{"", "user@crannog-1", "crannog-1:22", "a/b"} {
		assert.ErrorContains(t, Opener{}.ZedSSH(host, "/work"), "invalid SSH host", host)
	}
	assert.ErrorContains(t, Opener{}.ZedSSH("crannog-1", "work"), "not an absolute path")
}

func TestZedSSHURL(t *testing.T) {
	assert.Equal(t, "zed://ssh/crannog-1/work/a%23b/c%3Fd", zedSSHURL("crannog-1", "/work/a#b/c?d"))
}

func TestWeb(t *testing.T) {
	fakeOpen(t, `[ "$*" = "https://192.168.64.5:8080" ] || exit 99`)
	require.NoError(t, Opener{}.Web(false, netip.MustParseAddr("192.168.64.5"), 8080))
	assert.ErrorContains(t, Opener{}.Web(false, netip.Addr{}, 8080), "invalid address")
}

func TestWebURL(t *testing.T) {
	ip := netip.MustParseAddr("192.168.64.5")
	assert.Equal(t, "https://192.168.64.5", WebURL(false, ip, 0))
	assert.Equal(t, "http://192.168.64.5", WebURL(true, ip, 0))
	assert.Equal(t, "https://192.168.64.5:8080", WebURL(false, ip, 8080))
	assert.Equal(t, "http://192.168.64.5:8080", WebURL(true, ip, 8080))
	assert.Equal(t, "https://[fd00::5]:8080", WebURL(false, netip.MustParseAddr("fd00::5"), 8080))
}

// Tests never run a program outside the temporary directory, like the
// real open.
func TestOpenOutsideTheTempDirIsRefused(t *testing.T) {
	saved := program
	program = "/bin/sh"
	t.Cleanup(func() { program = saved })
	assert.ErrorContains(t, Opener{}.ZedSSH("crannog-1", "/work"), "tests must run a fake open in a temporary directory, not /bin/sh")
}
