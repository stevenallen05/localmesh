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

func TestSchemeProtocol(t *testing.T) {
	tests := []struct {
		scheme  string
		want    string
		wantErr bool
	}{
		{"http", "http", false},
		{"https", "http", false},
		{"grpc", "grpc", false},
		{"tcp", "tcp", false},
		{"postgresql", "tcp", false},
		{"", "", true},
		{"redis", "", true},   // sanity: 'redis' is not a valid scheme per manifest validator
		{"frobnitz", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			got, err := schemeProtocol(tt.scheme)
			if (err != nil) != tt.wantErr {
				t.Fatalf("schemeProtocol(%q) error = %v, wantErr %v", tt.scheme, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("schemeProtocol(%q) = %q, want %q", tt.scheme, got, tt.want)
			}
		})
	}
}
