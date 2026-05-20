package main

import "github.com/spf13/cobra"

func newCACmd() *cobra.Command {
	ca := &cobra.Command{
		Use:   "ca",
		Short: "Root CA lifecycle (via mkcert)",
	}
	ca.AddCommand(newCAMintCmd(), newCAInstallCmd(), newCAUninstallCmd())
	return ca
}

func newCAMintCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "mint",
		Short: "Generate a new root CA (requires --force if one exists)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented("ca mint")
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing CA")
	return cmd
}

func newCAInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install root CA into host trust store (requires sudo)",
		RunE:  func(*cobra.Command, []string) error { return errNotImplemented("ca install") },
	}
}

func newCAUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove root CA from host trust store (requires sudo)",
		RunE:  func(*cobra.Command, []string) error { return errNotImplemented("ca uninstall") },
	}
}
