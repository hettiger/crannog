package crannog

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"github.com/google/renameio/v2"
)

// caValidity is how long a crannog's CA is valid.
const caValidity = 20 * 365 * 24 * time.Hour

// newCA generates a CA for the proxy causeway: a self-signed certificate
// and its PKCS #8 key, both PEM-encoded.
func newCA(name string) (crt, key []byte, err error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caValidity),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	if err != nil {
		return nil, nil, err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), nil
}

// writeCA generates the crannog's CA: certificate and key for the proxy,
// the certificate alone for the crannog.
func writeCA(t *target) error {
	crt, key, err := newCA("crannog " + t.names.Container)
	if err != nil {
		return err
	}
	proxy, err := t.dir.open(proxyDir, false)
	if err != nil {
		return err
	}
	defer proxy.Close()
	if err := renameio.WriteFile(caKeyFile, key, 0o600, renameio.WithRoot(proxy)); err != nil {
		return err
	}
	if err := renameio.WriteFile(caCertFile, crt, 0o644, renameio.WithRoot(proxy)); err != nil {
		return err
	}
	ca, err := t.dir.open(caDir, false)
	if err != nil {
		return err
	}
	defer ca.Close()
	return renameio.WriteFile(caCertFile, crt, 0o644, renameio.WithRoot(ca))
}
