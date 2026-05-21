// Package mtls mints leaf certs signed by mkcert's root CA per spec §2.6.
// Always regenerates; 7-day lifetime to build prod cert-rotation habit.
package mtls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// LeafLifetime — 7 days per spec §2.6. Forces regular rotation in dev to
// build the prod habit (where SPIRE SVIDs rotate ~1h).
// TODO: needs_prod_decisions 7-day dev leaf cert lifetime; prod uses SPIRE ~1h
const LeafLifetime = 7 * 24 * time.Hour

// IPSANs are the over-permissive dev set. Allows dev curls and library
// clients dialing by various IP forms.
// TODO: needs_prod_decisions IP SAN list tightening; prod uses DNS-only identity
var IPSANs = []net.IP{
	net.ParseIP("127.0.0.1"), net.ParseIP("::1"),
	net.ParseIP("0.0.0.0"), net.ParseIP("::"),
}

// Minter signs leaf certs using a loaded root CA.
//
// caKey is held as a crypto.Signer so we accept whatever algorithm the
// CA happens to use. mkcert v1.4.4 mints an RSA-3072 CA by default;
// the in-memory test CA uses ECDSA P256.
type Minter struct {
	caRoot      string // localmesh/secrets/root_ca
	outputRoot  string // localmesh/secrets
	projectName string
	localDomain string
	caCert      *x509.Certificate
	caKey       crypto.Signer
}

// New loads mkcert's rootCA.pem + rootCA-key.pem and returns a Minter.
func New(repoRoot, projectName, localDomain string) (*Minter, error) {
	caRoot := filepath.Join(repoRoot, "localmesh", "secrets", "root_ca")
	outRoot := filepath.Join(repoRoot, "localmesh", "secrets")
	caCert, caKey, err := loadCA(caRoot)
	if err != nil {
		return nil, err
	}
	return &Minter{
		caRoot:      caRoot,
		outputRoot:  outRoot,
		projectName: projectName,
		localDomain: localDomain,
		caCert:      caCert,
		caKey:       caKey,
	}, nil
}

// Mint writes id.crt / id.key / trust.ca.crt for one container.
// Always overwrites — assumes regular rotation.
func (m *Minter) Mint(container string) error {
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key for %s: %w", container, err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("serial for %s: %w", container, err)
	}
	spiffe := &url.URL{Scheme: "spiffe", Host: fmt.Sprintf("%s.%s.%s", container, m.projectName, m.localDomain)}
	dnsNames := []string{
		container,
		fmt.Sprintf("%s.%s.%s", container, m.projectName, m.localDomain),
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: container},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(LeafLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              dnsNames,
		IPAddresses:           IPSANs,
		URIs:                  []*url.URL{spiffe},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, m.caCert, &leafKey.PublicKey, m.caKey)
	if err != nil {
		return fmt.Errorf("sign cert for %s: %w", container, err)
	}
	outDir := filepath.Join(m.outputRoot, container)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	// id.crt
	if err := writePEM(filepath.Join(outDir, "id.crt"), "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	// id.key (0600 — postgres + similar consumers enforce strict perms)
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		return fmt.Errorf("marshal key for %s: %w", container, err)
	}
	if err := writePEM(filepath.Join(outDir, "id.key"), "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}
	// trust.ca.crt — copy of the root CA cert for the consumer
	if err := writePEMFromCert(filepath.Join(outDir, "trust.ca.crt"), m.caCert); err != nil {
		return err
	}
	return nil
}

// MintAll runs Mint for every container in the list, returning the first error.
func (m *Minter) MintAll(containers []string) error {
	for _, c := range containers {
		if err := m.Mint(c); err != nil {
			return err
		}
	}
	return nil
}

func loadCA(caRoot string) (*x509.Certificate, crypto.Signer, error) {
	certPath := filepath.Join(caRoot, "rootCA.pem")
	keyPath := filepath.Join(caRoot, "rootCA-key.pem")
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA cert %s — run `localmesh ca mint --force` first: %w", certPath, err)
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA key %s: %w", keyPath, err)
	}
	certBlock, _ := pem.Decode(certBytes)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("CA cert PEM decode failed at %s", certPath)
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}
	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("CA key PEM decode failed at %s", keyPath)
	}
	// mkcert v1.4.4 writes PKCS8-encoded RSA; older versions wrote EC.
	// Try PKCS8 first, fall back to EC-specific parser for legacy CAs.
	if key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes); err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return cert, k, nil
		case *ecdsa.PrivateKey:
			return cert, k, nil
		default:
			return nil, nil, fmt.Errorf("CA key algorithm unsupported (got %T)", key)
		}
	}
	if ecKey, err := x509.ParseECPrivateKey(keyBlock.Bytes); err == nil {
		return cert, ecKey, nil
	}
	if rsaKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes); err == nil {
		return cert, rsaKey, nil
	}
	return nil, nil, fmt.Errorf("CA key %s: unrecognized PEM (tried PKCS8, EC, PKCS1)", keyPath)
}

func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, pemBytes, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writePEMFromCert(path string, cert *x509.Certificate) error {
	return writePEM(path, "CERTIFICATE", cert.Raw, 0o644)
}
