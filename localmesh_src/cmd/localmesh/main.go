// Package main is the localmesh CLI entry point.
//
// See docs/engineering/rules/golang-basics.md for style conventions.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "localmesh",
		Short:         "LocalMesh dev-environment CLI",
		SilenceErrors: true, // we print our own error format in main()
		SilenceUsage:  true,
		// TODO: needs_prod_decisions validate verb when go schema validator picked
		// TODO: needs_prod_decisions install verb for production deployment
	}
	root.AddCommand(newBuildCmd(), newCACmd(), newMTLSCmd())
	return root
}
