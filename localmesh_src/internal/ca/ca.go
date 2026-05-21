// Package ca wraps mkcert for root CA generation + host trust-store
// install/uninstall. Spec §2.6.
//
// CAROOT pinned to localmesh/secrets/root_ca/; mkcert writes
// rootCA.pem + rootCA-key.pem under that path.
package ca

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ErrAlreadyExists indicates a CA already exists at the target path
// and --force was not given.
var ErrAlreadyExists = errors.New("CA already exists; use --force to overwrite")

// CA wraps the mkcert binary and CAROOT path.
type CA struct {
	BinPath string // localmesh_src/tools/mkcert
	CARoot  string // localmesh/secrets/root_ca
}

// New returns a CA with defaults wired to the project's filesystem layout.
//
// The mkcert binary path defaults to `localmesh_src/tools/mkcert` under
// repoRoot. Override with `LOCALMESH_MKCERT=/abs/path/mkcert` — useful
// for smoke tests run from a scratch dir.
func New(repoRoot string) *CA {
	bin := os.Getenv("LOCALMESH_MKCERT")
	if bin == "" {
		bin = filepath.Join(repoRoot, "localmesh_src", "tools", "mkcert")
	}
	return &CA{
		BinPath: bin,
		CARoot:  filepath.Join(repoRoot, "localmesh", "secrets", "root_ca"),
	}
}

// Mint generates a new root CA at CAROOT. With force=false, refuses if
// a CA already exists.
//
// mkcert has no documented mint-only flag in v1.4.4, but invoking
// `mkcert -install` with `TRUST_STORES=none` mints the CA and skips
// every host trust store. That gives us an honest mint-only path
// for `ca mint`; the separate `ca install` verb is the one that
// actually touches the trust store (and prompts for sudo).
func (c *CA) Mint(ctx context.Context, force bool) error {
	if err := c.assertBinaryPresent(); err != nil {
		return err
	}
	caCrt := filepath.Join(c.CARoot, "rootCA.pem")
	if _, err := os.Stat(caCrt); err == nil && !force {
		return ErrAlreadyExists
	}
	if force {
		if err := os.RemoveAll(c.CARoot); err != nil {
			return fmt.Errorf("force-remove %s: %w", c.CARoot, err)
		}
	}
	if err := os.MkdirAll(c.CARoot, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", c.CARoot, err)
	}
	cmd := exec.CommandContext(ctx, c.BinPath, "-install")
	cmd.Env = append(os.Environ(), "CAROOT="+c.CARoot, "TRUST_STORES=none")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert mint: %w", err)
	}
	if _, err := os.Stat(caCrt); err != nil {
		return fmt.Errorf("mkcert did not create %s: %w", caCrt, err)
	}
	return nil
}

// Install runs `mkcert -install` to add the CA to the host trust store.
// Requires sudo; mkcert prompts on stdin.
func (c *CA) Install(ctx context.Context) error {
	if err := c.assertBinaryPresent(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.BinPath, "-install")
	cmd.Env = append(os.Environ(), "CAROOT="+c.CARoot)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert -install: %w", err)
	}
	return nil
}

// Uninstall runs `mkcert -uninstall`.
func (c *CA) Uninstall(ctx context.Context) error {
	if err := c.assertBinaryPresent(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.BinPath, "-uninstall")
	cmd.Env = append(os.Environ(), "CAROOT="+c.CARoot)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert -uninstall: %w", err)
	}
	return nil
}

func (c *CA) assertBinaryPresent() error {
	if _, err := os.Stat(c.BinPath); err != nil {
		return fmt.Errorf("mkcert binary missing at %s — download v1.4.4-linux-amd64: %w", c.BinPath, err)
	}
	return nil
}
