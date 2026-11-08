package crannog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

// generatedCA sets up a target whose proxy directory holds what
// generate-ca wrote.
func generatedCA(t *testing.T, crt string) (*fixture, *target) {
	f := newFixture(t)
	tg := f.target("main")
	require.NoError(t, tg.dir.mkdirs(proxyDir, caDir))
	if crt != "" {
		testfs.WriteFile(t, f.m.Home.Root, filepath.Join(tg.dir.path, proxyDir, caCertFile), crt, 0o644)
	}
	testfs.WriteFile(t, f.m.Home.Root, filepath.Join(tg.dir.path, proxyDir, caKeyFile), "key", 0o644)
	return f, tg
}

func TestPublishCA(t *testing.T) {
	f, tg := generatedCA(t, "cert")
	require.NoError(t, publishCA(tg))

	assert.Equal(t, "cert", testfs.ReadFile(t, f.m.Home.Root, filepath.Join(tg.dir.path, caDir, caCertFile)))
	info, err := f.m.Home.Root.Stat(filepath.Join(tg.dir.path, proxyDir, caKeyFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// A malicious proxy image can leave symlinks to host files in the proxy
// directory. crannog must not publish their targets.
func TestPublishCARefusesASymlinkedCertificate(t *testing.T) {
	f, tg := generatedCA(t, "")
	user := f.userRoot()
	testfs.WriteFile(t, user, "secret", "secret", 0o600)
	proxy := testfs.Root(t, tg.dir.ProxyDir())
	require.NoError(t, proxy.Symlink(filepath.Join(user.Name(), "secret"), caCertFile))

	assert.Error(t, publishCA(tg))
	assert.False(t, testfs.Exists(t, f.m.Home.Root, filepath.Join(tg.dir.path, caDir, caCertFile)))
}

// Nor may it chmod the target of a symlinked key.
func TestPublishCARefusesASymlinkedKey(t *testing.T) {
	f, tg := generatedCA(t, "cert")
	user := f.userRoot()
	testfs.WriteFile(t, user, "tool", "", 0o755)
	proxy := testfs.Root(t, tg.dir.ProxyDir())
	require.NoError(t, proxy.Remove(caKeyFile))
	require.NoError(t, proxy.Symlink(filepath.Join(user.Name(), "tool"), caKeyFile))

	assert.Error(t, publishCA(tg))
	info, err := user.Stat("tool")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "mode unchanged")
}

func TestProxyConfigReplacesSymlinks(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	tg := f.existing("main", s)
	user := f.userRoot()
	testfs.WriteFile(t, user, "host", "host", 0o644)
	proxy := testfs.Root(t, tg.dir.ProxyDir())
	require.NoError(t, proxy.Remove(proxyConfigFile))
	require.NoError(t, proxy.Symlink(filepath.Join(user.Name(), "host"), proxyConfigFile))

	_, err := readProxyFile(tg)
	assert.Error(t, err)
	require.NoError(t, f.m.writeProxyConfig(tg, s))
	assert.Equal(t, "host", testfs.ReadFile(t, user, "host"), "host file untouched")
	info, err := proxy.Lstat(proxyConfigFile)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the symlink is replaced")
}

func TestProxyMountsItsDirectoryReadOnly(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	mounts := f.rt.specs[tg.names.Proxy].Mounts
	require.Len(t, mounts, 1)
	assert.Equal(t, tg.dir.ProxyDir(), mounts[0].Source)
	assert.True(t, mounts[0].ReadOnly)
}
