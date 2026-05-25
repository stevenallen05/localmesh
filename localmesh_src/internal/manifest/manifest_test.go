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

func TestLoadPlugin_configVarsParse(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "redis")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// [[services]] intentionally omitted — validateServiceSchemes is a no-op
	// on an empty slice, and this test is scoped to config_vars parsing.
	body := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[config_vars]]
name        = "maxmemory"
description = "Memory cap"
value       = "256mb"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlugin(dir, "redis")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(p.ConfigVars), 1; got != want {
		t.Fatalf("len(ConfigVars) = %d, want %d", got, want)
	}
	if got, want := p.ConfigVars[0].Name, "maxmemory"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := p.ConfigVars[0].Description, "Memory cap"; got != want {
		t.Errorf("Description = %q, want %q", got, want)
	}
	if got, want := p.ConfigVars[0].Value, "256mb"; got != want {
		t.Errorf("Value = %q, want %q", got, want)
	}
}

func TestLoadPlugin_exportsParse(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "redis")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[exports]]
name        = "connection_url"
description = "Redis connection URL"
template    = "redis://host:6379"
env         = "REDIS_URL"
required    = true
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlugin(dir, "redis")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(p.Exports), 1; got != want {
		t.Fatalf("len(Exports) = %d, want %d", got, want)
	}
	if got, want := p.Exports[0].Name, "connection_url"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := p.Exports[0].Template, "redis://host:6379"; got != want {
		t.Errorf("Template = %q, want %q", got, want)
	}
	if got, want := p.Exports[0].Env, "REDIS_URL"; got != want {
		t.Errorf("Env = %q, want %q", got, want)
	}
	if !p.Exports[0].Required {
		t.Errorf("Required = false, want true")
	}
}

func TestLoadPlugin_configVarsValidation(t *testing.T) {
	identity := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"
`
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"missingName", `[[config_vars]]
description = "x"
value       = "y"
`, `config_vars[0].name required`},
		{"multilineName", "[[config_vars]]\nname        = \"a\\nb\"\ndescription = \"x\"\nvalue       = \"y\"\n", `config_vars[0].name must be single-line`},
		{"missingDescription", `[[config_vars]]
name  = "x"
value = "y"
`, `config_vars[0].description required`},
		{"multilineDescription", "[[config_vars]]\nname        = \"x\"\ndescription = \"a\\nb\"\nvalue       = \"y\"\n", `config_vars[0].description must be single-line`},
		{"missingValue", `[[config_vars]]
name        = "x"
description = "y"
`, `config_vars[0].value required`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pluginDir := filepath.Join(dir, "redis")
			if err := os.MkdirAll(pluginDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(identity+tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadPlugin(dir, "redis")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadPlugin_configVarDuplicateName(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "redis")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[config_vars]]
name        = "maxmemory"
description = "first"
value       = "256mb"

[[config_vars]]
name        = "maxmemory"
description = "second"
value       = "512mb"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPlugin(dir, "redis")
	if err == nil {
		t.Fatal("expected duplicate-name error, got nil")
	}
	if !strings.Contains(err.Error(), `duplicate name "maxmemory"`) {
		t.Errorf("err = %v, want substring %q", err, `duplicate name "maxmemory"`)
	}
}

func TestLoadPlugin_exportsValidation(t *testing.T) {
	identity := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"
`
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"missingName", `[[exports]]
template = "x"
env      = "Y"
`, `exports[0].name required`},
		{"multilineName", "[[exports]]\nname     = \"a\\nb\"\ntemplate = \"x\"\nenv      = \"Y\"\n", `exports[0].name must be single-line`},
		{"missingTemplate", `[[exports]]
name = "x"
env  = "Y"
`, `exports[0].template required`},
		{"missingEnv", `[[exports]]
name     = "x"
template = "y"
`, `exports[0].env required`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pluginDir := filepath.Join(dir, "redis")
			if err := os.MkdirAll(pluginDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(identity+tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadPlugin(dir, "redis")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadPlugin_exportDuplicateName(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "redis")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[exports]]
name     = "connection_url"
template = "redis://host:6379"
env      = "REDIS_URL"

[[exports]]
name     = "connection_url"
template = "redis://other:6380"
env      = "REDIS_URL_2"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPlugin(dir, "redis")
	if err == nil {
		t.Fatal("expected duplicate-name error, got nil")
	}
	if !strings.Contains(err.Error(), `duplicate name "connection_url"`) {
		t.Errorf("err = %v, want substring %q", err, `duplicate name "connection_url"`)
	}
}

func TestLoadPlugin_nameCollisionAcrossArrays(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "redis")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[config_vars]]
name        = "connection_url"
description = "knob with the same name as an export"
value       = "ignored"

[[exports]]
name     = "connection_url"
template = "redis://host:6379"
env      = "REDIS_URL"
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPlugin(dir, "redis")
	if err == nil {
		t.Fatal("expected name-collision error, got nil")
	}
	if !strings.Contains(err.Error(), `name collision between config_vars and exports: "connection_url"`) {
		t.Errorf("err = %v, want substring %q", err, `name collision between config_vars and exports: "connection_url"`)
	}
}

func TestLoadDeploy_parses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy.toml")
	body := `
[plugins]
[[plugins.redis]]
service_name = "cache"
  [plugins.redis.config]
  maxmemory = "512mb"
  [plugins.redis.exports]
  connection_url = "CACHE_URL"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDeploy(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(d.Plugins["redis"]), 1; got != want {
		t.Fatalf("len(Plugins[redis]) = %d, want %d", got, want)
	}
	inst := d.Plugins["redis"][0]
	if got, want := inst.ServiceName, "cache"; got != want {
		t.Errorf("ServiceName = %q, want %q", got, want)
	}
	if got, want := inst.Config["maxmemory"], "512mb"; got != want {
		t.Errorf(`Config["maxmemory"] = %v, want %v`, got, want)
	}
	if got, want := inst.Exports["connection_url"], "CACHE_URL"; got != want {
		t.Errorf(`Exports["connection_url"] = %q, want %q`, got, want)
	}
}

func TestLoadDeploy_parsesMultipleInstancesPerSlug(t *testing.T) {
	// LoadDeploy is purely structural — the Phase 1 cap belongs in
	// ValidateDeploy. BurntSushi/toml populates the slice in declaration order.
	path := filepath.Join(t.TempDir(), "deploy.toml")
	body := `
[plugins]
[[plugins.redis]]
service_name = "cache"

[[plugins.redis]]
service_name = "analytics"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDeploy(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(d.Plugins["redis"]), 2; got != want {
		t.Fatalf("len(Plugins[redis]) = %d, want %d", got, want)
	}
	if got, want := d.Plugins["redis"][0].ServiceName, "cache"; got != want {
		t.Errorf("Plugins[redis][0].ServiceName = %q, want %q", got, want)
	}
	if got, want := d.Plugins["redis"][1].ServiceName, "analytics"; got != want {
		t.Errorf("Plugins[redis][1].ServiceName = %q, want %q", got, want)
	}
}

func TestLoadDeploy_absent(t *testing.T) {
	d, err := LoadDeploy(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("expected non-nil Deploy")
	}
	if len(d.Plugins) != 0 {
		t.Errorf("expected empty Plugins, got %v", d.Plugins)
	}
}

func TestLoadDeploy_configKeepsTomlScalar(t *testing.T) {
	// Coercion to string happens in envwriter; LoadDeploy stores the raw
	// TOML scalar (here: a bool) in Config[any].
	path := filepath.Join(t.TempDir(), "deploy.toml")
	body := `
[plugins]
[[plugins.redis]]
  [plugins.redis.config]
  noevict = true
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := LoadDeploy(path)
	if err != nil {
		t.Fatal(err)
	}
	inst := d.Plugins["redis"][0]
	if got, want := inst.Config["noevict"], true; got != want {
		t.Errorf(`Config["noevict"] = %v (%T), want %v (bool)`, got, got, want)
	}
}

func TestValidateDeploy(t *testing.T) {
	cases := []struct {
		name    string
		deploy  string
		plugins []*Plugin
		wantErr string
	}{
		{
			name: "sourceNotSelected",
			deploy: `[plugins]
[[plugins.foo]]
`,
			plugins: []*Plugin{{Name: "redis"}},
			wantErr: `plugins.foo: source not selected in project.toml`,
		},
		{
			name: "phase1RejectsSecondInstance",
			deploy: `[plugins]
[[plugins.redis]]
service_name = "a"

[[plugins.redis]]
service_name = "b"
`,
			plugins: []*Plugin{{Name: "redis"}},
			wantErr: `Phase 1 accepts at most one instance per slug`,
		},
		{
			// Duplicate-service-name detection is Phase 2 scaffold;
			// the multi-instance cap fires first in Phase 1. Asserting
			// the cap error pins precedence so the validator's shape
			// doesn't drift across phases.
			name: "duplicateServiceNameWithinSlug",
			deploy: `[plugins]
[[plugins.redis]]
service_name = "x"

[[plugins.redis]]
service_name = "x"
`,
			plugins: []*Plugin{{Name: "redis"}},
			wantErr: `Phase 1 accepts at most one instance per slug`,
		},
		{
			name: "unknownConfigKey",
			deploy: `[plugins]
[[plugins.redis]]
[plugins.redis.config]
nope = "x"
`,
			plugins: []*Plugin{{Name: "redis", ConfigVars: []ConfigVar{{Name: "maxmemory"}}}},
			wantErr: `plugins.redis.config.nope unknown`,
		},
		{
			name: "unknownExportKey",
			deploy: `[plugins]
[[plugins.redis]]
[plugins.redis.exports]
nope = "X"
`,
			plugins: []*Plugin{{Name: "redis", Exports: []Export{{Name: "connection_url"}}}},
			wantErr: `plugins.redis.exports.nope unknown`,
		},
		{
			name: "happyPath",
			deploy: `[plugins]
[[plugins.redis]]
service_name = "cache"
[plugins.redis.config]
maxmemory = "512mb"
[plugins.redis.exports]
connection_url = "CACHE_URL"
`,
			plugins: []*Plugin{{
				Name:       "redis",
				ConfigVars: []ConfigVar{{Name: "maxmemory"}},
				Exports:    []Export{{Name: "connection_url"}},
			}},
			wantErr: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "deploy.toml")
			if err := os.WriteFile(path, []byte(tc.deploy), 0o600); err != nil {
				t.Fatal(err)
			}
			d, err := LoadDeploy(path)
			if err != nil {
				t.Fatalf("LoadDeploy: %v", err)
			}
			err = ValidateDeploy(d, tc.plugins)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoadAll_sampleCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "sample_catalog")
	proj, plugins, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
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

	_, plugins, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
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

	_, _, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
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

	_, _, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
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

	_, _, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
	var cyc *CycleError
	if !errors.As(err, &cyc) {
		t.Fatalf("got %v, want *CycleError", err)
	}
	if !strings.Contains(err.Error(), "a -> a") {
		t.Errorf("error message %q missing self-cycle path", err.Error())
	}
}

func TestLoadAll_returnsDeploy(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "redis", nil)
	writeProject(t, root, []string{"redis"})
	deploy := `[plugins]
[[plugins.redis]]
service_name = "cache"
`
	if err := os.WriteFile(filepath.Join(root, "deploy.toml"), []byte(deploy), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, d, err := LoadAll(filepath.Join(root, "project.toml"), root)
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("expected non-nil Deploy")
	}
	if got, want := len(d.Plugins["redis"]), 1; got != want {
		t.Fatalf("len(Plugins[redis]) = %d, want %d", got, want)
	}
	if got, want := d.Plugins["redis"][0].ServiceName, "cache"; got != want {
		t.Errorf("ServiceName = %q, want %q", got, want)
	}
}

func TestLoadAll_absentDeployReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "redis", nil)
	writeProject(t, root, []string{"redis"})
	// no deploy.toml written
	_, _, d, err := LoadAll(filepath.Join(root, "project.toml"), root)
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("expected non-nil Deploy")
	}
	if len(d.Plugins) != 0 {
		t.Errorf("expected empty Plugins, got %v", d.Plugins)
	}
}

func TestLoadAll_invalidDeployErrors(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "redis", nil)
	writeProject(t, root, []string{"redis"})
	deploy := `[plugins]
[[plugins.foo]]
service_name = "x"
`
	if err := os.WriteFile(filepath.Join(root, "deploy.toml"), []byte(deploy), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := LoadAll(filepath.Join(root, "project.toml"), root)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "source not selected") {
		t.Errorf("err = %v, want substring 'source not selected'", err)
	}
}


// TestLoadPlugin_RejectsDeprecatedNeedsMtlsSidecar guards against silent
// reintroduction of the dropped per-service `needs_mtls_sidecar` flag.
// strict-decode on LoadPlugin should surface it as an unknown field.
func TestLoadPlugin_RejectsDeprecatedNeedsMtlsSidecar(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "stale")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := []byte(`
[identity]
module_name = "stale"
owned_by    = "test@example.com"

[[services]]
container          = "x"
port               = 1234
scheme             = "http"
needs_mtls_sidecar = false
`)
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), body, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := LoadPlugin(root, "stale")
	if err == nil {
		t.Fatal("expected LoadPlugin to reject needs_mtls_sidecar, got nil")
	}
	if !strings.Contains(err.Error(), "needs_mtls_sidecar") && !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("err = %v, want substring 'needs_mtls_sidecar' or 'unknown'", err)
	}
}
