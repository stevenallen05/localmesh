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
	Name           string    `toml:"project_name"`
	Namespace      string    `toml:"project_namespace"`
	TechLead       string    `toml:"tech_lead_email"`
	ExternalDomain string    `toml:"external_domain"`
	LocalDomain    string    `toml:"local_domain"`
	Plugins        []string  `toml:"plugins"`
	Services       []Service `toml:"services"` // app-tier services minted alongside plugins
	// (compliance + legal + billing + vendors omitted —
	// templates don't need them today; add when a use surfaces.)
}

// Plugin is the typed plugin.toml schema for templates.
type Plugin struct {
	Name     string    `toml:"-"` // populated from directory name
	Identity Identity  `toml:"identity"`
	Plugins  []string  `toml:"plugins"` // meta-package dependencies; nil for leaf plugins
	Services []Service `toml:"services"`
}

type Identity struct {
	ModuleName string `toml:"module_name"`
	OwnedBy    string `toml:"owned_by"`
}

type Service struct {
	Container        string `toml:"container"`
	Port             int    `toml:"port"`
	Scheme           string `toml:"scheme"` // grpc | http | https | tcp — app's loopback bind protocol
	ExposeViaIngress bool   `toml:"expose_via_ingress"`
	Ingress          bool   `toml:"ingress"`
	RequiresAuth     *bool  `toml:"requires_auth"` // pointer: default true unless explicit false
}

// validSchemes is the closed set of supported [[services]].scheme values.
var validSchemes = map[string]bool{"grpc": true, "http": true, "https": true, "tcp": true}

// validateServiceSchemes checks every service has a valid scheme. path is
// used in the error message so callers don't need to wrap.
func validateServiceSchemes(path string, services []Service) error {
	for _, s := range services {
		if s.Scheme == "" {
			return fmt.Errorf("%s: scheme required on container %q: %w", path, s.Container, ErrMalformed)
		}
		if !validSchemes[s.Scheme] {
			return fmt.Errorf("%s: container %q has invalid scheme %q (want grpc|http|https|tcp): %w", path, s.Container, s.Scheme, ErrMalformed)
		}
	}
	return nil
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
	if err := validateServiceSchemes(path, p.Services); err != nil {
		return nil, err
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
	if err := validateServiceSchemes(path, p.Services); err != nil {
		return nil, err
	}
	return &p, nil
}

// LoadAll loads project.toml from projectFile + every plugin.toml listed
// in project.Plugins, resolved relative to catalogRoot. In the real repo
// project.toml lives at the repo root and plugins under service_catalog/;
// in test fixtures both live in the same directory.
func LoadAll(projectFile, catalogRoot string) (*Project, []*Plugin, error) {
	proj, err := LoadProject(projectFile)
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
