package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tensorwave/localmesh/internal/ca"
)

func newCACmd() *cobra.Command {
	c := &cobra.Command{Use: "ca", Short: "Root CA lifecycle (via mkcert)"}
	c.AddCommand(newCAMintCmd(), newCAInstallCmd(), newCAUninstallCmd())
	return c
}

func newCAMintCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "mint",
		Short: "Generate a new root CA",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getcwd: %w", err)
			}
			return ca.New(repoRoot).Mint(cmd.Context(), force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing CA")
	return cmd
}

func newCAInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install root CA into host trust store (requires sudo)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			return ca.New(repoRoot).Install(cmd.Context())
		},
	}
}

func newCAUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove root CA from host trust store (requires sudo)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			return ca.New(repoRoot).Uninstall(cmd.Context())
		},
	}
}
