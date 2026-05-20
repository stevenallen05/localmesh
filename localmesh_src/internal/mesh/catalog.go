package mesh

import (
	"fmt"
	"sort"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

// Registry is a name-keyed view of every service across project + plugins.
// Use Get to resolve a container name to its declared Service.
type Registry struct {
	byContainer map[string]manifest.Service
	owner       map[string]string // container → owning plugin name (for diagnostics)
}

// BuildRegistry returns a unified registry of every [[services]] entry.
// Panics on duplicate container names — the catalog is canonical and a
// duplicate is a build-time invariant violation.
func BuildRegistry(proj *manifest.Project, plugins []*manifest.Plugin) *Registry {
	r := &Registry{
		byContainer: map[string]manifest.Service{},
		owner:       map[string]string{},
	}
	add := func(svc manifest.Service, ownerName string) {
		if _, exists := r.byContainer[svc.Container]; exists {
			panic(fmt.Sprintf("registry: duplicate container %q (owners: %s + %s)",
				svc.Container, r.owner[svc.Container], ownerName))
		}
		r.byContainer[svc.Container] = svc
		r.owner[svc.Container] = ownerName
	}
	for _, svc := range proj.Services {
		add(svc, "project")
	}
	for _, p := range plugins {
		for _, svc := range p.Services {
			add(svc, p.Name)
		}
	}
	return r
}

// Get returns the Service declared for container, or false if unknown.
func (r *Registry) Get(container string) (manifest.Service, bool) {
	svc, ok := r.byContainer[container]
	return svc, ok
}

// AllContainers lists every container name in registry order (alphabetical).
func (r *Registry) AllContainers() []string {
	out := make([]string, 0, len(r.byContainer))
	for k := range r.byContainer {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
