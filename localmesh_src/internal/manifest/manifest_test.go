package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadProject(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{"happy path", `project_name="x"
local_domain="lvh.me"`, nil},
		{"missing name", `local_domain="lvh.me"`, ErrMalformed},
		{"missing local_domain", `project_name="x"`, ErrMalformed},
		{"malformed toml", `not toml`, ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := filepath.Join(t.TempDir(), "project.toml")
			if err := os.WriteFile(tmp, []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadProject(tmp)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadPlugin_SchemeRequired(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "x")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[identity]
module_name = "x"
owned_by    = "x@example.com"

[[services]]
container = "x"
port      = 1234
# scheme intentionally missing
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPlugin(dir, "x")
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("expected ErrMalformed for missing scheme; got %v", err)
	}
}

func TestLoadPlugin_SchemeValid(t *testing.T) {
	valid := []string{"grpc", "http", "https", "tcp", "postgresql"}
	for _, s := range valid {
		t.Run(s, func(t *testing.T) {
			dir := t.TempDir()
			pluginDir := filepath.Join(dir, "x")
			if err := os.MkdirAll(pluginDir, 0o755); err != nil {
				t.Fatal(err)
			}
			body := fmt.Sprintf(`
[identity]
module_name = "x"
owned_by    = "x@example.com"

[[services]]
container = "x"
port      = 1234
scheme    = %q
`, s)
			if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			p, err := LoadPlugin(dir, "x")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Services[0].Scheme != s {
				t.Errorf("scheme = %q, want %q", p.Services[0].Scheme, s)
			}
		})
	}
}

func TestLoadPlugin_SchemeInvalid(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "x")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[identity]
module_name = "x"
owned_by    = "x@example.com"

[[services]]
container = "x"
port      = 1234
scheme    = "ftp"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPlugin(dir, "x")
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("expected ErrMalformed for invalid scheme; got %v", err)
	}
}

func TestLoadAll_sampleCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "sample_catalog")
	proj, plugins, err := LoadAll(filepath.Join(root, "project.toml"), root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if proj.Name != "sample" {
		t.Errorf("Name = %q, want sample", proj.Name)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}
	if plugins[0].Name != "alpha" || plugins[1].Name != "beta" {
		t.Errorf("plugin names = %v, want [alpha beta]", []string{plugins[0].Name, plugins[1].Name})
	}
}

// writePlugin writes a minimal plugin.toml in <root>/<name>/plugin.toml.
// plugins is the value of the new `plugins = [...]` array; nil omits the key.
func writePlugin(t *testing.T, root, name string, plugins []string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	if plugins != nil {
		quoted := make([]string, len(plugins))
		for i, p := range plugins {
			quoted[i] = fmt.Sprintf("%q", p)
		}
		body += fmt.Sprintf("plugins = [%s]\n", strings.Join(quoted, ", "))
	}
	body += fmt.Sprintf("[identity]\nmodule_name = %q\nowned_by    = %q\n", name, name+"@example.com")
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeProject writes a minimal project.toml at <root>/project.toml selecting
// the given plugin names.
func writeProject(t *testing.T, root string, plugins []string) {
	t.Helper()
	quoted := make([]string, len(plugins))
	for i, p := range plugins {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	body := fmt.Sprintf("project_name = \"sample\"\nlocal_domain = \"lvh.me\"\nplugins = [%s]\n", strings.Join(quoted, ", "))
	if err := os.WriteFile(filepath.Join(root, "project.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAll_transitiveFlattenDedup(t *testing.T) {
	root := t.TempDir()
	// base requires [auth, security]; security requires [auth]; postgres16 is a leaf.
	// Project selects [base, postgres16]. Auth must appear once, depth-first post-order.
	writePlugin(t, root, "auth", nil)
	writePlugin(t, root, "security", []string{"auth"})
	writePlugin(t, root, "base", []string{"auth", "security"})
	writePlugin(t, root, "postgres16", nil)
	writeProject(t, root, []string{"base", "postgres16"})

	_, plugins, err := LoadAll(filepath.Join(root, "project.toml"), root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	got := make([]string, len(plugins))
	for i, p := range plugins {
		got[i] = p.Name
	}
	want := []string{"auth", "security", "base", "postgres16"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("flatten order = %v, want %v", got, want)
	}
}

func TestLoadAll_cycle(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "a", []string{"b"})
	writePlugin(t, root, "b", []string{"a"})
	writeProject(t, root, []string{"a"})

	_, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
	var cyc *CycleError
	if !errors.As(err, &cyc) {
		t.Fatalf("got %v, want *CycleError", err)
	}
	want := []string{"a", "b", "a"}
	if !reflect.DeepEqual(cyc.Path, want) {
		t.Errorf("cycle path = %v, want %v", cyc.Path, want)
	}
	if !strings.Contains(err.Error(), "a -> b -> a") {
		t.Errorf("error message %q missing path", err.Error())
	}
}

func TestLoadAll_unknownMember(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "base", []string{"nonexistent"})
	writeProject(t, root, []string{"base"})

	_, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
	if err == nil {
		t.Fatal("expected error for unknown plugin, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q should mention the missing plugin name", err.Error())
	}
}

func TestLoadAll_selfCycle(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "a", []string{"a"})
	writeProject(t, root, []string{"a"})

	_, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
	var cyc *CycleError
	if !errors.As(err, &cyc) {
		t.Fatalf("got %v, want *CycleError", err)
	}
	if !strings.Contains(err.Error(), "a -> a") {
		t.Errorf("error message %q missing self-cycle path", err.Error())
	}
}

func TestService_Meshed(t *testing.T) {
	tr, fa := true, false
	tests := []struct {
		name string
		in   *bool
		want bool
	}{
		{"absent defaults meshed", nil, true},
		{"explicit true", &tr, true},
		{"explicit false exempt", &fa, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Service{NeedsMTLSSidecar: tt.in}).Meshed(); got != tt.want {
				t.Errorf("Meshed() = %v, want %v", got, tt.want)
			}
		})
	}
}
