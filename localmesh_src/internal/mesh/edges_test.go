package mesh

import (
	"errors"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func TestDeriveEdges_FiltersExempt(t *testing.T) {
	reg := testRegistry(t)
	compose := ComposeAggregate{
		"app-a": ComposeService{
			DependsOn: []string{"app-b", "otel-collector"},
			Labels:    map[string]string{},
		},
		"app-b": ComposeService{
			DependsOn: []string{},
			Labels:    map[string]string{},
		},
		"otel-collector": ComposeService{
			Labels: map[string]string{"mesh.exempt": "true"},
		},
	}
	edges, err := DeriveEdges(compose, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges["app-a"]) != 1 {
		t.Fatalf("app-a edges = %v, want 1", edges["app-a"])
	}
	if edges["app-a"][0].Target != "app-b" {
		t.Errorf("edge target = %q, want app-b", edges["app-a"][0].Target)
	}
	if edges["app-a"][0].TargetPort != 50051 {
		t.Errorf("edge port = %d, want 50051", edges["app-a"][0].TargetPort)
	}
}

func TestDeriveEdges_PortCollision(t *testing.T) {
	// app-c and app-d both declare port 50051; app-a depends on both
	reg := registryWith(map[string]int{
		"app-a": 3000, "app-c": 50051, "app-d": 50051,
	})
	compose := ComposeAggregate{
		"app-a": ComposeService{DependsOn: []string{"app-c", "app-d"}},
		"app-c": ComposeService{},
		"app-d": ComposeService{},
	}
	_, err := DeriveEdges(compose, reg)
	if !errors.Is(err, ErrPortCollision) {
		t.Errorf("expected ErrPortCollision, got %v", err)
	}
}

func TestDeriveEdges_UnresolvedDepWarnNotFail(t *testing.T) {
	reg := testRegistry(t)
	compose := ComposeAggregate{
		"app-a": ComposeService{DependsOn: []string{"app-b", "unknown-external"}},
		"app-b": ComposeService{},
	}
	edges, err := DeriveEdges(compose, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges["app-a"]) != 1 {
		t.Errorf("expected 1 edge (unknown dep dropped), got %d", len(edges["app-a"]))
	}
}

// helpers
func testRegistry(t *testing.T) *Registry {
	t.Helper()
	return registryWith(map[string]int{"app-a": 3000, "app-b": 50051})
}

func registryWith(ports map[string]int) *Registry {
	svcs := make([]manifest.Service, 0, len(ports))
	for c, p := range ports {
		svcs = append(svcs, manifest.Service{Container: c, Port: p, Scheme: "http"})
	}
	return BuildRegistry(
		&manifest.Project{Name: "test", LocalDomain: "lvh.me", Services: svcs},
		nil,
	)
}
