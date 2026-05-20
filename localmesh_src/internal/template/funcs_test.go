package template

import (
	"strings"
	"testing"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func TestSpiffeURI(t *testing.T) {
	got := spiffeURI("server", "metrics-collector", "lvh.me")
	want := "spiffe://server.metrics-collector.lvh.me"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestIdentityLabels(t *testing.T) {
	p := &manifest.Plugin{Identity: manifest.Identity{ModuleName: "alpha", OwnedBy: "a@x"}}
	got := identityLabels(p, "alpha-svc")
	for _, want := range []string{"metrics.service_name: alpha-svc", "metrics.module_name: alpha", "metrics.owned_by: a@x"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
