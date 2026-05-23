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
	"slices"
	"sort"
	"strings"

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
	Name       string      `toml:"-"` // populated from directory name
	Identity   Identity    `toml:"identity"`
	Plugins    []string    `toml:"plugins"` // meta-package dependencies; nil for leaf plugins
	Services   []Service   `toml:"services"`
	ConfigVars []ConfigVar `toml:"config_vars"` // plugin-author defaults; overridable per instance in deploy.toml
	Exports    []Export    `toml:"exports"`     // env vars the plugin publishes for consumers
}

type Identity struct {
	ModuleName string `toml:"module_name"`
	OwnedBy    string `toml:"owned_by"`
}

type Service struct {
	Container        string `toml:"container"`
	Port             int    `toml:"port"`
	Scheme           string `toml:"scheme"` // grpc | http | https | tcp | postgresql — app's loopback bind protocol
	ExposeViaIngress bool   `toml:"expose_via_ingress"`
	Ingress          bool   `toml:"ingress"`
	RequiresAuth     *bool  `toml:"requires_auth"` // pointer: default true unless explicit false
}

// ConfigVar is a plugin-author-declared knob. The default `value` lives in
// plugin.toml; deploy.toml can override per instance. envwriter emits an
// internal env var named <UPCASE plugin slug>_<UPCASE name> so the plugin's
// compose can interpolate the resolved value.
type ConfigVar struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Value       string `toml:"value"`
}

// Export is a consumer-facing env var the plugin publishes. `template` is
// rendered via text/template against the per-instance context; `env`
// renders to the destination env-var name (defaults to upcased service name).
// deploy.toml may override `env` per instance.
type Export struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Template    string `toml:"template"`
	Env         string `toml:"env"`
	Required    bool   `toml:"required"`
}

// validSchemes is the closed set of supported [[services]].scheme values.
var validSchemes = map[string]bool{"grpc": true, "http": true, "https": true, "tcp": true, "postgresql": true}

// validateServiceSchemes checks every service has a valid scheme. path is
// used in the error message so callers don't need to wrap.
func validateServiceSchemes(path string, services []Service) error {
	for _, s := range services {
		if s.Scheme == "" {
			return fmt.Errorf("%s: scheme required on container %q: %w", path, s.Container, ErrMalformed)
		}
		if !validSchemes[s.Scheme] {
			return fmt.Errorf("%s: container %q has invalid scheme %q (want grpc|http|https|tcp|postgresql): %w", path, s.Container, s.Scheme, ErrMalformed)
		}
	}
	return nil
}

// validateConfigExportNameCollision rejects a plugin that uses the same
// `name` in both [[config_vars]] and [[exports]]. Spec §4 names them in one
// flat per-plugin namespace so consumer-facing templates and internal env
// vars can't fight over the same identifier.
func validateConfigExportNameCollision(path string, cvs []ConfigVar, exps []Export) error {
	names := map[string]bool{}
	for _, c := range cvs {
		names[c.Name] = true
	}
	for _, e := range exps {
		if names[e.Name] {
			return fmt.Errorf("%s: name collision between config_vars and exports: %q: %w", path, e.Name, ErrMalformed)
		}
	}
	return nil
}

// validateExports enforces the per-entry rules from the plugin-config-
// inversion spec §4 (exports): non-empty, single-line name; non-empty
// template + env; unique name across the array. description is intentionally
// not validated (spec asymmetry — exports are consumer-facing wiring, not
// human-facing schema like config_vars).
func validateExports(path string, exps []Export) error {
	seen := map[string]int{}
	for i, e := range exps {
		if e.Name == "" {
			return fmt.Errorf("%s: exports[%d].name required: %w", path, i, ErrMalformed)
		}
		if strings.Contains(e.Name, "\n") {
			return fmt.Errorf("%s: exports[%d].name must be single-line: %w", path, i, ErrMalformed)
		}
		if e.Template == "" {
			return fmt.Errorf("%s: exports[%d].template required: %w", path, i, ErrMalformed)
		}
		if e.Env == "" {
			return fmt.Errorf("%s: exports[%d].env required: %w", path, i, ErrMalformed)
		}
		if j, dup := seen[e.Name]; dup {
			return fmt.Errorf("%s: exports: duplicate name %q (entries %d and %d): %w", path, e.Name, j, i, ErrMalformed)
		}
		seen[e.Name] = i
	}
	return nil
}

// validateConfigVars enforces the per-entry rules from the plugin-config-
// inversion spec §4 (config_vars): non-empty, single-line name + description;
// non-empty value; unique name across the array.
func validateConfigVars(path string, vars []ConfigVar) error {
	seen := map[string]int{}
	for i, v := range vars {
		if v.Name == "" {
			return fmt.Errorf("%s: config_vars[%d].name required: %w", path, i, ErrMalformed)
		}
		if strings.Contains(v.Name, "\n") {
			return fmt.Errorf("%s: config_vars[%d].name must be single-line: %w", path, i, ErrMalformed)
		}
		if v.Description == "" {
			return fmt.Errorf("%s: config_vars[%d].description required: %w", path, i, ErrMalformed)
		}
		if strings.Contains(v.Description, "\n") {
			return fmt.Errorf("%s: config_vars[%d].description must be single-line: %w", path, i, ErrMalformed)
		}
		if v.Value == "" {
			return fmt.Errorf("%s: config_vars[%d].value required: %w", path, i, ErrMalformed)
		}
		if j, dup := seen[v.Name]; dup {
			return fmt.Errorf("%s: config_vars: duplicate name %q (entries %d and %d): %w", path, v.Name, j, i, ErrMalformed)
		}
		seen[v.Name] = i
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
	if err := validateConfigVars(path, p.ConfigVars); err != nil {
		return nil, err
	}
	if err := validateExports(path, p.Exports); err != nil {
		return nil, err
	}
	if err := validateConfigExportNameCollision(path, p.ConfigVars, p.Exports); err != nil {
		return nil, err
	}
	return &p, nil
}

// Deploy is the typed deploy.toml schema. Per-instance orders for selected
// plugins. Phase 1 caps each slug at one instance (enforced by ValidateDeploy);
// Phase 2 lifts that.
type Deploy struct {
	Plugins map[string][]Instance `toml:"plugins"`
}

// Instance is one ordered instantiation of a plugin. Config keys are TOML
// scalars (string / bool / int / float) and get coerced to string at render
// time in envwriter. Exports keys map to override destinations (env-var name
// templates) for the matching [[exports]] entry in plugin.toml.
type Instance struct {
	ServiceName string            `toml:"service_name"`
	Config      map[string]any    `toml:"config"`
	Exports     map[string]string `toml:"exports"`
}

// ValidateDeploy cross-checks a parsed Deploy against the flattened plugin
// list returned by LoadAll. Phase 1 caps each slug at one instance; the
// duplicate-service-name branch is kept as Phase 2 scaffold (structurally
// unreachable today, by design — the cap fires first).
func ValidateDeploy(d *Deploy, plugins []*Plugin) error {
	if d == nil {
		return nil
	}
	flat := map[string]*Plugin{}
	flatNames := make([]string, 0, len(plugins))
	for _, p := range plugins {
		flat[p.Name] = p
		flatNames = append(flatNames, p.Name)
	}
	sort.Strings(flatNames)
	// Deterministic iteration so error ordering is stable across runs.
	slugs := make([]string, 0, len(d.Plugins))
	for s := range d.Plugins {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		instances := d.Plugins[slug]
		p, ok := flat[slug]
		if !ok {
			return fmt.Errorf("deploy.toml: plugins.%s: source not selected in project.toml (flattened: %s): %w",
				slug, strings.Join(flatNames, ", "), ErrMalformed)
		}
		if len(instances) > 1 {
			return fmt.Errorf("deploy.toml: plugins.%s: Phase 1 accepts at most one instance per slug; multi-instance lands in Phase 2: %w", slug, ErrMalformed)
		}
		cvNames := map[string]bool{}
		for _, cv := range p.ConfigVars {
			cvNames[cv.Name] = true
		}
		expNames := map[string]bool{}
		for _, e := range p.Exports {
			expNames[e.Name] = true
		}
		seenSvc := map[string]int{}
		for i, inst := range instances {
			if strings.Contains(inst.ServiceName, "\n") {
				return fmt.Errorf("deploy.toml: plugins.%s[%d].service_name must be single-line: %w", slug, i, ErrMalformed)
			}
			if inst.ServiceName != "" {
				if j, dup := seenSvc[inst.ServiceName]; dup {
					return fmt.Errorf("deploy.toml: plugins.%s: duplicate service_name %q (entries %d and %d): %w",
						slug, inst.ServiceName, j, i, ErrMalformed)
				}
				seenSvc[inst.ServiceName] = i
			}
			for k := range inst.Config {
				if !cvNames[k] {
					return fmt.Errorf("deploy.toml: plugins.%s.config.%s unknown (not in %s/plugin.toml [[config_vars]]): %w",
						slug, k, slug, ErrMalformed)
				}
			}
			for k := range inst.Exports {
				if !expNames[k] {
					return fmt.Errorf("deploy.toml: plugins.%s.exports.%s unknown: %w", slug, k, ErrMalformed)
				}
			}
		}
	}
	return nil
}

// LoadDeploy reads deploy.toml. Absent file is not an error — returns an
// empty Deploy. Validation against the loaded plugin set lives in
// ValidateDeploy, called from LoadAll.
func LoadDeploy(path string) (*Deploy, error) {
	d := &Deploy{Plugins: map[string][]Instance{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return d, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, d); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, ErrMalformed)
	}
	if d.Plugins == nil {
		d.Plugins = map[string][]Instance{}
	}
	return d, nil
}

// CycleError indicates a cycle in the plugin dependency graph. Path is
// rendered as "a -> b -> a" so the operator can find the offending link.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	return "manifest LoadAll: plugin cycle: " + strings.Join(e.Path, " -> ")
}

// LoadAll loads project.toml from projectFile + every plugin.toml reachable
// from project.Plugins (transitively, via each plugin's own plugins = [...]
// field) + deploy.toml from the project root. Returns a depth-first
// post-order, first-occurrence-wins deduped plugin list and the deploy doc
// (non-nil even when deploy.toml is absent). Cycles return *CycleError.
// Unknown plugin names propagate the LoadPlugin error. Deploy is cross-
// validated against the flattened plugin list.
func LoadAll(projectFile, catalogRoot string) (*Project, []*Plugin, *Deploy, error) {
	proj, err := LoadProject(projectFile)
	if err != nil {
		return nil, nil, nil, err
	}
	out := make([]*Plugin, 0, len(proj.Plugins))
	seen := map[string]bool{}
	for _, name := range proj.Plugins {
		out, err = flatten(catalogRoot, name, out, seen, []string{})
		if err != nil {
			return nil, nil, nil, err
		}
	}
	// deploy.toml lives next to project.toml at the project root.
	deployPath := filepath.Join(filepath.Dir(projectFile), "deploy.toml")
	deploy, err := LoadDeploy(deployPath)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := ValidateDeploy(deploy, out); err != nil {
		return nil, nil, nil, err
	}
	return proj, out, deploy, nil
}

// flatten walks the dep graph from `name`, appending plugins in post-order.
// `seen` dedupes; `stack` detects cycles.
func flatten(catalogRoot, name string, out []*Plugin, seen map[string]bool, stack []string) ([]*Plugin, error) {
	if seen[name] {
		return out, nil
	}
	if slices.Contains(stack, name) {
		return nil, &CycleError{Path: append(append([]string{}, stack...), name)}
	}
	p, err := LoadPlugin(catalogRoot, name)
	if err != nil {
		return nil, err
	}
	newStack := append(slices.Clone(stack), name) // own backing array per frame; parent prefix never aliased
	for _, child := range p.Plugins {
		out, err = flatten(catalogRoot, child, out, seen, newStack)
		if err != nil {
			return nil, err
		}
	}
	out = append(out, p)
	seen[name] = true
	return out, nil
}
