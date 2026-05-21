package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/stevenallen05/localmesh/internal/devref"
	"github.com/stevenallen05/localmesh/internal/envwriter"
	"github.com/stevenallen05/localmesh/internal/manifest"
	"github.com/stevenallen05/localmesh/internal/render"
)

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Regenerate .env managed section + render localmesh/bundled.compose.yaml + localmesh/developer_reference.md from plugin templates",
		RunE: func(*cobra.Command, []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getcwd: %w", err)
			}
			catalogRoot := "localmesh/service_catalog"
			proj, plugins, err := manifest.LoadAll("project.toml", catalogRoot)
			if err != nil {
				return err
			}
			// .env first — dex.yaml seeding in setup reads .env
			// for $LOCALMESH_OIDC_CLIENT_SECRET substitution.
			if err := envwriter.WriteManaged(repoRoot, proj, plugins); err != nil {
				return err
			}
			composePath := filepath.Join("localmesh", "bundled.compose.yaml")
			if err := render.RunWith(proj, plugins, catalogRoot, composePath); err != nil {
				return err
			}
			return devref.Write(repoRoot, catalogRoot, proj, plugins)
		},
	}
}
