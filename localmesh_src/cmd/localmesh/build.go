package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/stevenallen05/localmesh/internal/envwriter"
	"github.com/stevenallen05/localmesh/internal/manifest"
	"github.com/stevenallen05/localmesh/internal/render"
)

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Regenerate .env managed section + render localmesh/bundled.compose.yaml from plugin templates",
		RunE: func(*cobra.Command, []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getcwd: %w", err)
			}
			proj, plugins, err := manifest.LoadAll("project.toml", "localmesh/service_catalog")
			if err != nil {
				return err
			}
			// .env first — dex.yaml seeding in the Makefile reads .env
			// for $LOCALMESH_OIDC_CLIENT_SECRET substitution.
			if err := envwriter.WriteManaged(repoRoot, proj, plugins); err != nil {
				return err
			}
			composePath := filepath.Join("localmesh", "bundled.compose.yaml")
			return render.RunWith(proj, plugins, "localmesh/service_catalog", composePath)
		},
	}
}
