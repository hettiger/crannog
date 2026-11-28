package crannog

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
	"github.com/hettiger/crannog/internal/testgit"
)

// The scripts crannog runs in a crannog, run against a local repository.

func runScript(t *testing.T, script string, args ...string) string {
	out, err := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func TestAuthorScript(t *testing.T) {
	dir := testgit.Init(t)
	runScript(t, authorScript, dir, "set", "Ada 'Lovelace'", "set", "ada@example.com")
	assert.Equal(t, "Ada 'Lovelace'", testgit.Git(t, dir, "config", "user.name"))
	assert.Equal(t, "ada@example.com", testgit.Git(t, dir, "config", "user.email"))

	runScript(t, authorScript, dir, "unset", "", "set", "new@example.com")
	_, err := testgit.Try(t, dir, "config", "user.name")
	assert.Error(t, err, "removed")
	assert.Equal(t, "new@example.com", testgit.Git(t, dir, "config", "user.email"))

	runScript(t, authorScript, dir, "unset", "", "unset", "")
}

// sshScript returns an SSH script that works in dir instead of
// sshDirInCrannog.
func sshScript(script, dir string) string {
	return strings.ReplaceAll(script, sshDirInCrannog, dir)
}

// runSSHScript runs a script with sh in a clean environment.
func runSSHScript(t *testing.T, script, stdin string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSSHKeysScript(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ssh")
	out, err := runSSHScript(t, sshScript(sshKeysScript, dir), "KEY\n", []string{"PATH=" + os.Getenv("PATH")}, "ssh-ed25519 AAAA")
	require.NoError(t, err, out)
	r := testfs.Root(t, dir)
	assert.Equal(t, "KEY\n", testfs.ReadFile(t, r, hostKeyFile))
	assert.Equal(t, "ssh-ed25519 AAAA\n", testfs.ReadFile(t, r, "authorized_keys"))
	for _, name := range []string{".", hostKeyFile, "authorized_keys"} {
		info, err := r.Stat(name)
		require.NoError(t, err)
		want := fs.FileMode(0o600)
		if name == "." {
			want = 0o700
		}
		assert.Equal(t, want, info.Mode().Perm(), name)
	}
	entries, err := fs.ReadDir(r.FS(), ".")
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temporary files are left")
}

// The sessions of sshd get the crannog's environment, except what sshd
// sets itself and values an SSH config cannot hold.
func TestSSHDConfigScript(t *testing.T) {
	out, err := runSSHScript(t, sshdConfigScript, "", []string{
		"PATH=" + os.Getenv("PATH"),
		"HTTPS_PROXY=http://crannog-proxy:8080",
		"WITH_SPACE=a b",
		"HOME=/home/agent",
		"SSH_AUTH_SOCK=/tmp/agent",
		"QUOTE=a\"b",
		"BACKSLASH=a\\b",
		"MULTILINE=a\nb",
	})
	require.NoError(t, err, out)
	assert.Contains(t, out, "HostKey "+sshDirInCrannog+"/"+hostKeyFile+"\n")
	assert.Contains(t, out, "AuthorizedKeysFile "+sshDirInCrannog+"/authorized_keys\n")
	var setEnv string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "SetEnv ") {
			require.Empty(t, setEnv, "one SetEnv line")
			setEnv = line
		}
	}
	for _, want := range []string{`"HTTPS_PROXY=http://crannog-proxy:8080"`, `"WITH_SPACE=a b"`, `"PATH=` + os.Getenv("PATH") + `"`} {
		assert.Contains(t, setEnv, want)
	}
	for _, unwanted := range []string{"HOME", "SSH_AUTH_SOCK", "QUOTE", "BACKSLASH", "MULTILINE"} {
		assert.NotContains(t, setEnv, unwanted)
	}
}

// fakeSSHD puts an sshd on PATH that prints its arguments and config.
func fakeSSHD(t *testing.T) string {
	bin := t.TempDir()
	testfs.WriteFile(t, testfs.Root(t, bin), "sshd", "#!/bin/sh\necho \"args: $*\"\ncat \"$3\"\n", 0o755)
	return bin + string(os.PathListSeparator) + os.Getenv("PATH")
}

func TestSSHDScript(t *testing.T) {
	dir := t.TempDir()
	script := sshScript(sshdScript, dir)
	env := []string{"PATH=" + fakeSSHD(t), "HTTPS_PROXY=http://crannog-proxy:8080"}

	out, err := runSSHScript(t, script, "", env)
	assert.Error(t, err)
	assert.Contains(t, out, "SSH is not prepared in this crannog; crannog checkout prepares it")

	testfs.WriteFile(t, testfs.Root(t, dir), hostKeyFile, "KEY\n", 0o600)
	out, err = runSSHScript(t, script, "", env)
	require.NoError(t, err, out)
	assert.True(t, strings.HasPrefix(out, "args: -i -f "+dir+"/sshd_config\n"), out)
	assert.Contains(t, out, "HostKey "+dir+"/"+hostKeyFile+"\n")
	assert.Contains(t, out, `"HTTPS_PROXY=http://crannog-proxy:8080"`)
	entries, err := fs.ReadDir(testfs.Root(t, dir).FS(), ".")
	require.NoError(t, err)
	assert.Len(t, entries, 2, "the key and the config, no temporary file")
}

func TestSSHDScriptWithoutSSHD(t *testing.T) {
	if _, err := runSSHScript(t, "[ -x /usr/sbin/sshd ]", "", nil); err == nil {
		t.Skip("this host has /usr/sbin/sshd")
	}
	out, err := runSSHScript(t, sshScript(sshdScript, t.TempDir()), "", []string{"PATH=/nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, out, "this crannog has no sshd; its image needs OpenSSH's server")
}

// Over SSH, the agent's shell runs crannog ssh's command in the repository
// with exactly its arguments, in the login shell.
func TestSSHCommand(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, testfs.Root(t, tmp).Mkdir("my repo", 0o700))
	dir := filepath.Join(tmp, "my repo")
	testfs.WriteFile(t, testfs.Root(t, dir), "in-repo", "", 0o600)
	env := []string{"PATH=" + os.Getenv("PATH"), "SHELL=/bin/sh", "HOME=" + t.TempDir()}
	out, err := runSSHScript(t, sshCommand(dir, []string{"sh", "-c", `ls; printf '[%s]' "$@"`, "sh", "say 'hi'", "$HOME", ""}), "", env)
	require.NoError(t, err, out)
	assert.Equal(t, "in-repo\n[say 'hi'][$HOME][]", out)

	out, err = runSSHScript(t, sshCommand(dir, nil), "ls\n", env)
	require.NoError(t, err, out)
	assert.Equal(t, "in-repo\n", out, "a login shell in dir, reading stdin")
}
