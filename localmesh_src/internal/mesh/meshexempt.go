// Package mesh derives Envoy bootstrap config from the plugin catalog.
//
// The package's primary entrypoint is RenderAll, called from
// localmesh build after compose has been merged.
package mesh

// MeshExemptLabel is the compose label that opts a service out of mesh
// participation (no sidecar, no iptables, no cert mint). Observability
// containers, dex, and postgres carry this label.
const MeshExemptLabel = "mesh.exempt"

// IsExempt reports whether a compose labels map carries the exempt mark.
func IsExempt(labels map[string]string) bool {
	return labels[MeshExemptLabel] == "true"
}
