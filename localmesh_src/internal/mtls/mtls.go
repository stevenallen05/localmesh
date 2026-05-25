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
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
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

// IngressEdgeDir is the per-cert directory holding the browser-facing
// TLS bundle. Keyed by "ingress-tls" (not "ingress") to keep mesh
// identity (per-workload SPIFFE leaf at .../ingress/) decoupled from
// edge TLS — they have different rotation needs and likely diverge in
// prod (cert-manager / Let's Encrypt vs SPIRE).
const ingressEdgeDirName = "ingress-tls"
const ingressEdgeSecretName = "ingress-tls"

// MintIngressEdge mints a wildcard leaf signed by the LocalMesh CA for
// browser-facing TLS termination on the kuma ingress MeshGateway.
// Writes id.crt, id.key, trust.ca.crt, and a kuma `Secret` YAML into
// localmesh/secrets/ingress-tls/.
//
// SAN scope is wildcard (`*.<project>.<local>`) + IPSANs only — explicit
// per-FQDN SANs are deliberately omitted so adding a new ingress service
// doesn't require re-minting the cert. See design doc 2026-05-21-ingress-
// edge-tls for the trade-off.
func (m *Minter) MintIngressEdge() error {
	hostname := fmt.Sprintf("*.%s.%s", m.projectName, m.localDomain)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ingress-edge key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("ingress-edge serial: %w", err)
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hostname},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(LeafLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, // terminate-only; no ClientAuth
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{hostname},
		IPAddresses:           IPSANs,
		// No URIs — edge TLS is hostname-keyed, not workload-keyed.
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, m.caCert, &leafKey.PublicKey, m.caKey)
	if err != nil {
		return fmt.Errorf("sign ingress-edge cert: %w", err)
	}

	outDir := filepath.Join(m.outputRoot, ingressEdgeDirName)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}
	certPath := filepath.Join(outDir, "id.crt")
	if err := writePEM(certPath, "CERTIFICATE", der, 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		return fmt.Errorf("marshal ingress-edge key: %w", err)
	}
	keyPath := filepath.Join(outDir, "id.key")
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}
	if err := writePEMFromCert(filepath.Join(outDir, "trust.ca.crt"), m.caCert); err != nil {
		return err
	}
	return writeIngressEdgeSecretYAML(outDir, certPath, keyPath)
}

// writeIngressEdgeSecretYAML reads the just-written cert + key PEMs and
// emits a kuma `Secret` YAML containing the base64-encoded concatenation.
// Production code path uses fmt.Sprintf (4-line file, not worth a YAML
// library dep); tests parse with gopkg.in/yaml.v3 to verify the shape.
//
// File mode is 0644 even though the body encodes the private key. The
// file is a transient bootstrap artifact: docker compose mounts it as a
// config into the localmesh-bootstrap container, which kumactl-applies
// it once and exits. It is not on a long-lived mount path. The id.key
// PEM written alongside (mode 0600) remains the canonical key file for
// any consumer that needs strict perms.
func writeIngressEdgeSecretYAML(outDir, certPath, keyPath string) error {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read %s for secret yaml: %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read %s for secret yaml: %w", keyPath, err)
	}
	bundle := append(append([]byte{}, certPEM...), keyPEM...)
	encoded := base64.StdEncoding.EncodeToString(bundle)
	body := fmt.Sprintf("type: Secret\nmesh: default\nname: %s\ndata: %s\n", ingressEdgeSecretName, encoded)
	yamlPath := filepath.Join(outDir, "secret.yaml")
	if err := os.WriteFile(yamlPath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", yamlPath, err)
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
