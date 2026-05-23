package template

import (
	"fmt"
	"text/template"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

func customFuncs() template.FuncMap {
	return template.FuncMap{
		"spiffeURI":      spiffeURI,
		"identityLabels": identityLabels,
	}
}

func spiffeURI(container, project, localDomain string) string {
	return fmt.Sprintf("spiffe://%s.%s.%s", container, project, localDomain)
}

// identityLabels emits a three-key YAML label block (indented by the caller).
// `service_name` is the per-container override; module + owner come from the plugin.
func identityLabels(plugin *manifest.Plugin, serviceName string) string {
	return fmt.Sprintf(
		"metrics.service_name: %s\nmetrics.module_name: %s\nmetrics.owned_by: %s",
		serviceName, plugin.Identity.ModuleName, plugin.Identity.OwnedBy,
	)
}

// SchemeProtocol maps a [[services]].scheme value to a Kuma kuma.io/protocol
// dataplane inbound tag. Closed map; unknown scheme is a build error so a
// new app-tier protocol doesn't fall back to a silent "tcp" default.
func SchemeProtocol(scheme string) (string, error) {
	switch scheme {
	case "http", "https":
		return "http", nil
	case "grpc":
		return "grpc", nil
	case "tcp", "postgresql":
		return "tcp", nil
	default:
		return "", fmt.Errorf("scheme %q has no kuma.io/protocol mapping", scheme)
	}
}
