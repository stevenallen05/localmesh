package mesh

import (
	"strings"
	"testing"
)

func TestRenderIptables_Outbound(t *testing.T) {
	workload := IptablesContext{
		Container:     "www",
		InboundPort:   3443,
		EnvoyInbound:  15006,
		EnvoyOutbound: 15001,
	}
	out, err := RenderIptables(workload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// catch-all outbound rule: all non-loopback TCP SYN → envoy outbound.
	// Envoy routes per-edge via filter_chain_match on destination_port, so
	// iptables intentionally does NOT install per-edge rules.
	if !strings.Contains(out, "OUTPUT -p tcp ! -d 127.0.0.1/32") || !strings.Contains(out, "--to-ports 15001") {
		t.Errorf("expected catch-all outbound REDIRECT -> 15001:\n%s", out)
	}
	// inbound: redirect workload's port
	if !strings.Contains(out, "--dport 3443") || !strings.Contains(out, "--to-ports 15006") {
		t.Errorf("expected inbound REDIRECT 3443 -> 15006:\n%s", out)
	}
	// shebang + set -e for safety
	if !strings.HasPrefix(out, "#!/bin/sh\nset -eu\n") {
		t.Errorf("expected '#!/bin/sh\\nset -eu\\n' header; got:\n%s", out)
	}
}

func TestRenderIptables_NoOutbounds(t *testing.T) {
	// service with no depends_on still needs inbound redirection
	workload := IptablesContext{
		Container:     "callee",
		InboundPort:   50051,
		EnvoyInbound:  15006,
		EnvoyOutbound: 15001,
	}
	out, err := RenderIptables(workload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "--dport 50051") {
		t.Errorf("expected inbound redirect for 50051")
	}
	// catch-all outbound rule still emitted
	if !strings.Contains(out, "OUTPUT -p tcp ! -d 127.0.0.1/32") {
		t.Errorf("expected catch-all outbound rule")
	}
}
