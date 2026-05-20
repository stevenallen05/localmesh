package main

import "github.com/spf13/cobra"

func newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Render .localmesh/localmesh.compose.yaml from plugin templates",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented("build")
		},
	}
}
