package mesh

import (
	"errors"
	"fmt"
	"sort"
)

// ErrPortCollision indicates two registry services share a port,
// which makes iptables port-based redirection ambiguous from any caller
// that depends on both.
var ErrPortCollision = errors.New("port collision between services")

// ErrUnknownDepends indicates a service's depends list names a container
// that doesn't exist in the registry.
var ErrUnknownDepends = errors.New("unknown depends target")

// Edge represents one outbound mTLS edge from a workload to a peer.
type Edge struct {
	Target     string // peer container name
	TargetPort int    // peer's [[services]] port
}

// DeriveEdges walks every registry service's Depends list, resolves each
// target to a registry entry, and returns the per-source outbound edge
// list. Targets that don't resolve are an error (typos in the manifest
// surface here rather than silently dropping).
func DeriveEdges(reg *Registry) (map[string][]Edge, error) {
	if err := validatePortCollisions(reg); err != nil {
		return nil, err
	}
	out := map[string][]Edge{}
	for _, src := range reg.AllContainers() {
		svc, _ := reg.Get(src)
		if len(svc.Depends) == 0 {
			continue
		}
		edges := make([]Edge, 0, len(svc.Depends))
		for _, dep := range svc.Depends {
			target, ok := reg.Get(dep)
			if !ok {
				return nil, fmt.Errorf("%w: %s depends on %q", ErrUnknownDepends, src, dep)
			}
			edges = append(edges, Edge{Target: dep, TargetPort: target.Port})
		}
		sort.Slice(edges, func(i, j int) bool { return edges[i].Target < edges[j].Target })
		out[src] = edges
	}
	return out, nil
}

func validatePortCollisions(reg *Registry) error {
	byPort := map[int][]string{}
	for _, container := range reg.AllContainers() {
		svc, _ := reg.Get(container)
		byPort[svc.Port] = append(byPort[svc.Port], container)
	}
	for port, containers := range byPort {
		if len(containers) > 1 {
			sort.Strings(containers)
			return fmt.Errorf("%w: containers %v all declare port %d", ErrPortCollision, containers, port)
		}
	}
	return nil
}
