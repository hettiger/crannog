package crannog

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

func testCert(t *testing.T, isCA bool) []byte {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  isCA,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// generatedCA sets up a target whose proxy directory holds what
// generate-ca wrote.
func generatedCA(t *testing.T, crt []byte) (*fixture, *target) {
	f := newFixture(t)
	tg := f.target("main")
	require.NoError(t, tg.dir.mkdirs(proxyDir, caDir))
	if crt != nil {
		testfs.WriteFile(t, f.m.Home.Root, filepath.Join(tg.dir.path, proxyDir, caCertFile), string(crt), 0o644)
	}
	testfs.WriteFile(t, f.m.Home.Root, filepath.Join(tg.dir.path, proxyDir, caKeyFile), "key", 0o644)
	return f, tg
}

func TestPublishCA(t *testing.T) {
	crt := testCert(t, true)
	f, tg := generatedCA(t, crt)
	require.NoError(t, publishCA(tg))

	assert.Equal(t, string(crt), testfs.ReadFile(t, f.m.Home.Root, filepath.Join(tg.dir.path, caDir, caCertFile)))
	info, err := f.m.Home.Root.Stat(filepath.Join(tg.dir.path, proxyDir, caKeyFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestPublishCARefusesInvalidCertificates(t *testing.T) {
	tests := map[string][]byte{
		"no PEM certificate":    []byte("not a certificate"),
		"more than one":         append(testCert(t, true), testCert(t, true)...),
		"not a CA certificate":  testCert(t, false),
		"did not generate a CA": nil,
	}
	for want, crt := range tests {
		f, tg := generatedCA(t, crt)
		assert.ErrorContains(t, publishCA(tg), want)
		assert.False(t, testfs.Exists(t, f.m.Home.Root, filepath.Join(tg.dir.path, caDir, caCertFile)), want)
	}
}

// A malicious proxy image can leave symlinks to host files in the proxy
// directory. crannog must neither publish nor chmod their targets.
func TestPublishCARefusesSymlinks(t *testing.T) {
	f, tg := generatedCA(t, nil)
	user := f.userRoot()
	testfs.WriteFile(t, user, "secret", string(testCert(t, true)), 0o600)
	proxy := testfs.Root(t, tg.dir.ProxyDir())
	require.NoError(t, proxy.Symlink(filepath.Join(user.Name(), "secret"), caCertFile))
	assert.Error(t, publishCA(tg))
	assert.False(t, testfs.Exists(t, f.m.Home.Root, filepath.Join(tg.dir.path, caDir, caCertFile)))

	f, tg = generatedCA(t, testCert(t, true))
	user = f.userRoot()
	testfs.WriteFile(t, user, "tool", "", 0o755)
	proxy = testfs.Root(t, tg.dir.ProxyDir())
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
