package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
	tmpl "github.com/stevenallen05/localmesh/internal/template"
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

func TestSecurityTemplate_emitsSidecarsAndRoutes(t *testing.T) {
	path := filepath.Join("..", "..", "..", "localmesh", "service_catalog", "security", "docker-compose.yaml.gotmpl")
	ctx := &tmpl.Context{
		Project: &manifest.Project{Name: "proj", LocalDomain: "lvh.me"},
		Plugin:  &manifest.Plugin{Name: "security"},
		Env:     map[string]string{},
		Services: []tmpl.ServiceCtx{
			{Name: "www", Port: 3443, Meshed: true, ExposeViaIngress: true},
			{Name: "server", Port: 50051, Meshed: true},
			{Name: "postgres", Port: 5432, Meshed: false},
		},
	}
	out, err := tmpl.Render(path, ctx)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"www-mesh:", "server-mesh:", `network_mode: "service:www"`,
		"localmesh-bootstrap:", "ingress:",
		"ingress-route-www", "www.proj.lvh.me",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	for _, bad := range []string{"postgres-mesh:", "mesh.exempt", "ProjectName"} {
		if strings.Contains(s, bad) {
			t.Errorf("unexpected %q in output", bad)
		}
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
