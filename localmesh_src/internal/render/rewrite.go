package render

import (
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RewriteRelativePaths walks a parsed plugin compose doc and prefixes
// every relative path with relPrefix. Absolute paths and paths starting
// with `/` are left alone. This compensates for the fact that the
// localmesh merger flattens N plugin compose files (each authored
// relative to its own directory) into a single file at a different
// directory.
//
// Surfaces rewritten:
//   - services.<svc>.build (string form — context shorthand)
//   - services.<svc>.build.context (mapping form)
//   - services.<svc>.volumes[i] (short-form `host:container[:mode]`)
//   - configs.<name>.file
//   - secrets.<name>.file
//
// Long-form volumes (mapping with `source:`) are not rewritten today —
// the catalog uses short-form throughout. TODO: needs_prod_decisions
// long-form volume rewriting when a plugin adopts it.
func RewriteRelativePaths(doc *yaml.Node, relPrefix string) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(top.Content); i += 2 {
		k, v := top.Content[i], top.Content[i+1]
		switch k.Value {
		case "services":
			rewriteServices(v, relPrefix)
		case "configs", "secrets":
			rewriteFileEntries(v, relPrefix)
		}
	}
}

func rewriteServices(services *yaml.Node, relPrefix string) {
	if services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(services.Content); i += 2 {
		svc := services.Content[i+1]
		if svc.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j < len(svc.Content); j += 2 {
			fk, fv := svc.Content[j], svc.Content[j+1]
			switch fk.Value {
			case "build":
				rewriteBuild(fv, relPrefix)
			case "volumes":
				rewriteVolumes(fv, relPrefix)
			}
		}
	}
}

func rewriteBuild(n *yaml.Node, relPrefix string) {
	if n.Kind == yaml.ScalarNode {
		n.Value = prefixIfRelative(n.Value, relPrefix)
		return
	}
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == "context" {
			n.Content[i+1].Value = prefixIfRelative(n.Content[i+1].Value, relPrefix)
		}
	}
}

func rewriteVolumes(n *yaml.Node, relPrefix string) {
	if n.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}
		// Short form: "host:container[:mode]". Named volumes have no `/`
		// or `.` prefix and are not paths — skip them.
		parts := strings.SplitN(item.Value, ":", 2)
		if len(parts) != 2 {
			continue
		}
		host := parts[0]
		if !isRelativePath(host) {
			continue
		}
		item.Value = prefixIfRelative(host, relPrefix) + ":" + parts[1]
	}
}

func rewriteFileEntries(n *yaml.Node, relPrefix string) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(n.Content); i += 2 {
		entry := n.Content[i+1]
		if entry.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j < len(entry.Content); j += 2 {
			if entry.Content[j].Value == "file" {
				entry.Content[j+1].Value = prefixIfRelative(entry.Content[j+1].Value, relPrefix)
			}
		}
	}
}

func isRelativePath(p string) bool {
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "/") {
		return false
	}
	if strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") || p == "." || p == ".." {
		return true
	}
	// Anything else (named volume, image ref) — leave alone.
	return false
}

func prefixIfRelative(p, relPrefix string) string {
	if !isRelativePath(p) {
		return p
	}
	joined := filepath.Join(relPrefix, p)
	// filepath.Join cleans the result, stripping any leading "./". A compose
	// short-form volume whose source lacks a "./" or "../" prefix is parsed
	// as a NAMED volume, not a bind mount — so when the plugin dir sits below
	// the bundle dir (relPrefix has no "../"), re-add "./" to keep it a path.
	if !strings.HasPrefix(joined, "./") && !strings.HasPrefix(joined, "../") {
		joined = "./" + joined
	}
	return joined
}
