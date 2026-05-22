package dexseed

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSeed_skipsWhenTargetExists guards the dev-owned-edit contract:
// once `localmesh/dex.yaml` exists, re-runs of `localmesh setup` must
// not clobber it (the dev edits this file to add/remove staticPasswords).
func TestSeed_skipsWhenTargetExists(t *testing.T) {
	dir := t.TempDir()
	samplePath := filepath.Join(dir, "dex.yaml.sample")
	if err := os.WriteFile(samplePath, []byte("issuer: replaced-content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dir, "dex.yaml")
	original := "issuer: dev-edited\n"
	if err := os.WriteFile(targetPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	skipped, err := Seed(samplePath, targetPath, map[string]string{})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if !skipped {
		t.Error("Seed returned skipped=false; want true when target exists")
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("target was clobbered: got %q, want %q", string(got), original)
	}
}

// TestSeed_expandsThreeVarsAndWrites pins the three env-var placeholders
// the sample contains today ($PROJECT_NAME, $LOCAL_DOMAIN,
// $LOCALMESH_OIDC_CLIENT_SECRET) so that dev.yaml.sample changes that
// add or rename a placeholder break this test loudly.
func TestSeed_expandsThreeVarsAndWrites(t *testing.T) {
	dir := t.TempDir()
	sample := "issuer: http://dex:5556\n" +
		"client: $LOCALMESH_OIDC_CLIENT_SECRET\n" +
		"redirect: https://www.$PROJECT_NAME.$LOCAL_DOMAIN/oauth2/callback\n"
	samplePath := filepath.Join(dir, "dex.yaml.sample")
	if err := os.WriteFile(samplePath, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dir, "dex.yaml")
	env := map[string]string{
		"PROJECT_NAME":                 "metrics-collector",
		"LOCAL_DOMAIN":                 "lvh.me",
		"LOCALMESH_OIDC_CLIENT_SECRET": "cafebabe",
	}
	skipped, err := Seed(samplePath, targetPath, env)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if skipped {
		t.Error("Seed returned skipped=true on first-run; want false")
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "issuer: http://dex:5556\n" +
		"client: cafebabe\n" +
		"redirect: https://www.metrics-collector.lvh.me/oauth2/callback\n"
	if string(got) != want {
		t.Errorf("Seed wrote:\n%s\nwant:\n%s", string(got), want)
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("file mode = %v, want 0644", info.Mode().Perm())
	}
}

// TestSeed_preservesBcryptHashes guards against the most subtle blocker
// in this work: dex.yaml.sample's staticPasswords entries contain
// bcrypt hashes shaped like '$2y$10$N.b.upq/...'. Go's os.Expand
// consumes $2, $10, and $N as variable references — producing garbled
// hashes that compile but fail every login attempt. This test pins the
// hash through Seed unchanged.
func TestSeed_preservesBcryptHashes(t *testing.T) {
	dir := t.TempDir()
	const bcryptLine = "    hash:     '$2y$10$N.b.upq/.fOOUGbkQ2uy8u7mCgjQJA6hUnedHSVy4zcL4c23Ug7R2'\n"
	samplePath := filepath.Join(dir, "dex.yaml.sample")
	if err := os.WriteFile(samplePath, []byte(bcryptLine), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dir, "dex.yaml")
	if _, err := Seed(samplePath, targetPath, map[string]string{}); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != bcryptLine {
		t.Errorf("bcrypt hash was mangled:\n got: %q\nwant: %q", string(got), bcryptLine)
	}
}
