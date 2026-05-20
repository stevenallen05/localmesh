package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tensorwave/localmesh/internal/envwriter"
	"github.com/tensorwave/localmesh/internal/manifest"
	"github.com/tensorwave/localmesh/internal/render"
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
			// .env first — Caddyfile + dex.yaml generation (still in
			// scripts/secrets-gen.py for now) reads .env for substitution.
			if err := envwriter.WriteManaged(repoRoot, proj, plugins); err != nil {
				return err
			}
			return render.RunWith(proj, plugins, "service_catalog", ".localmesh/localmesh.compose.yaml")
		},
	}
}
