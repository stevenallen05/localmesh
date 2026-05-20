package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMTLSCmd() *cobra.Command {
	m := &cobra.Command{
		Use:   "mtls",
		Short: "Leaf cert lifecycle (native Go crypto/x509)",
	}
	m.AddCommand(&cobra.Command{
		Use:   "mint",
		Short: "Mint leaf certs for every workload (always regenerates; 7-day lifetime)",
		RunE:  func(*cobra.Command, []string) error { return errNotImplemented("mtls mint") },
	})
	return m
}

func errNotImplemented(verb string) error {
	return fmt.Errorf("%s: not yet implemented", verb)
}
