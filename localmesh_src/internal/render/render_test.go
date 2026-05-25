package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestRun_sampleCatalog(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "bundled.compose.yaml")
	root := filepath.Join("..", "..", "testdata", "sample_catalog")
	if err := Run(filepath.Join(root, "project.toml"), root, out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("testdata", "expected.compose.yaml")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to regen)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output drifted from golden:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestServiceList_protocolAndIngress(t *testing.T) {
	proj := &manifest.Project{Services: []manifest.Service{
		{Container: "www", Port: 3443, Scheme: "http", ExposeViaIngress: true},
		{Container: "server", Port: 50051, Scheme: "grpc"},
	}}
	plugins := []*manifest.Plugin{{Name: "postgres16", Services: []manifest.Service{
		{Container: "postgres", Port: 5432, Scheme: "postgresql"},
	}}}
	got, err := serviceList(proj, plugins)
	if err != nil {
		t.Fatalf("serviceList: %v", err)
	}
	by := map[string]struct {
		protocol, plugin string
		ingress          bool
		port             int
	}{}
	for _, s := range got {
		by[s.Name] = struct {
			protocol, plugin string
			ingress          bool
			port             int
		}{s.Protocol, s.Plugin, s.ExposeViaIngress, s.Port}
	}
	if by["www"].protocol != "http" || by["www"].plugin != "app" || !by["www"].ingress || by["www"].port != 3443 {
		t.Errorf("www: %+v", by["www"])
	}
	if by["server"].protocol != "grpc" || by["server"].plugin != "app" || by["server"].ingress {
		t.Errorf("server: %+v", by["server"])
	}
	if by["postgres"].protocol != "tcp" || by["postgres"].plugin != "postgres16" {
		t.Errorf("postgres: %+v", by["postgres"])
	}
	if got[0].Name != "postgres" { // sorted by name
		t.Errorf("not sorted: %v", got)
	}
}

// TestSecurityTemplate_* used to validate the live security/ plugin's
// docker-compose.yaml.gotmpl output. The plugin moved into
// archive/legacy_plugins/security/ when the catalog was reshaped around the
// config inversion (see archive/legacy_plugins/README.md). The tests reached
// into the live catalog from internal/ — a data-source-rule violation that
// also broke once the path moved — and were deleted rather than re-pointed.
// If the security plugin is re-promoted, write fresh tests against a fixture
// under testdata/, not against the live catalog path.

func TestRun_idempotent(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "bundled.compose.yaml")
	root := filepath.Join("..", "..", "testdata", "sample_catalog")
	if err := Run(filepath.Join(root, "project.toml"), root, out); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(out)
	if err := Run(filepath.Join(root, "project.toml"), root, out); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(out)
	if !bytes.Equal(first, second) {
		t.Fatalf("Run is not idempotent")
	}
}
