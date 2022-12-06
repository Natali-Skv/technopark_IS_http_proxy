package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/pkg/errors"
)

const (
	systemCADir = "/usr/local/share/ca-certificates"
	caMaxAge    = 5 * 365 * 24 * time.Hour
	leafMaxAge  = 24 * time.Hour
	caUsage     = x509.KeyUsageDigitalSignature |
		x509.KeyUsageContentCommitment |
		x509.KeyUsageKeyEncipherment |
		x509.KeyUsageDataEncipherment |
		x509.KeyUsageKeyAgreement |
		x509.KeyUsageCertSign |
		x509.KeyUsageCRLSign
	leafUsage = caUsage
)

func GenCert(ca *tls.Certificate, names ...string) (*tls.Certificate, error) {
	now := time.Now().Add(-1 * time.Hour).UTC()
	if !ca.Leaf.IsCA {
		return nil, errors.New("CA cert is not a CA")
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: names[0]},
		NotBefore:             now,
		NotAfter:              now.Add(leafMaxAge),
		KeyUsage:              leafUsage,
		BasicConstraintsValid: true,
		DNSNames:              names,
		SignatureAlgorithm:    x509.ECDSAWithSHA512,
	}
	key, err := genKeyPair()
	if err != nil {
		return nil, err
	}
	x, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Leaf, key.Public(), ca.PrivateKey)
	if err != nil {
		return nil, err
	}
	cert := new(tls.Certificate)
	cert.Certificate = append(cert.Certificate, x)
	cert.PrivateKey = key
	cert.Leaf, _ = x509.ParseCertificate(x)
	return cert, nil
}

func genKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
}

func GenCA(name string) (certPEM, keyPEM []byte, err error) {
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now,
		NotAfter:              now.Add(caMaxAge),
		KeyUsage:              caUsage,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
		SignatureAlgorithm:    x509.ECDSAWithSHA512,
	}
	key, err := genKeyPair()
	if err != nil {
		return nil, nil, errors.Wrap(err, "error ganarating CA key pair")
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error creating CA certificate")
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error marshaling CA private key")
	}
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: keyDER,
	})
	return certPEM, keyPEM, nil
}

func LoadCA(certFile, keyFile, commonName string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	switch {
	case os.IsNotExist(err):
		var generatedCert *tls.Certificate
		generatedCert, err = genCA(certFile, keyFile, commonName)
		if err != nil {
			return nil, errors.Wrap(err, "some error while generating CA key pair from files")
		}
		cert = *generatedCert
	case os.IsPermission(err):
		return nil, errors.Wrap(err, "error opening CA certFile or CA keyFile")
	case err != nil:
		return nil, errors.Wrap(err, "some error while loading CA key pair from files")
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	return &cert, err
}

func genCA(certFile, keyFile, commonName string) (*tls.Certificate, error) {
	certPEM, keyPEM, err := GenCA(commonName)
	if err != nil {
		return nil, errors.Wrap(err, "error generating CA")
	}
	var caCert tls.Certificate
	caCert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing CA certificate from cert-PEM and key-PEM")
	}

	err = os.WriteFile(certFile, certPEM, 0400)
	if err != nil {
		return nil, errors.Wrap(err, "error writing cert-PEM to file")
	}
	err = os.WriteFile(keyFile, keyPEM, 0400)
	if err != nil {
		return nil, errors.Wrap(err, "error writing key-PEM to file")
	}

	return &caCert, err
}
