package mesh

import (
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func TestBuildContexts(t *testing.T) {
	proj := &manifest.Project{Name: "demo", LocalDomain: "lvh.me", ExternalDomain: "demo.example.com"}
	plugins := []*manifest.Plugin{
		{Name: "mesh", Identity: manifest.Identity{ModuleName: "mesh", OwnedBy: "sre@example.com"}, Services: []manifest.Service{
			{Container: "ingress-mesh", Port: 8443, Scheme: "https", Ingress: true},
			{Container: "egress-mesh", Port: 15001, Scheme: "tcp"},
		}},
		{Name: "app-b", Identity: manifest.Identity{ModuleName: "app", OwnedBy: "tl@example.com"}, Services: []manifest.Service{
			{Container: "app-b", Port: 50051, Scheme: "grpc"},
		}},
	}
	proj.Services = []manifest.Service{
		{Container: "app-a", Port: 3000, Scheme: "http", ExposeViaIngress: true},
	}
	compose := ComposeAggregate{
		"app-a":        {DependsOn: []string{"app-b"}, Labels: map[string]string{}},
		"app-b":        {DependsOn: nil, Labels: map[string]string{}},
		"ingress-mesh": {DependsOn: nil, Labels: map[string]string{}},
		"egress-mesh":  {DependsOn: nil, Labels: map[string]string{}},
	}
	ctxs, err := BuildContexts(proj, plugins, compose)
	if err != nil {
		t.Fatalf("build contexts: %v", err)
	}
	if len(ctxs.Sidecars) != 2 {
		t.Errorf("expected 2 sidecar contexts (app-a, app-b), got %d", len(ctxs.Sidecars))
	}
	// ingress + egress contexts always present
	if ctxs.Ingress.ProjectName != "demo" {
		t.Errorf("ingress ctx project = %q, want demo", ctxs.Ingress.ProjectName)
	}
	if len(ctxs.Ingress.Routes) == 0 {
		t.Errorf("expected at least one ingress route (app-a expose_via_ingress=true)")
	}

	// AppLoopbackPort convention: InboundPort + 1
	if sc := ctxs.Sidecars["app-a"]; sc.AppLoopbackPort != sc.InboundPort+1 {
		t.Errorf("app-a AppLoopbackPort = %d, want InboundPort+1 = %d", sc.AppLoopbackPort, sc.InboundPort+1)
	}
}
