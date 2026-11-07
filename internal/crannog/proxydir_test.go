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
func generatedCA(t *testing.T, crt []byte) *target {
	f := newFixture(t)
	tg := f.target("main")
	for _, d := range []string{tg.dir.ProxyDir(), tg.dir.CADir()} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	if crt != nil {
		require.NoError(t, os.WriteFile(filepath.Join(tg.dir.ProxyDir(), caCertFile), crt, 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(tg.dir.ProxyDir(), caKeyFile), []byte("key"), 0o644))
	return tg
}

func TestPublishCA(t *testing.T) {
	crt := testCert(t, true)
	tg := generatedCA(t, crt)
	require.NoError(t, publishCA(tg))

	published, err := os.ReadFile(filepath.Join(tg.dir.CADir(), caCertFile))
	require.NoError(t, err)
	assert.Equal(t, crt, published)
	info, err := os.Stat(filepath.Join(tg.dir.ProxyDir(), caKeyFile))
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
		tg := generatedCA(t, crt)
		assert.ErrorContains(t, publishCA(tg), want)
		assert.NoFileExists(t, filepath.Join(tg.dir.CADir(), caCertFile), want)
	}
}

// A malicious proxy image can leave symlinks to host files in the proxy
// directory. crannog must neither publish nor chmod their targets.
func TestPublishCARefusesSymlinks(t *testing.T) {
	host := t.TempDir()
	secret := filepath.Join(host, "secret")
	require.NoError(t, os.WriteFile(secret, testCert(t, true), 0o600))
	executable := filepath.Join(host, "tool")
	require.NoError(t, os.WriteFile(executable, nil, 0o755))

	tg := generatedCA(t, nil)
	require.NoError(t, os.Symlink(secret, filepath.Join(tg.dir.ProxyDir(), caCertFile)))
	assert.Error(t, publishCA(tg))
	assert.NoFileExists(t, filepath.Join(tg.dir.CADir(), caCertFile))

	tg = generatedCA(t, testCert(t, true))
	key := filepath.Join(tg.dir.ProxyDir(), caKeyFile)
	require.NoError(t, os.Remove(key))
	require.NoError(t, os.Symlink(executable, key))
	assert.Error(t, publishCA(tg))
	info, err := os.Stat(executable)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "mode unchanged")
}

func TestProxyConfigReplacesSymlinks(t *testing.T) {
	f := newFixture(t)
	s := defaultSetup(t)
	tg := f.existing("main", s)
	hostFile := filepath.Join(t.TempDir(), "host")
	require.NoError(t, os.WriteFile(hostFile, []byte("host"), 0o644))
	p := filepath.Join(tg.dir.ProxyDir(), proxyConfigFile)
	require.NoError(t, os.Remove(p))
	require.NoError(t, os.Symlink(hostFile, p))

	_, err := readProxyFile(tg)
	assert.Error(t, err)
	require.NoError(t, f.m.writeProxyConfig(tg, s))
	data, err := os.ReadFile(hostFile)
	require.NoError(t, err)
	assert.Equal(t, "host", string(data), "host file untouched")
	info, err := os.Lstat(p)
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
