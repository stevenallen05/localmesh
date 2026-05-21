// Package envwriter writes the managed section of .env that compose
// interpolates onto container env + labels.
//
// The managed section sits between greppable delimiters. Content outside
// the delimiters round-trips verbatim, so a contributor can hand-edit
// non-managed pre-bootstrap vars (e.g. IMAGE_TAG) without losing them on
// re-render. Managed keys are emitted alphabetically for deterministic
// byte-identical output on repeat runs.
//
// Mint-once-preserve: LOCALMESH_OIDC_CLIENT_SECRET (hex) is minted on
// first run and preserved across re-renders by reading the existing
// .env (any section — managed or unmanaged) before regeneration.
//
// TODO: needs_prod_decisions .env move under .localmesh once config-loading tools improve
package envwriter

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stevenallen05/localmesh/internal/manifest"
)

const (
	managedStart = "# >>> secrets-gen managed — do not edit; regenerated on `make certs`"
	managedEnd   = "# <<< secrets-gen managed"
)

// WriteManaged regenerates .env's managed section under repoRoot. Content
// outside the managed delimiters is preserved verbatim; mint-once-preserve
// secrets are reused when present in the existing .env.
func WriteManaged(repoRoot string, proj *manifest.Project, plugins []*manifest.Plugin) error {
	envPath := filepath.Join(repoRoot, ".env")
	preamble, existing, postamble, err := readExisting(envPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", envPath, err)
	}
	managed, err := buildManaged(proj, plugins, existing)
	if err != nil {
		return fmt.Errorf("build managed section: %w", err)
	}
	out := assemble(preamble, managed, postamble)
	if err := os.WriteFile(envPath, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", envPath, err)
	}
	return nil
}

// buildManaged returns the alphabetically-sorted KEY=VALUE pairs for the
// managed section. `existing` is the set of KEY=VALUE pairs already
// present in .env (any section) — consulted to preserve mint-once secrets.
func buildManaged(proj *manifest.Project, plugins []*manifest.Plugin, existing map[string]string) (map[string]string, error) {
	out := map[string]string{}

	// Project-level identity scalars. Empty fields drop out so a project
	// without (e.g.) external_domain doesn't emit `EXTERNAL_DOMAIN=`.
	if proj.Name != "" {
		out["PROJECT_NAME"] = proj.Name
	}
	if proj.Namespace != "" {
		out["PROJECT_NAMESPACE"] = proj.Namespace
	}
	if proj.TechLead != "" {
		out["TECH_LEAD_EMAIL"] = proj.TechLead
	}
	if proj.ExternalDomain != "" {
		out["EXTERNAL_DOMAIN"] = proj.ExternalDomain
	}
	if proj.LocalDomain != "" {
		out["LOCAL_DOMAIN"] = proj.LocalDomain
	}

	// Per-plugin identity tuple → <PLUGIN>_MODULE_NAME / <PLUGIN>_OWNED_BY.
	for _, p := range plugins {
		prefix := upcaseSnake(p.Name)
		out[prefix+"_MODULE_NAME"] = p.Identity.ModuleName
		out[prefix+"_OWNED_BY"] = p.Identity.OwnedBy
	}

	// Per-service flags. Project-level first, then plugins in load order;
	// final map output is alphabetical so insertion order doesn't matter.
	addService := func(s manifest.Service) {
		prefix := upcaseSnake(s.Container)
		out[prefix+"_PORT"] = fmt.Sprintf("%d", s.Port)
		out[prefix+"_EXPOSE_VIA_INGRESS"] = boolStr(s.ExposeViaIngress)
		if s.Ingress {
			out[prefix+"_INGRESS"] = "true"
		}
	}
	for _, s := range proj.Services {
		addService(s)
	}
	for _, p := range plugins {
		for _, s := range p.Services {
			addService(s)
		}
	}

	// Mint-once-preserve secrets. Existing value wins; otherwise mint fresh.
	oidc, err := preserveOrMint(existing, "LOCALMESH_OIDC_CLIENT_SECRET", func() (string, error) { return mintHex(16) })
	if err != nil {
		return nil, err
	}
	out["LOCALMESH_OIDC_CLIENT_SECRET"] = oidc

	return out, nil
}

// readExisting splits .env into (preamble, managedKVs, postamble). When
// the file has no managed section, managedKVs is empty and the whole file
// is the preamble. When the file doesn't exist, all three are empty.
//
// managedKVs combines managed-section pairs AND unmanaged KEY=VALUE pairs
// from preamble/postamble so mint-once-preserve secrets carry over even
// if a contributor hand-pasted one outside the markers.
func readExisting(envPath string) (preamble string, kvs map[string]string, postamble string, err error) {
	kvs = map[string]string{}
	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", kvs, "", nil
		}
		return "", nil, "", err
	}
	text := string(data)

	preamble, rest, found := strings.Cut(text, managedStart)
	if !found {
		// No managed section — everything is preamble; kvs harvested for mint preservation.
		harvestKVs(text, kvs)
		return text, kvs, "", nil
	}
	managedBody, postamble, found := strings.Cut(rest, managedEnd)
	if !found {
		// Malformed: start marker without end marker. Treat the rest of
		// the file as managed body so we don't double-emit on rewrite.
		harvestKVs(rest, kvs)
		harvestKVs(preamble, kvs)
		return preamble, kvs, "", nil
	}
	harvestKVs(managedBody, kvs)
	harvestKVs(preamble, kvs)
	harvestKVs(postamble, kvs)
	return preamble, kvs, postamble, nil
}

func harvestKVs(s string, out map[string]string) {
	scanner := bufio.NewScanner(strings.NewReader(s))
	// .env values are short by construction; the default 64KB buffer is fine.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		// First write wins — preamble harvested before postamble, so
		// a contributor's hand-edit before the managed section trumps
		// a stale managed-section value when both are present.
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
}

// assemble rebuilds the .env body: preamble + a single blank line +
// managed block + postamble. Handles empty cases without double newlines.
func assemble(preamble string, managed map[string]string, postamble string) string {
	var b strings.Builder
	pre := strings.TrimRight(preamble, "\n")
	if pre != "" {
		b.WriteString(pre)
		b.WriteString("\n\n")
	}
	b.WriteString(managedStart)
	b.WriteByte('\n')
	keys := make([]string, 0, len(managed))
	for k := range managed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(managed[k])
		b.WriteByte('\n')
	}
	b.WriteString(managedEnd)
	b.WriteByte('\n')
	post := strings.TrimLeft(postamble, "\n")
	if post != "" {
		b.WriteString(post)
		if !strings.HasSuffix(post, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func preserveOrMint(existing map[string]string, key string, mint func() (string, error)) (string, error) {
	if v, ok := existing[key]; ok && v != "" {
		return v, nil
	}
	return mint()
}

func mintHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("mintHex: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func upcaseSnake(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// LoadEnv reads the given .env file and returns every KEY=VALUE pair
// it finds — across both the managed and unmanaged sections, since
// hand-edited values outside the markers are equally valid sources.
// Empty file or missing file returns an empty map, no error.
//
// Used by callers that need to substitute .env values into other
// artifacts (e.g. dexseed's template expansion of dex.yaml.sample).
func LoadEnv(envPath string) (map[string]string, error) {
	out := map[string]string{}
	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("read %s: %w", envPath, err)
	}
	harvestKVs(string(data), out)
	return out, nil
}
