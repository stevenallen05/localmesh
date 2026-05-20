package envwriter

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func sampleProject() *manifest.Project {
	return &manifest.Project{
		Name:           "demo",
		Namespace:      "test",
		TechLead:       "x@y.z",
		ExternalDomain: "demo.example.com",
		LocalDomain:    "lvh.me",
		Services: []manifest.Service{
			{Container: "www", Port: 3443, ExposeViaIngress: true},
		},
	}
}

func samplePlugins() []*manifest.Plugin {
	return []*manifest.Plugin{
		{
			Name:     "caddy",
			Identity: manifest.Identity{ModuleName: "caddy", OwnedBy: "sre@example.com"},
			Services: []manifest.Service{
				{Container: "caddy", Port: 8443, Ingress: true},
			},
		},
		{
			Name:     "auth",
			Identity: manifest.Identity{ModuleName: "auth", OwnedBy: "sre@example.com"},
			Services: []manifest.Service{
				{Container: "dex", Port: 5556, ExposeViaIngress: true},
			},
		},
	}
}

func TestWriteManaged(t *testing.T) {
	tests := []struct {
		name         string
		seed         string // initial .env contents (empty = no file)
		wantContains []string
		wantMissing  []string
	}{
		{
			name: "fresh write happy path",
			seed: "",
			wantContains: []string{
				managedStart,
				"PROJECT_NAME=demo",
				"PROJECT_NAMESPACE=test",
				"EXTERNAL_DOMAIN=demo.example.com",
				"LOCAL_DOMAIN=lvh.me",
				"CADDY_MODULE_NAME=caddy",
				"AUTH_OWNED_BY=sre@example.com",
				"WWW_PORT=3443",
				"WWW_EXPOSE_VIA_INGRESS=true",
				"CADDY_PORT=8443",
				"CADDY_INGRESS=true",
				"DEX_EXPOSE_VIA_INGRESS=true",
				"LOCALMESH_OIDC_CLIENT_SECRET=",
				"OAUTH2_PROXY_COOKIE_SECRET=",
				managedEnd,
			},
		},
		{
			name: "roundtrip preserves preamble",
			seed: "# user preamble\nIMAGE_TAG=feat-x\n",
			wantContains: []string{
				"# user preamble",
				"IMAGE_TAG=feat-x",
				managedStart,
				"PROJECT_NAME=demo",
				managedEnd,
			},
		},
		{
			name:        "no spurious mesh-exempt key",
			seed:        "",
			wantMissing: []string{"MESH_EXEMPT"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			if tt.seed != "" {
				if err := os.WriteFile(filepath.Join(repo, ".env"), []byte(tt.seed), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteManaged(repo, sampleProject(), samplePlugins()); err != nil {
				t.Fatalf("WriteManaged: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(repo, ".env"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(got)
			for _, want := range tt.wantContains {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in output:\n%s", want, text)
				}
			}
			for _, miss := range tt.wantMissing {
				if strings.Contains(text, miss) {
					t.Errorf("unexpected %q in output:\n%s", miss, text)
				}
			}
		})
	}
}

func TestWriteManaged_idempotent(t *testing.T) {
	repo := t.TempDir()
	proj, plugins := sampleProject(), samplePlugins()
	if err := WriteManaged(repo, proj, plugins); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(repo, ".env"))
	if err := WriteManaged(repo, proj, plugins); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(repo, ".env"))
	if string(first) != string(second) {
		t.Fatalf("WriteManaged is not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestWriteManaged_mintOncePreserve(t *testing.T) {
	repo := t.TempDir()
	proj, plugins := sampleProject(), samplePlugins()
	if err := WriteManaged(repo, proj, plugins); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(repo, ".env"))
	oidc1 := extractKey(t, string(first), "LOCALMESH_OIDC_CLIENT_SECRET")
	cookie1 := extractKey(t, string(first), "OAUTH2_PROXY_COOKIE_SECRET")
	if len(oidc1) < 32 || len(cookie1) < 32 {
		t.Fatalf("minted secrets too short: oidc=%d cookie=%d", len(oidc1), len(cookie1))
	}

	if err := WriteManaged(repo, proj, plugins); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(repo, ".env"))
	if got := extractKey(t, string(second), "LOCALMESH_OIDC_CLIENT_SECRET"); got != oidc1 {
		t.Errorf("OIDC secret rotated unexpectedly: %q → %q", oidc1, got)
	}
	if got := extractKey(t, string(second), "OAUTH2_PROXY_COOKIE_SECRET"); got != cookie1 {
		t.Errorf("cookie secret rotated unexpectedly: %q → %q", cookie1, got)
	}
}

func TestWriteManaged_preservesPostamble(t *testing.T) {
	repo := t.TempDir()
	seed := managedStart + "\nSTALE=value\n" + managedEnd + "\n# trailing user note\nAGENTBOX_HOST=x\n"
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManaged(repo, sampleProject(), samplePlugins()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(repo, ".env"))
	text := string(got)
	if !strings.Contains(text, "# trailing user note") {
		t.Errorf("postamble dropped:\n%s", text)
	}
	if !strings.Contains(text, "AGENTBOX_HOST=x") {
		t.Errorf("postamble KV dropped:\n%s", text)
	}
	if strings.Contains(text, "STALE=value") {
		t.Errorf("stale managed key carried over:\n%s", text)
	}
}

func TestWriteManaged_keysSortedAlphabetically(t *testing.T) {
	repo := t.TempDir()
	if err := WriteManaged(repo, sampleProject(), samplePlugins()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(repo, ".env"))
	// Pull the lines between markers and check sort order.
	text := string(got)
	body := text[strings.Index(text, managedStart)+len(managedStart) : strings.Index(text, managedEnd)]
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if k, _, ok := strings.Cut(line, "="); ok {
			keys = append(keys, k)
		}
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("keys out of order at index %d: %q > %q", i, keys[i-1], keys[i])
		}
	}
}

func extractKey(t *testing.T, text, key string) string {
	t.Helper()
	re := regexp.MustCompile("(?m)^" + regexp.QuoteMeta(key) + "=(.*)$")
	m := re.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("key %s not found in:\n%s", key, text)
	}
	return m[1]
}
