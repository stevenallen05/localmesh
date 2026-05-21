package devref

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func TestRenderDoc_projectAllFields(t *testing.T) {
	proj := &manifest.Project{
		Name:           "metrics-collector",
		Namespace:      "laptop",
		TechLead:       "sallen@amberstyle.ca",
		ExternalDomain: "metrics.example.com",
		LocalDomain:    "lvh.me",
	}
	got := renderDoc(proj, nil)
	wantRows := []string{
		"| project_name      | metrics-collector              |",
		"| project_namespace | laptop                         |",
		"| tech_lead_email   | sallen@amberstyle.ca           |",
		"| external_domain   | metrics.example.com            |",
		"| local_domain      | lvh.me                         |",
	}
	for _, row := range wantRows {
		if !strings.Contains(got, row) {
			t.Errorf("missing row %q in output:\n%s", row, got)
		}
	}
	if !strings.Contains(got, "_To change any of these, edit `project.toml` and re-run `localmesh build`._") {
		t.Errorf("missing footer note; got:\n%s", got)
	}
	// Order: project_name MUST appear before project_namespace, etc.
	idxs := []int{
		strings.Index(got, "project_name"),
		strings.Index(got, "project_namespace"),
		strings.Index(got, "tech_lead_email"),
		strings.Index(got, "external_domain"),
		strings.Index(got, "local_domain"),
	}
	for i := 1; i < len(idxs); i++ {
		if idxs[i] <= idxs[i-1] {
			t.Errorf("field order wrong; indices = %v", idxs)
			break
		}
	}
}

func TestRenderDoc_projectOmitsEmptyOptionalFields(t *testing.T) {
	proj := &manifest.Project{
		Name:        "p",
		LocalDomain: "lvh.me",
		// Namespace, TechLead, ExternalDomain all empty.
	}
	got := renderDoc(proj, nil)
	if strings.Contains(got, "project_namespace") {
		t.Errorf("project_namespace row should be omitted; got:\n%s", got)
	}
	if strings.Contains(got, "tech_lead_email") {
		t.Errorf("tech_lead_email row should be omitted; got:\n%s", got)
	}
	if strings.Contains(got, "external_domain") {
		t.Errorf("external_domain row should be omitted; got:\n%s", got)
	}
	// Required fields must still render.
	if !strings.Contains(got, "project_name") {
		t.Errorf("project_name row missing; got:\n%s", got)
	}
	if !strings.Contains(got, "local_domain") {
		t.Errorf("local_domain row missing; got:\n%s", got)
	}
}

func TestRenderDoc_ingressMixedProjectAndPlugin(t *testing.T) {
	proj := &manifest.Project{
		Name:        "metrics-collector",
		LocalDomain: "lvh.me",
		Services: []manifest.Service{
			{Container: "www", Port: 3443, Scheme: "https", ExposeViaIngress: true},
			{Container: "server", Port: 50051, Scheme: "grpc", ExposeViaIngress: false},
		},
	}
	plugins := []*manifest.Plugin{
		{
			Name: "auth",
			Services: []manifest.Service{
				{Container: "dex", Port: 5556, Scheme: "http", ExposeViaIngress: true},
			},
		},
		{
			Name: "postgres16",
			Services: []manifest.Service{
				{Container: "postgres", Port: 5432, Scheme: "postgresql", ExposeViaIngress: false},
			},
		},
	}
	got := renderDoc(proj, plugins)

	wantRows := []string{
		"| https://www.metrics-collector.lvh.me:8443 | www | project |",
		"| https://dex.metrics-collector.lvh.me:8443 | dex | auth |",
	}
	for _, row := range wantRows {
		if !strings.Contains(got, row) {
			t.Errorf("missing ingress row %q in output:\n%s", row, got)
		}
	}
	if strings.Contains(got, "server") {
		t.Errorf("non-ingress service 'server' must not appear; got:\n%s", got)
	}
	if strings.Contains(got, "| postgres |") {
		t.Errorf("non-ingress service 'postgres' must not appear as an ingress row; got:\n%s", got)
	}
	// Order: project rows before plugin rows.
	wwwIdx := strings.Index(got, "www.metrics-collector")
	dexIdx := strings.Index(got, "dex.metrics-collector")
	if wwwIdx == -1 || dexIdx == -1 || wwwIdx > dexIdx {
		t.Errorf("project row must precede plugin row; wwwIdx=%d dexIdx=%d", wwwIdx, dexIdx)
	}
}

func TestRenderDoc_ingressZeroServicesEmitsHeaderOnly(t *testing.T) {
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	got := renderDoc(proj, nil)
	if !strings.Contains(got, "## Ingress\n") {
		t.Errorf("missing Ingress heading; got:\n%s", got)
	}
	// Tail of the document from `## Ingress` onward must not contain
	// any placeholder text. (At Task 3 time there is no `## Plugins`
	// section yet, so scoping the assertion to "tail from heading"
	// avoids brittle slicing on a non-existent next-heading index.)
	tail := got[strings.Index(got, "## Ingress"):]
	for _, placeholder := range []string{"_None_", "_no ingress_", "_none_"} {
		if strings.Contains(tail, placeholder) {
			t.Errorf("Ingress section must not contain %q; got:\n%s", placeholder, tail)
		}
	}
}

func TestWrite_createsFileWithHeader(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "localmesh"), 0o755); err != nil {
		t.Fatal(err)
	}
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	if err := Write(repoRoot, t.TempDir(), proj, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, "localmesh", "developer_reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.HasPrefix(got, "# Developer reference\n") {
		t.Errorf("missing title; got first 80 chars = %q", got[:min(80, len(got))])
	}
	if !strings.Contains(got, "Generated by `localmesh build`") {
		t.Errorf("missing 'generated by' note; got = %q", got)
	}
}
