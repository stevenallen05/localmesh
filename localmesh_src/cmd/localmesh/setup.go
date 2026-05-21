package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/stevenallen05/localmesh/internal/ca"
	"github.com/stevenallen05/localmesh/internal/dexseed"
	"github.com/stevenallen05/localmesh/internal/envwriter"
	"github.com/stevenallen05/localmesh/internal/manifest"
	"github.com/stevenallen05/localmesh/internal/mtls"
	"github.com/stevenallen05/localmesh/internal/render"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run the full LocalMesh bootstrap chain (uninstall → wipe → mint CA → install → mint leaves → build → dex-seed)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getcwd: %w", err)
			}
			ctx := cmd.Context()
			rootCA := ca.New(repoRoot)

			// Step 1: clear any previously-trusted CA on the host.
			// First-run safe: mkcert -uninstall errors when nothing is
			// trusted; we swallow that and move on.
			fmt.Println("==> Removing existing CA from host trust store")
			_ = rootCA.Uninstall(ctx)

			// Step 2: sledgehammer the local cert tree.
			fmt.Println("==> Wiping localmesh/secrets/")
			if err := os.RemoveAll(filepath.Join(repoRoot, "localmesh", "secrets")); err != nil {
				return fmt.Errorf("wipe secrets: %w", err)
			}

			// Step 3: mint a fresh LocalMesh root CA.
			fmt.Println("==> Minting LocalMesh root CA")
			if err := rootCA.Mint(ctx, true); err != nil {
				return fmt.Errorf("ca mint: %w", err)
			}

			// Step 4: install the root CA into the host trust store.
			fmt.Println("==> Installing CA into host trust store")
			if err := rootCA.Install(ctx); err != nil {
				return fmt.Errorf("ca install: %w", err)
			}

			// Step 5: mint per-service leaf certs.
			proj, plugins, err := manifest.LoadAll(
				filepath.Join(repoRoot, "project.toml"),
				filepath.Join(repoRoot, "localmesh", "service_catalog"),
			)
			if err != nil {
				return fmt.Errorf("manifest: %w", err)
			}
			fmt.Println("==> Minting per-service leaf certs (7d)")
			minter, err := mtls.New(repoRoot, proj.Name, proj.LocalDomain)
			if err != nil {
				return fmt.Errorf("mtls new: %w", err)
			}
			if err := minter.MintAll(collectMeshContainers(proj, plugins)); err != nil {
				return fmt.Errorf("mtls mint: %w", err)
			}

			// Step 6: render .env first, then bundled.compose.yaml.
			// Order is load-bearing for two reasons: .env carries the once-
			// preserved LOCALMESH_OIDC_CLIENT_SECRET that dexseed reads in
			// step 7; AND compose interpolation at `docker compose up` time
			// reads .env, so the render output must reflect the same managed
			// section the env file just received.
			fmt.Println("==> Rendering .env + bundled.compose.yaml")
			if err := envwriter.WriteManaged(repoRoot, proj, plugins); err != nil {
				return fmt.Errorf("envwriter: %w", err)
			}
			composePath := filepath.Join("localmesh", "bundled.compose.yaml")
			if err := render.RunWith(proj, plugins, "localmesh/service_catalog", composePath); err != nil {
				return fmt.Errorf("render: %w", err)
			}

			// Step 7: seed localmesh/dex.yaml from the sample on first run.
			env, err := envwriter.LoadEnv(filepath.Join(repoRoot, ".env"))
			if err != nil {
				return fmt.Errorf("load .env: %w", err)
			}
			samplePath := filepath.Join(repoRoot, "localmesh", "service_catalog", "auth", "dex.yaml.sample")
			targetPath := filepath.Join(repoRoot, "localmesh", "dex.yaml")
			skipped, err := dexseed.Seed(samplePath, targetPath, env)
			if err != nil {
				return fmt.Errorf("dexseed: %w", err)
			}
			if skipped {
				fmt.Println("==> Seeding localmesh/dex.yaml (skipped: file exists)")
			} else {
				fmt.Println("==> Seeded localmesh/dex.yaml from sample")
			}

			fmt.Println("setup complete.")
			return nil
		},
	}
}

// collectMeshContainers returns every container declared in project.toml
// [[services]] (app tier) plus every plugin.toml [[services]] entry.
// For v0 we mint for every declared service and let the workload ignore
// the mount when it doesn't need mTLS.
func collectMeshContainers(proj *manifest.Project, plugins []*manifest.Plugin) []string {
	out := []string{}
	for _, s := range proj.Services {
		out = append(out, s.Container)
	}
	for _, p := range plugins {
		for _, s := range p.Services {
			out = append(out, s.Container)
		}
	}
	return out
}
