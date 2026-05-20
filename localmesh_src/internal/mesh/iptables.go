package mesh

import (
	"fmt"
	"strings"
)

// IptablesContext is the per-workload context for init-script rendering.
type IptablesContext struct {
	Container     string
	InboundPort   int    // workload's [[services]] port; redirected inbound
	Outbounds     []Edge // per-edge dport redirected outbound
	EnvoyInbound  int    // envoy's inbound listener (typically 15006)
	EnvoyOutbound int    // envoy's outbound listener (typically 15001)
}

// RenderIptables produces a /bin/sh script that an init container runs in
// the workload's netns to install transparent-interception rules.
//
// The script is idempotent: existing rules are flushed before insertion so
// a re-run replaces them rather than appending duplicates.
func RenderIptables(ctx IptablesContext) (string, error) {
	if ctx.InboundPort == 0 {
		return "", fmt.Errorf("iptables: inbound port required for %s", ctx.Container)
	}
	if ctx.EnvoyInbound == 0 || ctx.EnvoyOutbound == 0 {
		return "", fmt.Errorf("iptables: envoy inbound + outbound ports required for %s", ctx.Container)
	}
	var b strings.Builder
	fmt.Fprintln(&b, "#!/bin/sh")
	fmt.Fprintln(&b, "set -eu")
	fmt.Fprintf(&b, "# Mesh interception rules for %s — installed by mesh-init.\n", ctx.Container)
	fmt.Fprintln(&b, "iptables -t nat -F OUTPUT 2>/dev/null || true")
	fmt.Fprintln(&b, "iptables -t nat -F PREROUTING 2>/dev/null || true")
	// Per-edge outbound redirects (specific ports → envoy outbound)
	for _, e := range ctx.Outbounds {
		fmt.Fprintf(&b, "iptables -t nat -A OUTPUT -p tcp --dport %d -j REDIRECT --to-ports %d\n",
			e.TargetPort, ctx.EnvoyOutbound)
	}
	// Catch-all outbound for everything else (excluding loopback)
	fmt.Fprintf(&b, "iptables -t nat -A OUTPUT -p tcp ! -d 127.0.0.1/32 --syn -j REDIRECT --to-ports %d\n",
		ctx.EnvoyOutbound)
	// Inbound: workload's [[services]] port → envoy inbound
	fmt.Fprintf(&b, "iptables -t nat -A PREROUTING -p tcp --dport %d -j REDIRECT --to-ports %d\n",
		ctx.InboundPort, ctx.EnvoyInbound)
	fmt.Fprintln(&b, "exit 0")
	return b.String(), nil
}
