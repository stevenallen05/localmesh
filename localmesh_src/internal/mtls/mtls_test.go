package mtls

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
)

func writeTestCA(t *testing.T, caRoot string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(caRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(filepath.Join(caRoot, "rootCA.pem"), certPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(filepath.Join(caRoot, "rootCA-key.pem"), keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMint_signsLeafWithExpectedSANs(t *testing.T) {
	repoRoot := t.TempDir()
	writeTestCA(t, filepath.Join(repoRoot, ".localmesh", "secrets", "root_ca"))
	m, err := New(repoRoot, "metrics-collector", "lvh.me")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Mint("server"); err != nil {
		t.Fatalf("Mint: %v", err)
	}
	leafPath := filepath.Join(repoRoot, ".localmesh", "secrets", "server", "id.crt")
	pemBytes, err := os.ReadFile(leafPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.URIs) != 1 || cert.URIs[0].String() != "spiffe://server.metrics-collector.lvh.me" {
		t.Errorf("URI SAN = %v, want spiffe://server.metrics-collector.lvh.me", cert.URIs)
	}
	wantDNS := []string{"server", "server.metrics-collector.lvh.me"}
	if len(cert.DNSNames) != len(wantDNS) {
		t.Errorf("DNSNames = %v, want %v", cert.DNSNames, wantDNS)
	}
	wantNotAfter := time.Now().Add(7 * 24 * time.Hour)
	if cert.NotAfter.After(wantNotAfter.Add(time.Minute)) || cert.NotAfter.Before(wantNotAfter.Add(-time.Minute)) {
		t.Errorf("NotAfter = %v, want ~7 days from now", cert.NotAfter)
	}
}

func TestMint_keyFileMode0600(t *testing.T) {
	repoRoot := t.TempDir()
	writeTestCA(t, filepath.Join(repoRoot, ".localmesh", "secrets", "root_ca"))
	m, err := New(repoRoot, "x", "lvh.me")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Mint("worker"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(repoRoot, ".localmesh", "secrets", "worker", "id.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("id.key mode = %v, want 0600", info.Mode().Perm())
	}
}
