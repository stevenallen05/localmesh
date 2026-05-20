// Package manifest loads and validates project.toml + plugin.toml files.
//
// See docs/engineering/rules/golang-basics.md §2 for error-handling style
// and the localmesh CLI design spec §2.4 for the schema shape.
package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ErrMalformed indicates a TOML file that parses but fails schema validation.
var ErrMalformed = errors.New("manifest malformed")

// Project is the typed project.toml schema for templates.
type Project struct {
	Name        string   `toml:"project_name"`
	Namespace   string   `toml:"project_namespace"`
	TechLead    string   `toml:"tech_lead_email"`
	LocalDomain string   `toml:"local_domain"`
	Plugins     []string `toml:"plugins"`
	// (services + compliance + legal + billing + vendors omitted —
	// templates don't need them today; add when a use surfaces.)
}

// Plugin is the typed plugin.toml schema for templates.
type Plugin struct {
	Name     string    `toml:"-"` // populated from directory name
	Identity Identity  `toml:"identity"`
	Services []Service `toml:"services"`
}

type Identity struct {
	ModuleName string `toml:"module_name"`
	OwnedBy    string `toml:"owned_by"`
}

type Service struct {
	Container        string `toml:"container"`
	Port             int    `toml:"port"`
	ExposeViaIngress bool   `toml:"expose_via_ingress"`
	Ingress          bool   `toml:"ingress"`
	RequiresAuth     *bool  `toml:"requires_auth"` // pointer: default true unless explicit false
}

// LoadProject reads and validates project.toml.
func LoadProject(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var p Project
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, ErrMalformed)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("%s: project_name required: %w", path, ErrMalformed)
	}
	if p.LocalDomain == "" {
		return nil, fmt.Errorf("%s: local_domain required: %w", path, ErrMalformed)
	}
	return &p, nil
}

// LoadPlugin reads and validates plugin.toml at <catalogRoot>/<name>/plugin.toml.
func LoadPlugin(catalogRoot, name string) (*Plugin, error) {
	path := filepath.Join(catalogRoot, name, "plugin.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var p Plugin
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, ErrMalformed)
	}
	p.Name = name
	if p.Identity.ModuleName == "" {
		return nil, fmt.Errorf("%s: identity.module_name required: %w", path, ErrMalformed)
	}
	if p.Identity.OwnedBy == "" {
		return nil, fmt.Errorf("%s: identity.owned_by required: %w", path, ErrMalformed)
	}
	return &p, nil
}

// LoadAll loads project.toml + every plugin.toml listed in project.Plugins.
func LoadAll(catalogRoot string) (*Project, []*Plugin, error) {
	proj, err := LoadProject(filepath.Join(catalogRoot, "project.toml"))
	if err != nil {
		return nil, nil, err
	}
	plugins := make([]*Plugin, 0, len(proj.Plugins))
	for _, name := range proj.Plugins {
		p, err := LoadPlugin(catalogRoot, name)
		if err != nil {
			return nil, nil, err
		}
		plugins = append(plugins, p)
	}
	return proj, plugins, nil
}
