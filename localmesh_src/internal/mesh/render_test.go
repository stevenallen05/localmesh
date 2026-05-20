package mesh

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

var update = flag.Bool("update", false, "update golden files")

func TestBuildContexts(t *testing.T) {
	proj := &manifest.Project{Name: "demo", LocalDomain: "lvh.me", ExternalDomain: "demo.example.com"}
	plugins := []*manifest.Plugin{
		{Name: "mesh", Identity: manifest.Identity{ModuleName: "mesh", OwnedBy: "sre@example.com"}, Services: []manifest.Service{
			{Container: "ingress-mesh", Port: 8443, Scheme: "https", Ingress: true},
			{Container: "egress-mesh", Port: 15001, Scheme: "tcp"},
		}},
		{Name: "app-b", Identity: manifest.Identity{ModuleName: "app", OwnedBy: "tl@example.com"}, Services: []manifest.Service{
			{Container: "app-b", Port: 50051, Scheme: "grpc"},
		}},
	}
	proj.Services = []manifest.Service{
		{Container: "app-a", Port: 3000, Scheme: "http", ExposeViaIngress: true},
	}
	compose := ComposeAggregate{
		"app-a":        {DependsOn: []string{"app-b"}, Labels: map[string]string{}},
		"app-b":        {DependsOn: nil, Labels: map[string]string{}},
		"ingress-mesh": {DependsOn: nil, Labels: map[string]string{}},
		"egress-mesh":  {DependsOn: nil, Labels: map[string]string{}},
	}
	ctxs, err := BuildContexts(proj, plugins, compose)
	if err != nil {
		t.Fatalf("build contexts: %v", err)
	}
	if len(ctxs.Sidecars) != 2 {
		t.Errorf("expected 2 sidecar contexts (app-a, app-b), got %d", len(ctxs.Sidecars))
	}
	// ingress + egress contexts always present
	if ctxs.Ingress.ProjectName != "demo" {
		t.Errorf("ingress ctx project = %q, want demo", ctxs.Ingress.ProjectName)
	}
	if len(ctxs.Ingress.Routes) == 0 {
		t.Errorf("expected at least one ingress route (app-a expose_via_ingress=true)")
	}

	// AppLoopbackPort convention: InboundPort + 1
	if sc := ctxs.Sidecars["app-a"]; sc.AppLoopbackPort != sc.InboundPort+1 {
		t.Errorf("app-a AppLoopbackPort = %d, want InboundPort+1 = %d", sc.AppLoopbackPort, sc.InboundPort+1)
	}
}

// TestRenderAll_GoldenFiles exercises RenderAll end-to-end against the
// sample_catalog fixture and golden-diffs every emitted file. Run with
// -update to (re)generate the golden directory.
func TestRenderAll_GoldenFiles(t *testing.T) {
	fixturesRoot := filepath.Join("..", "..", "testdata", "sample_catalog")
	proj, plugins, err := manifest.LoadAll(
		filepath.Join(fixturesRoot, "service_catalog", "project.toml"),
		filepath.Join(fixturesRoot, "service_catalog"),
	)
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	compose := ComposeAggregate{
		"app-a":        {DependsOn: []string{"app-b"}, Labels: map[string]string{}},
		"app-b":        {DependsOn: nil, Labels: map[string]string{}},
		"ingress-mesh": {DependsOn: nil, Labels: map[string]string{}},
		"egress-mesh":  {DependsOn: nil, Labels: map[string]string{}},
	}
	outDir := t.TempDir()
	templatesDir := filepath.Join("..", "..", "..", "service_catalog", "mesh", "templates")
	if err := RenderAll(proj, plugins, compose, outDir, templatesDir); err != nil {
		t.Fatalf("render all: %v", err)
	}
	expected := []string{
		"ingress-mesh.yaml", "egress-mesh.yaml",
		"app-a-mesh.yaml", "app-b-mesh.yaml",
		"app-a-mesh-init.sh", "app-b-mesh-init.sh",
	}
	for _, name := range expected {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("missing output %s: %v", name, err)
		}
	}
	goldenDir := filepath.Join("..", "..", "testdata", "golden")
	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
	}
	for _, name := range expected {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("read output %s: %v", name, err)
		}
		goldenPath := filepath.Join(goldenDir, name)
		if *update {
			if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
				t.Fatalf("write golden %s: %v", goldenPath, err)
			}
			continue
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("missing golden %s; rerun with -update: %v", goldenPath, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from golden:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
		}
	}
}
