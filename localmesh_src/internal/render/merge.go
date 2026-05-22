package render

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Merge folds a list of rendered plugin compose docs into one yaml.Node.
//
// Generic behavior (spec §2.5 simplified for v0):
//   - All mapping nodes: deep-merge, last-wins on equal keys
//   - All sequence nodes: list concatenation
//   - Scalar conflicts: fail loud with both source plugin names
//
// This satisfies the spec's per-field rule set for the compose surface
// the project actually uses (configs/volumes/ports concat; environment/
// labels/depends_on deep-merge) because every concat-flavored field IS
// a sequence and every deep-merge-flavored field IS a mapping. If a
// future field needs different semantics, switch on path-suffix here.
//
// sources[i] names the plugin that produced docs[i]; used in error messages.
func Merge(docs []*yaml.Node, sources []string) (*yaml.Node, error) {
	if len(docs) == 0 {
		return &yaml.Node{Kind: yaml.DocumentNode}, nil
	}
	out := docs[0]
	for i := 1; i < len(docs); i++ {
		if err := mergeDocs(out, docs[i], sources[0], sources[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func mergeDocs(dst, src *yaml.Node, dstSrc, srcSrc string) error {
	if dst.Kind != yaml.DocumentNode || src.Kind != yaml.DocumentNode {
		return fmt.Errorf("non-document node in merge: dst=%s src=%s", dstSrc, srcSrc)
	}
	if len(src.Content) == 0 {
		return nil
	}
	if len(dst.Content) == 0 {
		dst.Content = src.Content
		return nil
	}
	return mergeMapping(dst.Content[0], src.Content[0], dstSrc, srcSrc, "")
}

func mergeMapping(dst, src *yaml.Node, dstSrc, srcSrc, path string) error {
	if src.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(src.Content); i += 2 {
		k, v := src.Content[i], src.Content[i+1]
		_, dv := findKey(dst, k.Value)
		if dv == nil {
			dst.Content = append(dst.Content, k, v)
			continue
		}
		subPath := path + "." + k.Value
		if err := mergeNode(dv, v, dstSrc, srcSrc, subPath); err != nil {
			return err
		}
	}
	return nil
}

func mergeNode(dst, src *yaml.Node, dstSrc, srcSrc, path string) error {
	// Scalars: equal? OK. Different? conflict.
	if dst.Kind == yaml.ScalarNode && src.Kind == yaml.ScalarNode {
		if dst.Value == src.Value {
			return nil
		}
		return fmt.Errorf("conflict at %s: %q (from %s) vs %q (from %s)",
			path, dst.Value, dstSrc, src.Value, srcSrc)
	}
	if dst.Kind == yaml.MappingNode && src.Kind == yaml.MappingNode {
		// depends_on may need list→map normalization before deep-merge.
		// For v0, mapping-merge handles the common case where both sides
		// already arrived as mappings (compose-spec list form is rare in
		// plugin compose files).
		return mergeMapping(dst, src, dstSrc, srcSrc, path)
	}
	if dst.Kind == yaml.SequenceNode && src.Kind == yaml.SequenceNode {
		// List-concat per spec §2.5 (configs, volumes, ports, etc.)
		dst.Content = append(dst.Content, src.Content...)
		return nil
	}
	return fmt.Errorf("kind mismatch at %s: dst=%v src=%v (from %s vs %s)",
		path, dst.Kind, src.Kind, dstSrc, srcSrc)
}

func findKey(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}
