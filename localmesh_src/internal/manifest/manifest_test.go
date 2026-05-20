package manifest

import (
	"errors"
	"os"
	"path/filepath"
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

func TestLoadAll_sampleCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "sample_catalog")
	proj, plugins, err := LoadAll(root)
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
