package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tensorwave/localmesh/internal/manifest"
	"github.com/tensorwave/localmesh/internal/mtls"
)

func newMTLSCmd() *cobra.Command {
	m := &cobra.Command{Use: "mtls", Short: "Leaf cert lifecycle (native Go crypto/x509)"}
	m.AddCommand(newMTLSMintCmd())
	return m
}

func newMTLSMintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mint",
		Short: "Mint leaf certs for every workload (always regenerates; 7-day lifetime)",
		RunE: func(*cobra.Command, []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			proj, plugins, err := manifest.LoadAll(filepath.Join(repoRoot, "service_catalog"))
			if err != nil {
				return err
			}
			minter, err := mtls.New(repoRoot, proj.Name, proj.LocalDomain)
			if err != nil {
				return err
			}
			containers := collectMeshContainers(plugins)
			if err := minter.MintAll(containers); err != nil {
				return err
			}
			fmt.Printf("minted %d leaf cert(s) under .localmesh/secrets/\n", len(containers))
			return nil
		},
	}
}

// collectMeshContainers returns every container declared across the loaded
// plugins. Today's secrets-gen.py reads compose YAML for the mesh.exempt
// carve-out; for v0 we mint for every service in plugin.toml [[services]].
// TODO: needs_prod_decisions mesh.exempt enforcement once localmesh parses compose
func collectMeshContainers(plugins []*manifest.Plugin) []string {
	out := []string{}
	for _, p := range plugins {
		for _, s := range p.Services {
			out = append(out, s.Container)
		}
	}
	return out
}
