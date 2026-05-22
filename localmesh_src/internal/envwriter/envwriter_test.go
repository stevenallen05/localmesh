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
			Name:     "edge",
			Identity: manifest.Identity{ModuleName: "edge", OwnedBy: "sre@example.com"},
			Services: []manifest.Service{
				{Container: "edge", Port: 8443, Ingress: true},
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
				"EDGE_MODULE_NAME=edge",
				"AUTH_OWNED_BY=sre@example.com",
				"WWW_PORT=3443",
				"WWW_EXPOSE_VIA_INGRESS=true",
				"EDGE_PORT=8443",
				"EDGE_INGRESS=true",
				"DEX_EXPOSE_VIA_INGRESS=true",
				"LOCALMESH_OIDC_CLIENT_SECRET=",
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
			if err := WriteManaged(repo, sampleProject(), samplePlugins(), nil); err != nil {
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
	if err := WriteManaged(repo, proj, plugins, nil); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(repo, ".env"))
	if err := WriteManaged(repo, proj, plugins, nil); err != nil {
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
	if err := WriteManaged(repo, proj, plugins, nil); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(repo, ".env"))
	oidc1 := extractKey(t, string(first), "LOCALMESH_OIDC_CLIENT_SECRET")
	if len(oidc1) < 32 {
		t.Fatalf("minted secret too short: oidc=%d", len(oidc1))
	}

	if err := WriteManaged(repo, proj, plugins, nil); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(repo, ".env"))
	if got := extractKey(t, string(second), "LOCALMESH_OIDC_CLIENT_SECRET"); got != oidc1 {
		t.Errorf("OIDC secret rotated unexpectedly: %q → %q", oidc1, got)
	}
}

func TestWriteManaged_preservesPostamble(t *testing.T) {
	repo := t.TempDir()
	seed := managedStart + "\nSTALE=value\n" + managedEnd + "\n# trailing user note\nAGENTBOX_HOST=x\n"
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManaged(repo, sampleProject(), samplePlugins(), nil); err != nil {
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
	if err := WriteManaged(repo, sampleProject(), samplePlugins(), nil); err != nil {
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

// redisProject + redisPlugin: shared fixtures for the resolve+emit tests.
// Project has one app-tier service `www`; plugin `redis` declares one
// service, one config_var (`maxmemory`), one config_var (`noevict`),
// and one export (`connection_url`) with a default templated env name.
func redisProject() *manifest.Project {
	return &manifest.Project{
		Name:        "demo",
		Namespace:   "test",
		LocalDomain: "lvh.me",
		Services: []manifest.Service{
			{Container: "www", Port: 3443, ExposeViaIngress: true},
		},
	}
}

func redisPlugin() *manifest.Plugin {
	return &manifest.Plugin{
		Name:     "redis",
		Identity: manifest.Identity{ModuleName: "redis", OwnedBy: "sre@example.com"},
		Services: []manifest.Service{{Container: "redis", Port: 6379}},
		ConfigVars: []manifest.ConfigVar{
			{Name: "maxmemory", Description: "Memory cap", Value: "256mb"},
			{Name: "noevict", Description: "Noevict policy", Value: "false"},
		},
		Exports: []manifest.Export{
			{
				Name:     "connection_url",
				Template: "redis://{{ .Service.Container }}:{{ .Service.Port }}",
				Env:      "{{ .ServiceName | upper }}_URL",
				Required: true,
			},
		},
	}
}

// readEnv is a small helper for the resolve+emit tests.
func readEnv(t *testing.T, repo string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteManaged_exportEmitsDefaultEnvName(t *testing.T) {
	repo := t.TempDir()
	if err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, &manifest.Deploy{}); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, repo)
	want := "REDIS_URL=redis://redis:6379"
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}

func TestWriteManaged_deployOverridesEnvName(t *testing.T) {
	repo := t.TempDir()
	deploy := &manifest.Deploy{Plugins: map[string][]manifest.Instance{
		"redis": {{
			ServiceName: "cache",
			Exports:     map[string]string{"connection_url": "CACHE_URL"},
		}},
	}}
	if err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, deploy); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, repo)
	if !strings.Contains(got, "CACHE_URL=redis://redis:6379") {
		t.Errorf("missing CACHE_URL line in:\n%s", got)
	}
	if strings.Contains(got, "REDIS_URL=") {
		t.Errorf("default REDIS_URL should not be emitted when override is set:\n%s", got)
	}
}

func TestWriteManaged_overrideEnvNameIsRendered(t *testing.T) {
	repo := t.TempDir()
	deploy := &manifest.Deploy{Plugins: map[string][]manifest.Instance{
		"redis": {{
			Exports: map[string]string{"connection_url": "{{ .Project.Name | upper }}_URL"},
		}},
	}}
	if err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, deploy); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, repo)
	if !strings.Contains(got, "DEMO_URL=redis://redis:6379") {
		t.Errorf("missing DEMO_URL in:\n%s", got)
	}
}

func TestWriteManaged_deployOverridesConfigValue(t *testing.T) {
	repo := t.TempDir()
	deploy := &manifest.Deploy{Plugins: map[string][]manifest.Instance{
		"redis": {{
			Config: map[string]any{"maxmemory": "512mb"},
		}},
	}}
	if err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, deploy); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, repo)
	if !strings.Contains(got, "REDIS_MAXMEMORY=512mb") {
		t.Errorf("missing REDIS_MAXMEMORY=512mb in:\n%s", got)
	}
	if strings.Contains(got, "REDIS_MAXMEMORY=256mb") {
		t.Errorf("default 256mb should not appear when override is set:\n%s", got)
	}
}

func TestWriteManaged_configValueCoercion(t *testing.T) {
	repo := t.TempDir()
	deploy := &manifest.Deploy{Plugins: map[string][]manifest.Instance{
		"redis": {{
			Config: map[string]any{"noevict": true},
		}},
	}}
	if err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, deploy); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, repo)
	if !strings.Contains(got, "REDIS_NOEVICT=true") {
		t.Errorf("missing REDIS_NOEVICT=true in:\n%s", got)
	}
}

func TestWriteManaged_absentDeployUsesDefaults(t *testing.T) {
	repo := t.TempDir()
	if err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, nil); err != nil {
		t.Fatal(err)
	}
	got := readEnv(t, repo)
	for _, want := range []string{"REDIS_URL=redis://redis:6379", "REDIS_MAXMEMORY=256mb", "REDIS_NOEVICT=false"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestWriteManaged_requiredExportMissing(t *testing.T) {
	repo := t.TempDir()
	deploy := &manifest.Deploy{Plugins: map[string][]manifest.Instance{
		"redis": {{Exports: map[string]string{"connection_url": ""}}},
	}}
	err := WriteManaged(repo, redisProject(), []*manifest.Plugin{redisPlugin()}, deploy)
	if err == nil {
		t.Fatal("expected error for required export rendering empty destination")
	}
	if !strings.Contains(err.Error(), "required but rendered empty destination") {
		t.Errorf("err = %v, want substring 'required but rendered empty destination'", err)
	}
}

func TestWriteManaged_exportTemplateError(t *testing.T) {
	repo := t.TempDir()
	p := redisPlugin()
	p.Exports[0].Template = "{{ .Bogus }}" // missing key
	err := WriteManaged(repo, redisProject(), []*manifest.Plugin{p}, &manifest.Deploy{})
	if err == nil {
		t.Fatal("expected template execute error")
	}
	if !strings.Contains(err.Error(), "execute") {
		t.Errorf("err = %v, want substring 'execute'", err)
	}
}

func TestWriteManaged_exportCollision(t *testing.T) {
	repo := t.TempDir()
	// Two plugins both emit REDIS_URL (literal env override on both → same name).
	p1 := redisPlugin()
	p2 := &manifest.Plugin{
		Name:     "redis2",
		Identity: manifest.Identity{ModuleName: "redis2", OwnedBy: "sre@example.com"},
		Services: []manifest.Service{{Container: "redis2", Port: 6380}},
		Exports: []manifest.Export{{
			Name:     "connection_url",
			Template: "redis://{{ .Service.Container }}:{{ .Service.Port }}",
			Env:      "REDIS_URL",
			Required: true,
		}},
	}
	deploy := &manifest.Deploy{Plugins: map[string][]manifest.Instance{
		"redis": {{Exports: map[string]string{"connection_url": "REDIS_URL"}}},
	}}
	err := WriteManaged(repo, redisProject(), []*manifest.Plugin{p1, p2}, deploy)
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("err = %v, want substring 'collision'", err)
	}
}

func TestCoerceString(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string", "hi", "hi"},
		{"boolTrue", true, "true"},
		{"boolFalse", false, "false"},
		{"int", 42, "42"},
		{"int64", int64(7), "7"},
		{"float64", 3.14, "3.14"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coerceString(tc.in); got != tc.want {
				t.Errorf("coerceString(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRenderExportEnvName_default(t *testing.T) {
	ctx := exportCtx{ServiceName: "cache"}
	got, err := renderExportEnvName("{{ .ServiceName | upper }}_URL", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "CACHE_URL" {
		t.Errorf("got %q, want CACHE_URL", got)
	}
}

func TestRenderExportEnvName_overrideLiteral(t *testing.T) {
	ctx := exportCtx{ServiceName: "cache"}
	got, err := renderExportEnvName("CACHE_URL", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "CACHE_URL" {
		t.Errorf("got %q, want CACHE_URL", got)
	}
}

func TestRenderExportEnvName_overrideTemplated(t *testing.T) {
	ctx := exportCtx{ServiceName: "cache", Project: projectLite{Name: "demo"}}
	got, err := renderExportEnvName("{{ .Project.Name | upper }}_URL", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "DEMO_URL" {
		t.Errorf("got %q, want DEMO_URL", got)
	}
}

func TestRenderExportValue_simple(t *testing.T) {
	ctx := exportCtx{Service: serviceLite{Container: "redis", Port: 6379}}
	got, err := renderExportValue("redis://{{ .Service.Container }}:{{ .Service.Port }}", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "redis://redis:6379" {
		t.Errorf("got %q, want redis://redis:6379", got)
	}
}

func TestRenderExportValue_templateError(t *testing.T) {
	_, err := renderExportValue("{{ .Bogus }}", exportCtx{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadEnv_parsesKVsAcrossManagedAndUnmanagedSections(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	body := strings.Join([]string{
		"# hand-edited line",
		"PROJECT_NAME=metrics-collector",
		"",
		"# >>> secrets-gen managed — do not edit; regenerated on `make certs`",
		"LOCALMESH_OIDC_CLIENT_SECRET=cafebabe",
		"LOCAL_DOMAIN=lvh.me",
		"# <<< secrets-gen managed",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadEnv(envPath)
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	want := map[string]string{
		"PROJECT_NAME":                 "metrics-collector",
		"LOCALMESH_OIDC_CLIENT_SECRET": "cafebabe",
		"LOCAL_DOMAIN":                 "lvh.me",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("LoadEnv()[%q] = %q, want %q", k, got[k], v)
		}
	}
}
