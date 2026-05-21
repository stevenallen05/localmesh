// Package dexseed copies dex.yaml.sample to localmesh/dex.yaml on first
// run, expanding the three named env-var placeholders the sample
// contains. Subsequent runs skip silently so the dev's hand-edits
// (staticPasswords entries, etc.) are preserved.
//
// IMPORTANT: we expand ONLY the three known keys via strings.NewReplacer,
// NOT os.Expand. The sample contains bcrypt password hashes like
// '$2y$10$N.b.upq/...'; Go's os.Expand consumes shell-special tokens
// ($2, $10) and identifier-shaped runs ($N), corrupting the hash to
// 'y0.b.upq/...' and silently breaking every staticPassword login. The
// Makefile's old python expandvars target preserved these by luck of
// Python's stricter identifier rule; we don't get to rely on that here.
package dexseed

import (
	"fmt"
	"os"
	"strings"
)

// Seed reads the sample at samplePath, substitutes the three known
// placeholders ($PROJECT_NAME, $LOCAL_DOMAIN,
// $LOCALMESH_OIDC_CLIENT_SECRET) with values from env, and writes the
// result to targetPath at mode 0644. If targetPath already exists,
// Seed returns skipped=true and writes nothing. Missing samplePath
// is an error.
//
// Unrecognised $-prefixed tokens (bcrypt hashes, anything else) round-
// trip verbatim — only the literal three keys above are matched.
func Seed(samplePath, targetPath string, env map[string]string) (skipped bool, err error) {
	if _, err := os.Stat(targetPath); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat %s: %w", targetPath, err)
	}
	sample, err := os.ReadFile(samplePath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", samplePath, err)
	}
	r := strings.NewReplacer(
		"$PROJECT_NAME", env["PROJECT_NAME"],
		"$LOCAL_DOMAIN", env["LOCAL_DOMAIN"],
		"$LOCALMESH_OIDC_CLIENT_SECRET", env["LOCALMESH_OIDC_CLIENT_SECRET"],
	)
	expanded := r.Replace(string(sample))
	if err := os.WriteFile(targetPath, []byte(expanded), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", targetPath, err)
	}
	return false, nil
}
