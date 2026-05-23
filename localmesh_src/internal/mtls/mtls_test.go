package mtls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stevenallen05/localmesh/internal/ca"
	"gopkg.in/yaml.v3"
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

// newTestMinter is a one-call fixture: write a fresh in-memory CA into
// <repoRoot>/localmesh/secrets/root_ca/, then load it into a Minter
// scoped to that repoRoot. Wraps the existing writeTestCA + New pattern
// so per-test setup stays tight.
func newTestMinter(t *testing.T, repoRoot, projectName, localDomain string) *Minter {
	t.Helper()
	caRoot := filepath.Join(repoRoot, "localmesh", "secrets", "root_ca")
	writeTestCA(t, caRoot)
	m, err := New(repoRoot, projectName, localDomain)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

// TestSecretsPath_caAndMtlsAgree is a regression guard. The package
// constants in internal/ca and internal/mtls used to drift between
// `.localmesh/secrets/...` and `localmesh/secrets/...`, which broke
// `make certs` end-to-end because the Makefile and docker-compose
// secrets paths only spoke the no-dot form. This test fails loudly if
// either package's constant strays from the agreed shape.
func TestSecretsPath_caAndMtlsAgree(t *testing.T) {
	repoRoot := t.TempDir()
	wantCARoot := filepath.Join(repoRoot, "localmesh", "secrets", "root_ca")
	if got := ca.New(repoRoot).CARoot; got != wantCARoot {
		t.Errorf("ca.New(%q).CARoot = %q, want %q", repoRoot, got, wantCARoot)
	}
	// mtls.New loads the CA at construction; give it a stub CA so we
	// can reach the path-only assertion below.
	writeTestCA(t, wantCARoot)
	m, err := New(repoRoot, "metrics-collector", "lvh.me")
	if err != nil {
		t.Fatalf("mtls.New: %v", err)
	}
	wantOutRoot := filepath.Join(repoRoot, "localmesh", "secrets")
	if m.outputRoot != wantOutRoot {
		t.Errorf("mtls.New(%q).outputRoot = %q, want %q", repoRoot, m.outputRoot, wantOutRoot)
	}
	if m.caRoot != wantCARoot {
		t.Errorf("mtls.New(%q).caRoot = %q, want %q", repoRoot, m.caRoot, wantCARoot)
	}
}

func TestMintIngressEdge_writesPEMAndYAML(t *testing.T) {
	repoRoot := t.TempDir()
	m := newTestMinter(t, repoRoot, "metrics-collector", "lvh.me")
	if err := m.MintIngressEdge(); err != nil {
		t.Fatalf("MintIngressEdge: %v", err)
	}
	dir := filepath.Join(repoRoot, "localmesh", "secrets", "ingress-tls")
	for _, name := range []string{"id.crt", "id.key", "trust.ca.crt", "secret.yaml"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s, stat err: %v", path, err)
		}
		if name == "id.key" && info.Mode().Perm() != 0o600 {
			t.Errorf("id.key mode = %o, want 0600", info.Mode().Perm())
		}
		if (name == "id.crt" || name == "trust.ca.crt" || name == "secret.yaml") && info.Mode().Perm() != 0o644 {
			t.Errorf("%s mode = %o, want 0644", name, info.Mode().Perm())
		}
	}
}

func TestMintIngressEdge_certSANs(t *testing.T) {
	repoRoot := t.TempDir()
	m := newTestMinter(t, repoRoot, "metrics-collector", "lvh.me")
	if err := m.MintIngressEdge(); err != nil {
		t.Fatalf("MintIngressEdge: %v", err)
	}
	certPEM, err := os.ReadFile(filepath.Join(repoRoot, "localmesh", "secrets", "ingress-tls", "id.crt"))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("decode id.crt PEM failed")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	wantDNS := []string{"*.metrics-collector.lvh.me"}
	if !reflect.DeepEqual(cert.DNSNames, wantDNS) {
		t.Errorf("DNSNames = %v, want %v", cert.DNSNames, wantDNS)
	}
	// Compare IPs by string form — sidesteps net.IP's 4-byte-vs-16-byte
	// encoding ambiguity across the x509 round-trip.
	wantIPs := make([]string, len(IPSANs))
	for i, ip := range IPSANs {
		wantIPs[i] = ip.String()
	}
	gotIPs := make([]string, len(cert.IPAddresses))
	for i, ip := range cert.IPAddresses {
		gotIPs[i] = ip.String()
	}
	if !reflect.DeepEqual(gotIPs, wantIPs) {
		t.Errorf("IPAddresses = %v, want %v", gotIPs, wantIPs)
	}
	if len(cert.URIs) != 0 {
		t.Errorf("URIs len = %d, want 0 (edge TLS has no SPIFFE URI)", len(cert.URIs))
	}
	if cert.Subject.CommonName != "*.metrics-collector.lvh.me" {
		t.Errorf("CN = %q, want %q", cert.Subject.CommonName, "*.metrics-collector.lvh.me")
	}
}

func TestMintIngressEdge_secretYAMLRoundTrips(t *testing.T) {
	repoRoot := t.TempDir()
	m := newTestMinter(t, repoRoot, "metrics-collector", "lvh.me")
	if err := m.MintIngressEdge(); err != nil {
		t.Fatalf("MintIngressEdge: %v", err)
	}
	dir := filepath.Join(repoRoot, "localmesh", "secrets", "ingress-tls")
	idCrt, _ := os.ReadFile(filepath.Join(dir, "id.crt"))
	idKey, _ := os.ReadFile(filepath.Join(dir, "id.key"))
	want := append(append([]byte{}, idCrt...), idKey...)

	yamlBody, err := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Type string `yaml:"type"`
		Mesh string `yaml:"mesh"`
		Name string `yaml:"name"`
		Data string `yaml:"data"`
	}
	if err := yaml.Unmarshal(yamlBody, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal: %v\nbody:\n%s", err, yamlBody)
	}
	if parsed.Type != "Secret" || parsed.Mesh != "default" || parsed.Name != "ingress-tls" {
		t.Errorf("yaml header wrong: %+v", parsed)
	}
	decoded, err := base64.StdEncoding.DecodeString(parsed.Data)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if !bytes.Equal(decoded, want) {
		t.Errorf("decoded data does not match id.crt || id.key concatenation\nwant first 40 bytes: %q\ngot:  %q", want[:40], decoded[:min(40, len(decoded))])
	}
}

func TestMintIngressEdge_idempotent(t *testing.T) {
	repoRoot := t.TempDir()
	m := newTestMinter(t, repoRoot, "metrics-collector", "lvh.me")
	if err := m.MintIngressEdge(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repoRoot, "localmesh", "secrets", "ingress-tls")
	first, _ := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	if err := m.MintIngressEdge(); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "secret.yaml"))
	// Header lines (type/mesh/name) must be byte-identical across runs.
	// Cert + key bytes legitimately differ run-to-run (new ECDSA key, new
	// serial), so we don't compare the data: line.
	firstHeader := strings.SplitN(string(first), "data:", 2)[0]
	secondHeader := strings.SplitN(string(second), "data:", 2)[0]
	if firstHeader != secondHeader {
		t.Errorf("secret.yaml header drifted across runs:\nfirst:\n%s\nsecond:\n%s", firstHeader, secondHeader)
	}
}
