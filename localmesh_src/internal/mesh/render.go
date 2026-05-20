package mesh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"

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

// BuildContexts walks the catalog to produce the per-role template
// contexts. Returns ErrPortCollision (from DeriveEdges) if the catalog
// has port collisions, or ErrUnknownDepends if any depends entry names
// a container that isn't in the registry.
func BuildContexts(proj *manifest.Project, plugins []*manifest.Plugin) (*Contexts, error) {
	reg := BuildRegistry(proj, plugins)
	edges, err := DeriveEdges(reg)
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

// RenderAll renders the ingress, egress, and per-workload sidecar envoy
// YAML configs into outputDir (typically .localmesh/envoy/). Also emits a
// shell script per workload at <outputDir>/<container>-mesh-init.sh that
// the iptables-init container runs.
//
// templatesDir points at the mesh plugin's templates/ directory (typically
// service_catalog/mesh/templates).
func RenderAll(proj *manifest.Project, plugins []*manifest.Plugin, compose ComposeAggregate, outputDir, templatesDir string) error {
	ctxs, err := BuildContexts(proj, plugins, compose)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outputDir, err)
	}
	helperPath := filepath.Join(templatesDir, "_helpers.tmpl")
	helperData, err := os.ReadFile(helperPath)
	if err != nil {
		return fmt.Errorf("read helpers: %w", err)
	}
	render := func(mainTmpl string, data any, outPath string, mode os.FileMode) error {
		mainData, err := os.ReadFile(filepath.Join(templatesDir, mainTmpl))
		if err != nil {
			return fmt.Errorf("read %s: %w", mainTmpl, err)
		}
		// Pre-create template so we can register `include` against this
		// instance. Helm's `include` runs a named template and returns the
		// output as a string — the canonical workaround for text/template's
		// built-in `template` not being usable in pipelines.
		var tmpl *template.Template
		include := func(name string, data any) (string, error) {
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
				return "", err
			}
			return buf.String(), nil
		}
		tmpl = template.New(mainTmpl).
			Funcs(sprig.TxtFuncMap()).
			Funcs(template.FuncMap{"include": include}).
			Option("missingkey=error")
		if _, err := tmpl.Parse(string(helperData)); err != nil {
			return fmt.Errorf("parse helpers: %w", err)
		}
		if _, err := tmpl.Parse(string(mainData)); err != nil {
			return fmt.Errorf("parse %s: %w", mainTmpl, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("execute %s: %w", mainTmpl, err)
		}
		if err := os.WriteFile(outPath, buf.Bytes(), mode); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		return nil
	}
	if err := render("ingress-gateway.envoy.yaml.gotmpl", ctxs.Ingress, filepath.Join(outputDir, "ingress-mesh.yaml"), 0o644); err != nil {
		return err
	}
	if err := render("egress-gateway.envoy.yaml.gotmpl", ctxs.Egress, filepath.Join(outputDir, "egress-mesh.yaml"), 0o644); err != nil {
		return err
	}
	// Sorted iteration for deterministic write order (also keeps goldens stable).
	names := make([]string, 0, len(ctxs.Sidecars))
	for n := range ctxs.Sidecars {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		sidecar := ctxs.Sidecars[name]
		if err := render("workload-sidecar.envoy.yaml.gotmpl", sidecar, filepath.Join(outputDir, name+"-mesh.yaml"), 0o644); err != nil {
			return err
		}
		script, err := RenderIptables(IptablesContext{
			Container:     name,
			InboundPort:   sidecar.InboundPort,
			EnvoyInbound:  sidecar.EnvoyInbound,
			EnvoyOutbound: sidecar.EnvoyOutbound,
		})
		if err != nil {
			return fmt.Errorf("iptables %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(outputDir, name+"-mesh-init.sh"), []byte(script), 0o755); err != nil {
			return fmt.Errorf("write iptables script for %s: %w", name, err)
		}
	}
	return nil
}
