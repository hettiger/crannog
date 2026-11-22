package crannog

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hettiger/crannog/internal/testfs"
)

func TestWriteCA(t *testing.T) {
	f := newFixture(t)
	tg := f.target("main")
	require.NoError(t, tg.dir.mkdirs(proxyDir, caDir))
	require.NoError(t, writeCA(tg))

	proxy := filepath.Join(tg.dir.path, proxyDir)
	crt := testfs.ReadFile(t, f.m.CrannogHome.Root, filepath.Join(proxy, caCertFile))
	assert.Equal(t, crt, testfs.ReadFile(t, f.m.CrannogHome.Root, filepath.Join(tg.dir.path, caDir, caCertFile)))
	block, _ := pem.Decode([]byte(crt))
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.True(t, cert.IsCA)
	assert.Equal(t, "crannog "+tg.names.Container, cert.Subject.CommonName)

	key := testfs.ReadFile(t, f.m.CrannogHome.Root, filepath.Join(proxy, caKeyFile))
	block, _ = pem.Decode([]byte(key))
	require.NotNil(t, block)
	_, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err, "iron-proxy reads PKCS #8 keys")
	info, err := f.m.CrannogHome.Root.Stat(filepath.Join(proxy, caKeyFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestProxyMountsItsDirectoryReadOnly(t *testing.T) {
	f := newFixture(t)
	tg := f.existing("main", defaultSetup(t))
	mounts := f.rt.specs[tg.names.Proxy].Mounts
	require.Len(t, mounts, 1)
	assert.Equal(t, tg.dir.ProxyDir(), mounts[0].Source)
	assert.True(t, mounts[0].ReadOnly)
}
