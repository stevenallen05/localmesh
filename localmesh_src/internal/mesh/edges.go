package mesh

import (
	"errors"
	"fmt"
	"sort"
)

// ErrPortCollision indicates two non-exempt registry services share a port,
// which makes iptables port-based redirection ambiguous from any caller
// that depends on both.
var ErrPortCollision = errors.New("port collision between non-exempt services")

// ComposeService is the subset of a parsed compose service entry the mesh
// derivation needs. The full compose merge is the responsibility of
// internal/render; this is the slim view edges.go consumes.
type ComposeService struct {
	DependsOn []string          // keys of depends_on (compose canonicalizes the map form)
	Labels    map[string]string // top-level service labels
}

// ComposeAggregate is the post-merge view: every service across the project +
// every included plugin, keyed by container name.
type ComposeAggregate map[string]ComposeService

// Edge represents one outbound mTLS edge from a workload to a peer.
type Edge struct {
	Target     string // peer container name
	TargetPort int    // peer's [[services]] port
}

// DeriveEdges walks each non-exempt service's compose depends_on, intersects
// the targets with registry-known containers, drops mesh-exempt targets, and
// returns the per-workload outbound edge list. Errors if any non-exempt
// services share a port.
//
// Foreign deps (depends_on names not in the registry) are dropped silently
// — they're legitimate cross-mesh boundaries that the workload sidecar
// passes through the catch-all.
func DeriveEdges(compose ComposeAggregate, reg *Registry) (map[string][]Edge, error) {
	if err := validatePortCollisions(compose, reg); err != nil {
		return nil, err
	}
	out := map[string][]Edge{}
	for src, svc := range compose {
		if IsExempt(svc.Labels) {
			continue
		}
		var edges []Edge
		for _, dep := range svc.DependsOn {
			target, ok := reg.Get(dep)
			if !ok {
				continue
			}
			depCompose, hasComposeEntry := compose[dep]
			if hasComposeEntry && IsExempt(depCompose.Labels) {
				continue
			}
			edges = append(edges, Edge{Target: dep, TargetPort: target.Port})
		}
		sort.Slice(edges, func(i, j int) bool { return edges[i].Target < edges[j].Target })
		if len(edges) > 0 {
			out[src] = edges
		}
	}
	return out, nil
}

func validatePortCollisions(compose ComposeAggregate, reg *Registry) error {
	byPort := map[int][]string{}
	for _, container := range reg.AllContainers() {
		svc, _ := reg.Get(container)
		if cs, ok := compose[container]; ok && IsExempt(cs.Labels) {
			continue
		}
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
