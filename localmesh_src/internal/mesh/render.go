package mesh

import (
	"fmt"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

// IngressCtx feeds ingress-gateway.envoy.yaml.gotmpl.
type IngressCtx struct {
	ProjectName, LocalDomain, ExternalDomain string
	DexIssuer, DexJWKSURI                    string
	OIDCClientID, OIDCClientSecret           string
	Routes                                   []IngressRoute
	AdminTargets                             []AdminTarget
	OTelEndpoint                             string
}

// IngressRoute is one host → cluster edge on the ingress-mesh listener.
type IngressRoute struct {
	Host         string
	Cluster      string
	Port         int
	RequiresAuth bool
}

// AdminTarget is one admin-proxy target the ingress aggregates under /admin/.
// Empty Name = ingress-mesh's own admin; else "<container>-mesh".
type AdminTarget struct {
	Name string
}

// SidecarCtx feeds workload-sidecar.envoy.yaml.gotmpl, one per workload.
type SidecarCtx struct {
	Name            string // <workload> (not <workload>-mesh)
	InboundPort     int    // workload's [[services]] port — iptables redirects here to envoy inbound
	AppLoopbackPort int    // app's plaintext bind port on 127.0.0.1 — convention: InboundPort + 1
	Outbounds       []Edge
	CertDir         string // /run/<name>
	OTelEndpoint    string
	EnvoyInbound    int
	EnvoyOutbound   int
}

// EgressCtx feeds egress-gateway.envoy.yaml.gotmpl.
type EgressCtx struct {
	ProjectName, LocalDomain string
	OTelEndpoint             string
}

// Contexts is the assembled render input for a project.
type Contexts struct {
	Ingress  IngressCtx
	Egress   EgressCtx
	Sidecars map[string]SidecarCtx // keyed by container name
}

// BuildContexts walks the catalog + compose merge to produce the per-role
// template contexts. Returns ErrPortCollision (from DeriveEdges) if the
// catalog has port collisions.
func BuildContexts(proj *manifest.Project, plugins []*manifest.Plugin, compose ComposeAggregate) (*Contexts, error) {
	reg := BuildRegistry(proj, plugins)
	edges, err := DeriveEdges(compose, reg)
	if err != nil {
		return nil, fmt.Errorf("derive edges: %w", err)
	}
	otelEndpoint := "otel-collector:4317" // convention; observability plugin emits this hostname

	ctxs := &Contexts{
		Ingress: IngressCtx{
			ProjectName:    proj.Name,
			LocalDomain:    proj.LocalDomain,
			ExternalDomain: proj.ExternalDomain,
			OTelEndpoint:   otelEndpoint,
			// DexIssuer/JWKS/OIDCClient* sourced from .env at runtime via compose
			// interpolation in the template — not at build time.
		},
		Egress: EgressCtx{
			ProjectName:  proj.Name,
			LocalDomain:  proj.LocalDomain,
			OTelEndpoint: otelEndpoint,
		},
		Sidecars: map[string]SidecarCtx{},
	}

	// Ingress routes: every service with expose_via_ingress=true.
	// Exempt services (dex, grafana) still appear on ingress — their cluster
	// target is bare "<container>", not "<container>-mesh" (no sidecar hop).
	for _, container := range reg.AllContainers() {
		svc, _ := reg.Get(container)
		if !svc.ExposeViaIngress {
			continue
		}
		requiresAuth := true
		if svc.RequiresAuth != nil {
			requiresAuth = *svc.RequiresAuth
		}
		ctxs.Ingress.Routes = append(ctxs.Ingress.Routes, IngressRoute{
			Host:         fmt.Sprintf("%s.%s.%s", container, proj.Name, proj.LocalDomain),
			Cluster:      meshClusterName(container, compose),
			Port:         svc.Port,
			RequiresAuth: requiresAuth,
		})
	}

	// Admin targets: ingress-mesh's own + per-non-exempt-workload + egress-mesh.
	ctxs.Ingress.AdminTargets = []AdminTarget{{Name: ""}} // self
	for _, container := range reg.AllContainers() {
		if cs, ok := compose[container]; ok && IsExempt(cs.Labels) {
			continue
		}
		// exclude ingress-mesh from sidecar-pattern admins (it's the root admin)
		if container == "ingress-mesh" {
			continue
		}
		// egress-mesh is a standalone envoy, not a sidecar
		if container == "egress-mesh" {
			ctxs.Ingress.AdminTargets = append(ctxs.Ingress.AdminTargets, AdminTarget{Name: "egress-mesh"})
			continue
		}
		ctxs.Ingress.AdminTargets = append(ctxs.Ingress.AdminTargets, AdminTarget{Name: container + "-mesh"})
	}

	// Per-workload sidecar contexts
	for _, container := range reg.AllContainers() {
		if cs, ok := compose[container]; ok && IsExempt(cs.Labels) {
			continue
		}
		// mesh roles (ingress + egress) don't get a sidecar themselves
		if container == "ingress-mesh" || container == "egress-mesh" {
			continue
		}
		svc, _ := reg.Get(container)
		ctxs.Sidecars[container] = SidecarCtx{
			Name:            container,
			InboundPort:     svc.Port,
			AppLoopbackPort: svc.Port + 1, // TODO: needs_prod_decisions formalize app-loopback port discovery
			Outbounds:       edges[container],
			CertDir:         "/run/" + container,
			OTelEndpoint:    otelEndpoint,
			EnvoyInbound:    15006,
			EnvoyOutbound:   15001,
		}
	}
	return ctxs, nil
}

// meshClusterName resolves the envoy cluster name for an ingress route:
// "<container>-mesh" for non-exempt services, bare "<container>" for exempt
// (dex, grafana) whose ingress hop is plaintext (no sidecar termination).
func meshClusterName(container string, compose ComposeAggregate) string {
	if cs, ok := compose[container]; ok && IsExempt(cs.Labels) {
		return container
	}
	return container + "-mesh"
}

// RenderAll is wired in Chunk 2. For now: no-op success so build can call it.
func RenderAll(proj *manifest.Project, plugins []*manifest.Plugin, compose ComposeAggregate, outputDir string) error {
	_, err := BuildContexts(proj, plugins, compose)
	if err != nil {
		return err
	}
	// Template rendering arrives in Chunk 2.
	_ = outputDir
	return nil
}
