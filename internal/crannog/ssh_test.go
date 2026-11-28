package crannog

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/hettiger/crannog/internal/home"
	"github.com/hettiger/crannog/internal/runtime"
	"github.com/hettiger/crannog/internal/testfs"
)

// sshFile reads a file of the SSH causeway below ~/.crannog and checks
// that only the user can read it.
func (f *fixture) sshFile(name string) string {
	f.t.Helper()
	info, err := f.m.CrannogHome.Root.Stat(name)
	require.NoError(f.t, err)
	assert.Equal(f.t, fs.FileMode(0o600), info.Mode().Perm(), name)
	return testfs.ReadFile(f.t, f.m.CrannogHome.Root, name)
}

func (f *fixture) assertPrivateDir(name string) {
	f.t.Helper()
	info, err := f.m.CrannogHome.Root.Stat(name)
	require.NoError(f.t, err)
	assert.True(f.t, info.IsDir())
	assert.Equal(f.t, fs.FileMode(0o700), info.Mode().Perm(), name)
}

func parseKey(t *testing.T, pem string) ssh.Signer {
	t.Helper()
	k, err := ssh.ParsePrivateKey([]byte(pem))
	require.NoError(t, err)
	return k
}

// The host generates both keys and writes the user's side from what it
// knows; the crannog gets the host key and the client's public key as
// content and writes them itself.
func TestPrepareSSH(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	f.rt.containers[tg.names.Container].State = runtime.StateRunning

	require.NoError(t, f.m.prepareSSH(tg))

	f.assertPrivateDir(home.SSH)
	f.assertPrivateDir(filepath.Join(tg.dir.path, sshDir))
	assert.Equal(t, "# Written by crannog. Include it at the top of ~/.ssh/config to reach\n"+
		"# crannogs with SSH:  Include ~/.crannog/ssh/config\n"+
		"Include ~/.crannog/repositories/*/crannogs/*/ssh/config\n",
		f.sshFile(filepath.Join(home.SSH, sshConfigFile)))

	client := parseKey(t, f.sshFile(filepath.Join(home.SSH, clientKeyFile)))
	assert.Equal(t, publicKeyLine(client.PublicKey())+" crannog\n", f.sshFile(filepath.Join(home.SSH, clientKeyFile+".pub")))

	hostKeyPEM := f.sshFile(filepath.Join(tg.dir.path, sshDir, hostKeyFile))
	hostKey := parseKey(t, hostKeyPEM)
	assert.Equal(t, tg.names.Container+" "+publicKeyLine(hostKey.PublicKey())+"\n",
		f.sshFile(filepath.Join(tg.dir.path, sshDir, knownHostsFile)))

	assert.Equal(t, `# Written by crannog for the crannog of main; rewritten by every
# crannog checkout of this branch, removed by crannog destroy.
Host `+tg.names.Container+`
  HostKeyAlias `+tg.names.Container+`
  User agent
  ProxyCommand /usr/local/bin/crannog ssh-proxy --container-cli /usr/local/bin/container `+f.m.Repo.ID+` `+tg.names.Slug+`
  IdentityFile ~/.crannog/ssh/id_ed25519
  IdentitiesOnly yes
  IdentityAgent none
  ForwardAgent no
  ForwardX11 no
  PermitLocalCommand no
  StrictHostKeyChecking yes
  UpdateHostKeys no
  UserKnownHostsFile ~/.crannog/repositories/`+f.m.Repo.ID+`/crannogs/`+tg.names.Slug+`/ssh/known_hosts
`, f.sshFile(filepath.Join(tg.dir.path, sshDir, sshConfigFile)))

	assert.Equal(t, hostKeyPEM, f.rt.files[sshDirInCrannog+"/"+hostKeyFile], "the host key is passed as content")
	assert.Equal(t, publicKeyLine(client.PublicKey())+"\n", f.rt.files[sshDirInCrannog+"/authorized_keys"])

	// The keys stay: known_hosts entries and copied Host blocks remain
	// valid.
	f.m.Executable = "/elsewhere/crannog"
	f.m.ContainerCLI = "/opt/homebrew/bin/container"
	require.NoError(t, f.m.prepareSSH(tg))
	assert.Equal(t, hostKeyPEM, f.sshFile(filepath.Join(tg.dir.path, sshDir, hostKeyFile)))
	assert.Equal(t, publicKeyLine(client.PublicKey())+"\n", f.rt.files[sshDirInCrannog+"/authorized_keys"])
	assert.Contains(t, f.sshFile(filepath.Join(tg.dir.path, sshDir, sshConfigFile)),
		"ProxyCommand /elsewhere/crannog ssh-proxy --container-cli /opt/homebrew/bin/container ",
		"the paths of the running binary and of the container CLI")
}

// All crannogs share the client key; each has its own host key.
func TestSSHKeysPerCrannog(t *testing.T) {
	f := newFixture(t)
	a := f.existing("main", defaultSetup(t))
	b := f.existing("feature/x", defaultSetup(t))
	var hostKeys []string
	for _, tg := range []*target{a, b} {
		f.rt.containers[tg.names.Container].State = runtime.StateRunning
		require.NoError(t, f.m.prepareSSH(tg))
		hostKeys = append(hostKeys, f.rt.files[sshDirInCrannog+"/"+hostKeyFile])
	}
	assert.NotEqual(t, hostKeys[0], hostKeys[1])
	assert.Contains(t, f.sshFile(filepath.Join(b.dir.path, sshDir, sshConfigFile)), "Host "+b.names.Container+"\n")
}

// A client key that another crannog process created meanwhile wins.
func TestClientKeyCreatedConcurrently(t *testing.T) {
	r, _ := testfs.TempRoot(t)
	first, err := createLinked(r, clientKeyFile, "crannog")
	require.NoError(t, err)
	second, err := createLinked(r, clientKeyFile, "crannog")
	require.NoError(t, err)
	assert.Equal(t, first.PublicKey().Marshal(), second.PublicKey().Marshal())
	entries, err := fs.ReadDir(r.FS(), ".")
	require.NoError(t, err)
	require.Len(t, entries, 1, "no temporary file is left")
}

func TestBrokenKeyIsReported(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.m.CrannogHome.Root.Mkdir(home.SSH, 0o700))
	testfs.WriteFile(t, f.m.CrannogHome.Root, filepath.Join(home.SSH, clientKeyFile), "garbage", 0o600)
	_, err := ensureClientKey(f.m.CrannogHome)
	assert.ErrorContains(t, err, "delete it to have crannog create a new one")
}

func TestHostBlockQuoting(t *testing.T) {
	e := hostEntry{
		Host:         "crannog-12345678-main-0d6e4079",
		Branch:       "main",
		Executable:   "/Users/Ada Lovelace/go/bin/crannog",
		ContainerCLI: "/usr/local/bin/container",
		ID:           strings.Repeat("a", 64),
		Slug:         "main-0d6e4079",
	}
	block, err := hostBlock(e)
	require.NoError(t, err)
	assert.Contains(t, block, "  ProxyCommand '/Users/Ada Lovelace/go/bin/crannog' ssh-proxy --container-cli /usr/local/bin/container "+e.ID+" main-0d6e4079\n")

	e.Executable = "/odd/100%/crannog"
	block, err = hostBlock(e)
	require.NoError(t, err)
	assert.Contains(t, block, "ProxyCommand /odd/100%%/crannog ssh-proxy")

	e.Branch = "evil\nHost *"
	block, err = hostBlock(e)
	require.NoError(t, err)
	assert.Contains(t, block, "# Written by crannog for the crannog of evil?Host *;", "a branch cannot add lines")

	for name, change := range map[string]func(*hostEntry){
		"newline in the binary": func(e *hostEntry) { e.Executable = "/a\nb" },
		"newline in the CLI":    func(e *hostEntry) { e.ContainerCLI = "/a\nb" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := e
			change(&bad)
			_, err := hostBlock(bad)
			assert.ErrorContains(t, err, "cannot be used in an SSH config")
		})
	}

	bad := e
	bad.Slug = "a b"
	_, err = hostBlock(bad)
	assert.ErrorContains(t, err, "invalid crannog", "names in the unquoted paths are validated")
}

func TestSSHProxy(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	require.NoError(t, f.m.recordPath())
	var replaced []string
	var opts runtime.ExecOptions
	replace := func(name string, o runtime.ExecOptions, args ...string) error {
		replaced = append([]string{name}, args...)
		opts = o
		return errors.New("replaced")
	}
	proxy := func(id, s string) error { return SSHProxy(f.m.CrannogHome, f.rt, replace, id, s) }

	for _, args := range [][2]string{
		{"../" + strings.Repeat("a", 61), tg.names.Slug},
		{f.m.Repo.ID, "../main-0d6e4079"},
		{f.m.Repo.ID, ""},
	} {
		assert.ErrorContains(t, proxy(args[0], args[1]), "invalid crannog")
	}
	assert.ErrorContains(t, proxy(f.m.Repo.ID, f.target("other").names.Slug), "there is no crannog crannog-")

	err := proxy(f.m.Repo.ID, tg.names.Slug)
	assert.EqualError(t, err, "the crannog of main in "+filepath.Dir(f.m.Repo.CommonDir)+" is not running; start it with: crannog checkout -d main")
	assert.Nil(t, replaced, "never starts a crannog")
	assert.Empty(t, f.rt.callsWith("start"))

	f.rt.containers[tg.names.Container].State = runtime.StateRunning
	assert.EqualError(t, proxy(f.m.Repo.ID, tg.names.Slug), "replaced")
	assert.Equal(t, []string{tg.names.Container, "sh", "-c", sshdScript}, replaced)
	assert.Equal(t, runtime.ExecOptions{User: AgentUser, Interactive: true}, opts, "stdin and stdout carry the connection, no terminal")
}

// The SSH causeway is optional: a session starts without it.
func TestCheckoutWithoutSSH(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	f.existing("main", defaultSetup(t))
	f.rt.execHook = func(name string, args []string) error {
		if len(args) > 2 && args[2] == sshKeysScript {
			return errors.New("read-only file system")
		}
		return nil
	}
	code, err := f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	assert.Equal(t, 0, code)
	assert.Contains(t, f.out.String(), "Warning: SSH to the crannog of main is not available: writing the SSH keys in the crannog: read-only file system")
}

// checkout writes the files of the SSH causeway; destroy removes those of
// the crannog, the shared ones stay.
func TestCheckoutPreparesSSHAndDestroyRemovesIt(t *testing.T) {
	f := newFixture(t)
	stubHelper(t)
	tg := f.existing("main", defaultSetup(t))
	f.redirectURL(tg)
	_, err := f.m.Checkout("", []string{"true"})
	require.NoError(t, err, f.out.String())
	assert.NotContains(t, f.out.String(), "SSH")
	assert.Contains(t, f.sshFile(filepath.Join(tg.dir.path, sshDir, sshConfigFile)), "Host "+tg.names.Container+"\n")

	f.m.UI.Yes = true
	require.NoError(t, f.m.Destroy(""), f.out.String())
	assert.False(t, testfs.Exists(t, f.m.CrannogHome.Root, tg.dir.path))
	assert.True(t, testfs.Exists(t, f.m.CrannogHome.Root, filepath.Join(home.SSH, clientKeyFile)))
	assert.True(t, testfs.Exists(t, f.m.CrannogHome.Root, filepath.Join(home.SSH, sshConfigFile)))
}
