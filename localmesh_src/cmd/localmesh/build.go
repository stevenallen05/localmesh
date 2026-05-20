package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/stevenallen05/localmesh/internal/envwriter"
	"github.com/stevenallen05/localmesh/internal/manifest"
	"github.com/stevenallen05/localmesh/internal/mesh"
	"github.com/stevenallen05/localmesh/internal/render"
)

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Regenerate .env managed section + render .localmesh/localmesh.compose.yaml from plugin templates",
		RunE: func(*cobra.Command, []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getcwd: %w", err)
			}
			proj, plugins, err := manifest.LoadAll("project.toml", "service_catalog")
			if err != nil {
				return err
			}
			// .env first — dex.yaml seeding (still in scripts/secrets-gen.py
			// for now) reads .env for substitution.
			if err := envwriter.WriteManaged(repoRoot, proj, plugins); err != nil {
				return err
			}
			composePath := filepath.Join(".localmesh", "localmesh.compose.yaml")
			if err := render.RunWith(proj, plugins, "service_catalog", composePath); err != nil {
				return err
			}
			composeAggregate, err := parseMergedCompose(composePath)
			if err != nil {
				return fmt.Errorf("parse merged compose: %w", err)
			}
			return mesh.RenderAll(
				proj, plugins, composeAggregate,
				filepath.Join(repoRoot, ".localmesh", "envoy"),
				filepath.Join(repoRoot, "service_catalog", "mesh", "templates"),
			)
		},
	}
}

// parseMergedCompose reads the rendered .localmesh/localmesh.compose.yaml and
// extracts the slim ComposeAggregate view mesh.RenderAll needs: per-service
// depends_on + labels.
//
// Compose normalizes depends_on to either the list form ([a, b, c]) or the
// map form ({a: {condition: ...}, b: {condition: ...}}); both yield the same
// keys. Labels follow the same shape but `key=value` strings appear in the
// sequence form.
func parseMergedCompose(path string) (mesh.ComposeAggregate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		Services map[string]struct {
			DependsOn yaml.Node `yaml:"depends_on"`
			Labels    yaml.Node `yaml:"labels"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	out := make(mesh.ComposeAggregate, len(doc.Services))
	for name, svc := range doc.Services {
		out[name] = mesh.ComposeService{
			DependsOn: dependsOnKeys(svc.DependsOn),
			Labels:    labelsMap(svc.Labels),
		}
	}
	return out, nil
}

// dependsOnKeys returns the dependency container names from either compose
// form (sequence of strings, or mapping keyed by service name).
func dependsOnKeys(n yaml.Node) []string {
	switch n.Kind {
	case yaml.SequenceNode:
		out := make([]string, 0, len(n.Content))
		for _, e := range n.Content {
			if e.Value != "" {
				out = append(out, e.Value)
			}
		}
		return out
	case yaml.MappingNode:
		out := make([]string, 0, len(n.Content)/2)
		for i := 0; i < len(n.Content); i += 2 {
			out = append(out, n.Content[i].Value)
		}
		return out
	}
	return nil
}

// labelsMap returns the labels map from either compose form (mapping or
// sequence of "key=value" strings).
func labelsMap(n yaml.Node) map[string]string {
	out := map[string]string{}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			out[n.Content[i].Value] = n.Content[i+1].Value
		}
	case yaml.SequenceNode:
		for _, e := range n.Content {
			if k, v, ok := strings.Cut(e.Value, "="); ok {
				out[k] = v
			}
		}
	}
	return out
}
