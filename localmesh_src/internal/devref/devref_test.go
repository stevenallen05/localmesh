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
	got, err := renderDoc("", proj, nil)
	if err != nil {
		t.Fatal(err)
	}
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
	got, err := renderDoc("", proj, nil)
	if err != nil {
		t.Fatal(err)
	}
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
	got, err := renderDoc("", proj, plugins)
	if err != nil {
		t.Fatal(err)
	}

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
	got, err := renderDoc("", proj, nil)
	if err != nil {
		t.Fatal(err)
	}
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

// writeSidecar is a per-test helper that writes a sidecar TOML into
// <catalogRoot>/<pluginName>/developer_reference.md.toml.
func writeSidecar(t *testing.T, catalogRoot, pluginName, body string) {
	t.Helper()
	dir := filepath.Join(catalogRoot, pluginName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "developer_reference.md.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWrite_pluginsHeadingAlwaysEmitted(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	if err := Write(repoRoot, catalogRoot, proj, nil); err != nil {
		t.Fatal(err)
	}
	got := readOut(t, repoRoot)
	if !strings.Contains(got, "## Plugins\n") {
		t.Errorf("## Plugins heading must always be present; got:\n%s", got)
	}
}

func TestWrite_pluginWithDescriptionOnly(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = "Identity provider (Dex). Hands out OIDC tokens.\nConfigure via OIDC_ISSUER env var."`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatal(err)
	}
	got := readOut(t, repoRoot)
	if !strings.Contains(got, "### auth\n") {
		t.Errorf("missing ### auth heading; got:\n%s", got)
	}
	if !strings.Contains(got, "Identity provider (Dex).") {
		t.Errorf("missing description body; got:\n%s", got)
	}
}

func TestWrite_pluginWithDescriptionAndExtras(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = "Short description."
extras = """
## Env vars
- OIDC_ISSUER
- OIDC_CLIENT_ID
"""`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatal(err)
	}
	got := readOut(t, repoRoot)
	// Description appears, blank line, then extras.
	section := got[strings.Index(got, "### auth"):]
	descIdx := strings.Index(section, "Short description.")
	envIdx := strings.Index(section, "## Env vars")
	if descIdx == -1 || envIdx == -1 || envIdx < descIdx {
		t.Errorf("description must precede extras; descIdx=%d envIdx=%d section:\n%s", descIdx, envIdx, section)
	}
	// Verify a blank line separates them.
	between := section[descIdx+len("Short description.") : envIdx]
	if !strings.Contains(between, "\n\n") {
		t.Errorf("expected blank line between description and extras; between=%q", between)
	}
}

func TestWrite_pluginExtrasEmptyStringTreatedAsAbsent(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = "Just description."
extras = ""`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatal(err)
	}
	got := readOut(t, repoRoot)
	section := got[strings.Index(got, "### auth"):]
	// Trim to just this section (up to next ### or EOF).
	if next := strings.Index(section[3:], "###"); next != -1 {
		section = section[:next+3]
	}
	// Should end with the description (plus trailing newline), with no
	// trailing blank line that would imply an empty extras body.
	trimmed := strings.TrimRight(section, "\n")
	if !strings.HasSuffix(trimmed, "Just description.") {
		t.Errorf("section should end with description when extras is empty; got:\n%q", section)
	}
}

func TestWrite_pluginWithoutSidecarOmitted(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	// No sidecar written for "postgres16".
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "postgres16"}}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatal(err)
	}
	got := readOut(t, repoRoot)
	if strings.Contains(got, "### postgres16") {
		t.Errorf("plugin without sidecar must not produce a section; got:\n%s", got)
	}
}

func TestWrite_descriptionMissingErrors(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `extras = "only extras, no description"`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	err := Write(repoRoot, catalogRoot, proj, plugins)
	if err == nil {
		t.Fatal("expected error for missing description, got nil")
	}
	if !strings.Contains(err.Error(), "description required") {
		t.Errorf("error should mention 'description required'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "developer_reference.md.toml") {
		t.Errorf("error should include the sidecar path; got: %v", err)
	}
}

func TestWrite_descriptionExceedsCapErrors(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = """
line 1
line 2
line 3
line 4
line 5
"""`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	err := Write(repoRoot, catalogRoot, proj, plugins)
	if err == nil {
		t.Fatal("expected error for >4-line description, got nil")
	}
	if !strings.Contains(err.Error(), "description must be ≤4 lines, got 5") {
		t.Errorf("error should mention 'description must be ≤4 lines, got 5'; got: %v", err)
	}
}

func TestWrite_descriptionAtCapAccepted(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = """
line 1
line 2
line 3
line 4
"""`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatalf("4-line description should be accepted; got: %v", err)
	}
}

func TestWrite_sidecarUnparseableErrors(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = "unterminated`)
	proj := &manifest.Project{Name: "p", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{{Name: "auth"}}
	err := Write(repoRoot, catalogRoot, proj, plugins)
	if err == nil {
		t.Fatal("expected error for malformed TOML, got nil")
	}
	if !strings.Contains(err.Error(), "developer_reference.md.toml") {
		t.Errorf("error should include the sidecar path; got: %v", err)
	}
}

func TestWrite_byteStableAcrossRuns(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, "localmesh"))
	catalogRoot := t.TempDir()
	writeSidecar(t, catalogRoot, "auth", `description = "Description for auth."`)
	writeSidecar(t, catalogRoot, "postgres16", `description = "Description for postgres16."
extras = "more"`)
	proj := &manifest.Project{
		Name:           "metrics-collector",
		Namespace:      "laptop",
		TechLead:       "x@example.com",
		ExternalDomain: "x.example.com",
		LocalDomain:    "lvh.me",
		Services: []manifest.Service{
			{Container: "www", Port: 3443, Scheme: "https", ExposeViaIngress: true},
		},
	}
	plugins := []*manifest.Plugin{
		{Name: "auth", Services: []manifest.Service{{Container: "dex", Port: 5556, Scheme: "http", ExposeViaIngress: true}}},
		{Name: "postgres16"},
	}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(repoRoot, "localmesh", "developer_reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(repoRoot, catalogRoot, proj, plugins); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(repoRoot, "localmesh", "developer_reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("output not byte-stable across runs:\nfirst:\n%s\n\nsecond:\n%s", first, second)
	}
}

// Helpers — keep at the bottom of devref_test.go.

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func readOut(t *testing.T, repoRoot string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "localmesh", "developer_reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
