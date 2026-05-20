package mesh

import (
	"strings"
	"testing"
)

func TestRenderIptables_Outbound(t *testing.T) {
	edges := []Edge{
		{Target: "server", TargetPort: 50051},
		{Target: "auth", TargetPort: 5000},
	}
	workload := IptablesContext{
		Container:     "www",
		InboundPort:   3443,
		Outbounds:     edges,
		EnvoyInbound:  15006,
		EnvoyOutbound: 15001,
	}
	out, err := RenderIptables(workload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// outbound: redirect target ports
	if !strings.Contains(out, "--dport 50051") || !strings.Contains(out, "--to-ports 15001") {
		t.Errorf("expected outbound REDIRECT for 50051 -> 15001:\n%s", out)
	}
	if !strings.Contains(out, "--dport 5000") {
		t.Errorf("expected outbound REDIRECT for auth port 5000:\n%s", out)
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
		Outbounds:     nil,
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
