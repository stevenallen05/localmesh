package ca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCA_MintMissingBinary(t *testing.T) {
	c := &CA{
		BinPath: filepath.Join(t.TempDir(), "no-mkcert"),
		CARoot:  filepath.Join(t.TempDir(), "ca"),
	}
	err := c.Mint(context.Background(), false)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want path-not-exist", err)
	}
}

func TestCA_MintAlreadyExistsWithoutForce(t *testing.T) {
	caRoot := t.TempDir()
	// Pretend a CA exists
	if err := os.WriteFile(filepath.Join(caRoot, "rootCA.pem"), []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &CA{BinPath: "/usr/bin/true", CARoot: caRoot} // /usr/bin/true won't be called
	err := c.Mint(context.Background(), false)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("got %v, want ErrAlreadyExists", err)
	}
}
