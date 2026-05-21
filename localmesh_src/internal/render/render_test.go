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

func TestServiceList_meshedAndIngress(t *testing.T) {
	f := false
	proj := &manifest.Project{Services: []manifest.Service{
		{Container: "www", Port: 3443, ExposeViaIngress: true}, // default → meshed
		{Container: "server", Port: 50051},                     // default → meshed
	}}
	plugins := []*manifest.Plugin{{Name: "postgres16", Services: []manifest.Service{
		{Container: "postgres", Port: 5432, NeedsMTLSSidecar: &f}, // exempt
	}}}
	got := serviceList(proj, plugins)
	by := map[string]struct {
		meshed, ingress bool
		port            int
	}{}
	for _, s := range got {
		by[s.Name] = struct {
			meshed, ingress bool
			port            int
		}{s.Meshed, s.ExposeViaIngress, s.Port}
	}
	if !by["www"].meshed || !by["www"].ingress || by["www"].port != 3443 {
		t.Errorf("www: %+v", by["www"])
	}
	if !by["server"].meshed || by["server"].ingress {
		t.Errorf("server: %+v", by["server"])
	}
	if by["postgres"].meshed {
		t.Errorf("postgres should be exempt: %+v", by["postgres"])
	}
	if got[0].Name != "postgres" { // sorted by name
		t.Errorf("not sorted: %v", got)
	}
}

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
