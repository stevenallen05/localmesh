package mesh

import (
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func TestBuildRegistry(t *testing.T) {
	proj := &manifest.Project{
		Name: "test", LocalDomain: "lvh.me",
		Services: []manifest.Service{
			{Container: "app-a", Port: 3000, Scheme: "http"},
		},
	}
	plugins := []*manifest.Plugin{
		{Name: "mesh", Identity: manifest.Identity{ModuleName: "mesh"}, Services: []manifest.Service{
			{Container: "ingress-mesh", Port: 8443, Scheme: "https", Ingress: true},
			{Container: "egress-mesh", Port: 15001, Scheme: "tcp"},
		}},
		{Name: "app-b", Identity: manifest.Identity{ModuleName: "app"}, Services: []manifest.Service{
			{Container: "app-b", Port: 50051, Scheme: "grpc"},
		}},
	}
	reg := BuildRegistry(proj, plugins)
	cases := []struct {
		container  string
		wantPort   int
		wantScheme string
	}{
		{"app-a", 3000, "http"},
		{"app-b", 50051, "grpc"},
		{"ingress-mesh", 8443, "https"},
		{"egress-mesh", 15001, "tcp"},
	}
	for _, c := range cases {
		svc, ok := reg.Get(c.container)
		if !ok {
			t.Errorf("registry missing %q", c.container)
			continue
		}
		if svc.Port != c.wantPort {
			t.Errorf("%s: port = %d, want %d", c.container, svc.Port, c.wantPort)
		}
		if svc.Scheme != c.wantScheme {
			t.Errorf("%s: scheme = %q, want %q", c.container, svc.Scheme, c.wantScheme)
		}
	}
}

func TestBuildRegistry_DuplicateContainerErrors(t *testing.T) {
	proj := &manifest.Project{Name: "test", LocalDomain: "lvh.me"}
	plugins := []*manifest.Plugin{
		{Name: "a", Identity: manifest.Identity{ModuleName: "a"}, Services: []manifest.Service{
			{Container: "dup", Port: 1000, Scheme: "http"},
		}},
		{Name: "b", Identity: manifest.Identity{ModuleName: "b"}, Services: []manifest.Service{
			{Container: "dup", Port: 2000, Scheme: "http"},
		}},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate container")
		}
	}()
	_ = BuildRegistry(proj, plugins)
}
