# Plugin Config Inversion Implementation Plan

> **For agentic workers:** REQUIRED: Use `superpowers:subagent-driven-development` (if subagents available) or `superpowers:executing-plans` to implement this plan. Steps use checkbox (`- [ ]`) syntax. TDD discipline per `@superpowers:test-driven-development`. Stealth-mode commits only (per `CLAUDE.md`): short imperative subject, no AI attribution, no `Co-Authored-By` trailer.

**Goal:** Implement Phase 1 of the plugin config inversion. Manifest extensions (`config_vars` + `exports`), `deploy.toml` loader + validator, envwriter rendering pipeline, the redis worked example, live acceptance via `localmesh build` + `docker compose up`.

**Architecture:** Two packages change. `manifest` gains new types (`ConfigVar`, `Export`, `Deploy`, `Instance`) plus a parse-only `LoadDeploy` and a cross-validator `ValidateDeploy`. `LoadAll` returns `(*Project, []*Plugin, *Deploy, error)` and calls `ValidateDeploy` against the flattened plugin list. `envwriter.WriteManaged` takes the new `*Deploy` and runs a `resolveAndEmitInstances` step inside `buildManaged` (placed after the existing mint-once-preserve block, before `return out, nil`) that walks each plugin's `[[config_vars]]` and `[[exports]]`, renders templates with `text/template`, and emits resolved env vars into the managed section.

**Tech Stack:** Go 1.24 (workspace via `go.work`), `BurntSushi/toml` for parsing, `text/template` with `Option("missingkey=error")` (no Sprig), table-driven tests using `t.Fatal(err)` style (matches existing `manifest_test.go` / `envwriter_test.go`; testify is in go.sum but not used by these files), live acceptance via `localmesh build` + `docker compose up`.

**Reference:**
- Spec: `docs/superpowers/specs/2026-05-22-plugin-config-inversion-design.md` (gitignored; never committed).
- Cleanup tracker: `docs/CLEANUP.md` (tracked; committed in Chunk 4).
- Project conventions: `CLAUDE.md`, `docs/engineering/rules/plugin-conventions.md`, `docs/engineering/rules/golang-basics.md`.

**Project rule reminders:**
- Test changes and code changes never share a commit. Code commits require a green tree (`go test ./localmesh_src/...` passes). Test commits may be red intentionally during TDD.
- Stealth-mode commit messages. Short imperative subject. No AI attribution.
- `gopls` is mandatory for Go work per `CLAUDE.md`.

---

## Pre-existing worktree state

The brainstorming + spec passes left these moving-target files in the worktree (uncommitted). They are reference shapes during Chunks 0–3; Chunk 4 commits them as the worked example.

- `localmesh/service_catalog/redis/{plugin.toml,docker-compose.yml,README.md}` — untracked. Note: `plugin.toml` currently has `value = false` (bool) on the `noevict` config_var; spec says `ConfigVar.Value` is `string`. Task 4.2 corrects to `value = "false"`.
- `deploy.toml` (repo root) — untracked. Currently has **three** `[[plugins.redis]]` blocks (cache, async_handler, a `localmesh add`-style sketch). Phase 1's cap rejects multi-instance. Task 4.3 trims to one (the `cache` instance) and re-files the other two as comments documenting future shape.
- `docs/CLEANUP.md` — untracked.
- `project.toml` — modified (`plugins = ["redis"]`).
- `docker-compose.yml` — modified (www reads `${CACHE_URL}`).
- `.env` (gitignored) — sketches the output shape; CLI regenerates it.
- Deletions of `localmesh/service_catalog/{auth,base,postgres16,security}/` + `localmesh/bundled.compose.yaml` + `localmesh/developer_reference.md`.

Spec lives at `docs/superpowers/specs/2026-05-22-plugin-config-inversion-design.md`. Plan lives at `docs/superpowers/plans/2026-05-22-plugin-config-inversion.md`. Both gitignored.

## Known consequence of the catalog deletion (scoped for this plan)

Deleting `localmesh/service_catalog/auth/` removes the file `setup.go:100-102` reads via `dexseed.Seed` (`auth/dex.yaml.sample`). `make setup` will therefore fail at the dexseed step. This plan **does not** rip out the dexseed step or the OIDC-secret mint — both are dead-code-after-deletion that deserve their own focused cleanup. Live acceptance in Task 4.8 uses `localmesh build` directly (which does **not** call `dexseed`), so the inversion machinery can be exercised end-to-end without touching auth coupling. Task 4.9 adds a `docs/CLEANUP.md` entry for the dexseed removal.

## File structure

**Modify (Go):**

| Path | Responsibility |
|---|---|
| `localmesh_src/internal/manifest/manifest.go` | Add `ConfigVar`, `Export` types and `Plugin.ConfigVars` / `Plugin.Exports`. Add `Deploy`, `Instance` types. New `LoadDeploy(path)` (parse-only) and `ValidateDeploy(d, plugins)` (cross-validate). Extend `LoadAll` to return `(*Project, []*Plugin, *Deploy, error)`. New validators: `validateConfigVars`, `validateExports`, `validateConfigExportNameCollision`. |
| `localmesh_src/internal/manifest/manifest_test.go` | New tests per spec §10. Existing tests at lines 124, 185, 205, 224, 238 call `LoadAll(...)` with the old 3-return signature; update each to `_, _, _, err := LoadAll(...)` (or to receive the new `*Deploy`). |
| `localmesh_src/internal/envwriter/envwriter.go` | New types: `exportCtx`, `serviceLite`, `projectLite`, `pluginLite`. New helpers: `coerceString`, `renderExportValue`, `renderExportEnvName`, `emitConfigVarsInternal`, `resolveAndEmitInstances`. `WriteManaged` signature gains `*manifest.Deploy`. New step inserted **after** the existing mint-once-preserve block and **before** `return out, nil`. Collision check names offending export. |
| `localmesh_src/internal/envwriter/envwriter_test.go` | New tests per spec §10. Existing 8 callers of `WriteManaged(...)` (find with `grep -n 'WriteManaged(' envwriter_test.go`) need a trailing `nil` argument for the new `*Deploy` parameter unless they're exercising the new behavior. |
| `localmesh_src/cmd/localmesh/build.go:26` | `LoadAll` returns 4 values now; pass `deploy` into `WriteManaged`. |
| `localmesh_src/cmd/localmesh/setup.go:56,82` | Same: receive `deploy` from `LoadAll`, pass into `WriteManaged`. |
| `localmesh_src/internal/render/render.go:22` | Third caller of `LoadAll` (not in CLI). Discard the new return: `proj, plugins, _, err := manifest.LoadAll(projectFile, catalogRoot)` if render doesn't use it. |
| `localmesh_src/testdata/sample_catalog/alpha/plugin.toml` | Add `[[config_vars]]` + `[[exports]]` so render-golden tests exercise the new emissions. Regenerate `testdata/expected.compose.yaml` if it shifts. |

**Add (worked example + docs):** see Chunk 4. Already in the worktree as moving targets.

**Modify (docs):**

| Path | Change |
|---|---|
| `docs/engineering/rules/plugin-conventions.md` | §4 addendum: `config_vars` + `exports` schema, internal `<PLUGIN>_<CONFIG>` naming rule, name-collision rule, `.Service` singular for Phase 1 multi-service plugins. Verify section numbering still reads "§4" after recent edits. |
| `docs/stakeholder/DESIGN_DECISIONS.md` | Row 56 (Plugin manifest); Open section gains Phase 2 / Phase 3 + dexseed-cleanup entries. |
| `docs/CLEANUP.md` | Already in worktree from spec pass. Add the dexseed/OIDC-mint entry in Task 4.9. |

**Delete (catalog reshape):** the deletions are already staged in the worktree; Task 4.1 commits them.

---

## Chunk 0: Set baseline

Confirm Go toolchain is present and the test suite is green before TDD begins.

### Task 0.1: Verify Go toolchain

- [ ] **Step 1:** Run `go version`. Expected: `go version go1.24.x linux/amd64` or similar. If absent, stop and surface — do not proceed (`CLAUDE.md` rule).
- [ ] **Step 2:** Run `gopls version`. Expected: a recent version. If missing, install/fix before proceeding.
- [ ] **Step 3:** Run `go test ./localmesh_src/...`. Expected: PASS. Worktree HEAD is `288fd49` — the suite passes at HEAD even with the staged catalog deletions, because tests use `testdata/sample_catalog/`, not the real catalog.

### Task 0.2: Confirm worktree mutations don't break tests

- [ ] **Step 1:** `grep -rn 'localmesh/service_catalog' localmesh_src/internal/ | grep -v testdata`. Expected: zero results inside `internal/`. (One reference exists in `cmd/localmesh/setup.go:100` but it's in `cmd/`, not `internal/`, and is the dexseed coupling tracked in CLEANUP.)
- [ ] **Step 2:** Re-run `go test ./localmesh_src/...`. Expected: PASS. If any test FAILS, stop and surface — do not proceed.

---

## Chunk 1: Manifest types + `LoadPlugin` validation

Add `ConfigVar`, `Export` types to `Plugin`. Add validators called from `LoadPlugin`. Existing manifest_test.go style is `t.Fatal(err)` — match it.

### Task 1.1: ConfigVar + Export types parse off TOML

**Files:**
- Modify: `localmesh_src/internal/manifest/manifest.go`
- Test: `localmesh_src/internal/manifest/manifest_test.go`

- [ ] **Step 1 — failing test (config_vars):**

```go
func TestLoadPlugin_configVarsParse(t *testing.T) {
    root := t.TempDir()
    pluginDir := filepath.Join(root, "redis")
    if err := os.MkdirAll(pluginDir, 0o755); err != nil { t.Fatal(err) }
    body := `
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[config_vars]]
name        = "maxmemory"
description = "Memory cap"
value       = "256mb"
`
    // [[services]] intentionally omitted — manifest validation allows zero
    // services (validateServiceSchemes is a no-op on empty slice), and this
    // test is scoped to config_vars parsing.
    if err := os.WriteFile(filepath.Join(pluginDir, "plugin.toml"), []byte(body), 0o644); err != nil {
        t.Fatal(err)
    }

    p, err := LoadPlugin(root, "redis")
    if err != nil { t.Fatal(err) }
    if got, want := len(p.ConfigVars), 1; got != want {
        t.Fatalf("len(ConfigVars) = %d, want %d", got, want)
    }
    if got, want := p.ConfigVars[0].Name, "maxmemory"; got != want {
        t.Errorf("Name = %q, want %q", got, want)
    }
    if got, want := p.ConfigVars[0].Value, "256mb"; got != want {
        t.Errorf("Value = %q, want %q", got, want)
    }
}
```

- [ ] **Step 2 — failing test (exports):** same shape, plugin.toml has one `[[exports]]` block; assert `p.Exports[0].Name == "connection_url"`, `Template == "redis://host:6379"`, `Env == "REDIS_URL"`, `Required == true`.

- [ ] **Step 3:** `go test ./localmesh_src/internal/manifest/ -run 'TestLoadPlugin_(configVars|exports)Parse' -v`. Expected: FAIL — `ConfigVars`/`Exports` undefined on `Plugin`.

- [ ] **Step 4 — commit tests:**

```bash
git add localmesh_src/internal/manifest/manifest_test.go
git commit -m "test: cover plugin.toml config_vars + exports parse"
```

- [ ] **Step 5 — implement types:** in `manifest.go`, add:

```go
type ConfigVar struct {
    Name        string `toml:"name"`
    Description string `toml:"description"`
    Value       string `toml:"value"`
}

type Export struct {
    Name        string `toml:"name"`
    Description string `toml:"description"`
    Template    string `toml:"template"`
    Env         string `toml:"env"`
    Required    bool   `toml:"required"`
}
```

Add fields to `Plugin` (preserve existing field order, add new ones at the end):

```go
type Plugin struct {
    Name       string      `toml:"-"`
    Identity   Identity    `toml:"identity"`
    Plugins    []string    `toml:"plugins"`
    Services   []Service   `toml:"services"`
    ConfigVars []ConfigVar `toml:"config_vars"`
    Exports    []Export    `toml:"exports"`
}
```

- [ ] **Step 6:** Re-run `go test ./localmesh_src/internal/manifest/ -run 'TestLoadPlugin_(configVars|exports)Parse' -v`. Expected: PASS.
- [ ] **Step 7:** `go test ./localmesh_src/...`. Expected: PASS.
- [ ] **Step 8 — commit code:**

```bash
git add localmesh_src/internal/manifest/manifest.go
git commit -m "manifest: add ConfigVar + Export types to Plugin"
```

### Task 1.2: Validation rules for `config_vars`

Rules per spec §4 (config_vars): `name` empty/multiline, `description` empty/multiline, `value` empty, duplicate `name`.

- [ ] **Step 1 — failing tests** (table-driven `TestLoadPlugin_configVarsValidation` + `TestLoadPlugin_configVarDuplicateName`):

```go
func TestLoadPlugin_configVarsValidation(t *testing.T) {
    cases := []struct {
        name      string
        body      string // [[config_vars]] block content (the [identity] block is prepended)
        wantErr   string
    }{
        {"missingName", `[[config_vars]]
description = "x"
value       = "y"`, `config_vars[0].name required`},
        {"multilineName", "[[config_vars]]\nname        = \"a\\nb\"\ndescription = \"x\"\nvalue       = \"y\"", `config_vars[0].name must be single-line`},
        {"missingDescription", `[[config_vars]]
name  = "x"
value = "y"`, `config_vars[0].description required`},
        {"missingValue", `[[config_vars]]
name        = "x"
description = "y"`, `config_vars[0].value required`},
    }
    // ... for each case: write plugin.toml with identity + body, LoadPlugin, assert err contains wantErr
}
```

- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit tests:** `test: cover config_vars validation rules`.
- [ ] **Step 4 — implement `validateConfigVars`:**

```go
func validateConfigVars(path string, vars []ConfigVar) error {
    seen := map[string]int{}
    for i, v := range vars {
        if v.Name == "" {
            return fmt.Errorf("%s: config_vars[%d].name required", path, i)
        }
        if strings.Contains(v.Name, "\n") {
            return fmt.Errorf("%s: config_vars[%d].name must be single-line", path, i)
        }
        if v.Description == "" {
            return fmt.Errorf("%s: config_vars[%d].description required", path, i)
        }
        if strings.Contains(v.Description, "\n") {
            return fmt.Errorf("%s: config_vars[%d].description must be single-line", path, i)
        }
        if v.Value == "" {
            return fmt.Errorf("%s: config_vars[%d].value required", path, i)
        }
        if j, dup := seen[v.Name]; dup {
            return fmt.Errorf("%s: config_vars: duplicate name %q (entries %d and %d)", path, v.Name, j, i)
        }
        seen[v.Name] = i
    }
    return nil
}
```

Call from `LoadPlugin` (after the existing identity check).

- [ ] **Step 5:** Re-run; PASS. Full suite PASS.
- [ ] **Step 6 — commit code:** `manifest: validate config_vars at LoadPlugin`.

### Task 1.3: Validation rules for `exports`

Rules per spec §4 (exports): `name` empty/multiline, `template` empty, `env` empty, duplicate `name`. **`description` is not validated for exports** (spec §4 omits it; intentional asymmetry). Don't blindly mirror `validateConfigVars`.

- [ ] **Step 1 — failing tests** `TestLoadPlugin_exportsValidation` (table-driven) + `TestLoadPlugin_exportDuplicateName`. Cases: missingName, multilineName, missingTemplate, missingEnv, duplicate.
- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit tests:** `test: cover exports validation rules`.
- [ ] **Step 4 — implement `validateExports`** mirror of `validateConfigVars` minus the description check. Call from `LoadPlugin`.
- [ ] **Step 5:** Re-run; PASS. Full suite PASS.
- [ ] **Step 6 — commit code:** `manifest: validate exports at LoadPlugin`.

### Task 1.4: Name-collision validation across `config_vars` and `exports`

- [ ] **Step 1 — failing test:**

```go
func TestLoadPlugin_nameCollisionAcrossArrays(t *testing.T) {
    // plugin.toml with [[config_vars]] name="connection_url"
    // AND  [[exports]]      name="connection_url"
    // expect err contains: name collision between config_vars and exports: "connection_url"
}
```

- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit test:** `test: cover config_vars/exports name collision`.
- [ ] **Step 4 — implement `validateConfigExportNameCollision`:**

```go
func validateConfigExportNameCollision(path string, cvs []ConfigVar, exps []Export) error {
    names := map[string]bool{}
    for _, c := range cvs { names[c.Name] = true }
    for _, e := range exps {
        if names[e.Name] {
            return fmt.Errorf("%s: name collision between config_vars and exports: %q", path, e.Name)
        }
    }
    return nil
}
```

Call from `LoadPlugin` after the per-array validators.

- [ ] **Step 5:** Re-run; PASS. Full suite PASS.
- [ ] **Step 6 — commit code:** `manifest: reject config_vars/exports name collision`.

---

## Chunk 2: Deploy loader + `ValidateDeploy` + `LoadAll` extension

### Task 2.1: `Deploy` + `Instance` types + `LoadDeploy` parse

**Files:**
- Modify: `localmesh_src/internal/manifest/manifest.go`
- Test: `localmesh_src/internal/manifest/manifest_test.go`

- [ ] **Step 1 — failing tests:**

```go
func TestLoadDeploy_parses(t *testing.T) {
    path := filepath.Join(t.TempDir(), "deploy.toml")
    body := `
[plugins]
[[plugins.redis]]
service_name = "cache"
  [plugins.redis.config]
  maxmemory = "512mb"
  [plugins.redis.exports]
  connection_url = "CACHE_URL"
`
    if err := os.WriteFile(path, []byte(body), 0o644); err != nil { t.Fatal(err) }

    d, err := LoadDeploy(path)
    if err != nil { t.Fatal(err) }
    if got, want := len(d.Plugins["redis"]), 1; got != want {
        t.Fatalf("len(Plugins[redis]) = %d, want %d", got, want)
    }
    inst := d.Plugins["redis"][0]
    if got, want := inst.ServiceName, "cache"; got != want {
        t.Errorf("ServiceName = %q, want %q", got, want)
    }
    if got, want := inst.Config["maxmemory"], "512mb"; got != want {
        t.Errorf(`Config["maxmemory"] = %v, want %v`, got, want)
    }
    if got, want := inst.Exports["connection_url"], "CACHE_URL"; got != want {
        t.Errorf(`Exports["connection_url"] = %q, want %q`, got, want)
    }
}

func TestLoadDeploy_parsesMultipleInstancesPerSlug(t *testing.T) {
    // two [[plugins.redis]] entries (cache + analytics)
    // expect len(d.Plugins["redis"]) == 2 — LoadDeploy is purely structural;
    // the Phase 1 cap belongs in ValidateDeploy (Task 2.2). BurntSushi/toml
    // populates the slice in declaration order; confirm the order matches.
}

func TestLoadDeploy_absent(t *testing.T) {
    d, err := LoadDeploy(filepath.Join(t.TempDir(), "missing.toml"))
    if err != nil { t.Fatal(err) }
    if d == nil { t.Fatal("expected non-nil Deploy") }
    if len(d.Plugins) != 0 { t.Errorf("expected empty Plugins, got %v", d.Plugins) }
}

func TestLoadDeploy_configKeepsTomlScalar(t *testing.T) {
    // [plugins.redis.config] noevict = true (bool, not string)
    // Phase 1: LoadDeploy stores it as TOML's any; coercion to string
    // happens in envwriter (Chunk 3, Task 3.1).
    body := `
[plugins]
[[plugins.redis]]
  [plugins.redis.config]
  noevict = true
`
    // ... write, LoadDeploy
    inst := d.Plugins["redis"][0]
    // BurntSushi/toml decodes booleans into `any` as bool — assert that.
    if got, want := inst.Config["noevict"], true; got != want {
        t.Errorf(`Config["noevict"] = %v (%T), want %v (bool)`, got, got, want)
    }
}
```

- [ ] **Step 2:** Run; FAIL (`LoadDeploy`/`Deploy`/`Instance` undefined).
- [ ] **Step 3 — commit tests:** `test: cover LoadDeploy parse + multi-instance + scalar coercion`.
- [ ] **Step 4 — implement types + loader:**

```go
type Deploy struct {
    Plugins map[string][]Instance `toml:"plugins"`
}

type Instance struct {
    ServiceName string            `toml:"service_name"`
    Config      map[string]any    `toml:"config"`    // any TOML scalar; coerced to string at render time
    Exports     map[string]string `toml:"exports"`
}

func LoadDeploy(path string) (*Deploy, error) {
    d := &Deploy{Plugins: map[string][]Instance{}}
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return d, nil
        }
        return nil, fmt.Errorf("read %s: %w", path, err)
    }
    if _, err := toml.Decode(string(data), d); err != nil {
        return nil, fmt.Errorf("%s: %w", path, err)
    }
    if d.Plugins == nil {
        d.Plugins = map[string][]Instance{}
    }
    return d, nil
}
```

`BurntSushi/toml` decodes `[[plugins.foo]]` array-of-tables into a slice value at `Plugins["foo"]`. Single-element blocks become a one-element slice. Confirmed against the library's existing usage in `LoadPlugin`.

- [ ] **Step 5:** Re-run; PASS. Full suite PASS.
- [ ] **Step 6 — commit code:** `manifest: add LoadDeploy + Deploy/Instance types`.

### Task 2.2: `ValidateDeploy` against the flattened plugin list

- [ ] **Step 1 — failing tests** (table-driven `TestValidateDeploy`):

```go
func TestValidateDeploy(t *testing.T) {
    cases := []struct {
        name    string
        deploy  string // TOML body
        plugins []*Plugin
        wantErr string // "" for OK
    }{
        {
            name: "sourceNotSelected",
            deploy: `[plugins]
[[plugins.foo]]`,
            plugins: []*Plugin{{Name: "redis"}},
            wantErr: `plugins.foo: source not selected in project.toml`,
        },
        {
            name: "phase1RejectsSecondInstance",
            deploy: `[plugins]
[[plugins.redis]]
service_name = "a"
[[plugins.redis]]
service_name = "b"`,
            plugins: []*Plugin{{Name: "redis"}},
            wantErr: `Phase 1 accepts at most one instance per slug`,
        },
        {
            name: "duplicateServiceNameWithinSlug",
            // This case is structurally unreachable in Phase 1 because the
            // multi-instance cap fires first. The duplicate-detection branch
            // is Phase 2 scaffold; we keep the loop so the validator's shape
            // doesn't change between phases. Asserting the cap error wins
            // pins precedence ordering.
            deploy: `[plugins]
[[plugins.redis]]
service_name = "x"
[[plugins.redis]]
service_name = "x"`,
            plugins: []*Plugin{{Name: "redis"}},
            wantErr: `Phase 1 accepts at most one instance per slug`,
        },
        {
            name: "unknownConfigKey",
            deploy: `[plugins]
[[plugins.redis]]
[plugins.redis.config]
nope = "x"`,
            plugins: []*Plugin{{Name: "redis", ConfigVars: []ConfigVar{{Name: "maxmemory"}}}},
            wantErr: `plugins.redis.config.nope unknown`,
        },
        {
            name: "unknownExportKey",
            deploy: `[plugins]
[[plugins.redis]]
[plugins.redis.exports]
nope = "X"`,
            plugins: []*Plugin{{Name: "redis", Exports: []Export{{Name: "connection_url"}}}},
            wantErr: `plugins.redis.exports.nope unknown`,
        },
        {
            name: "happyPath",
            deploy: `[plugins]
[[plugins.redis]]
service_name = "cache"
[plugins.redis.config]
maxmemory = "512mb"
[plugins.redis.exports]
connection_url = "CACHE_URL"`,
            plugins: []*Plugin{{
                Name: "redis",
                ConfigVars: []ConfigVar{{Name: "maxmemory"}},
                Exports:    []Export{{Name: "connection_url"}},
            }},
            wantErr: "",
        },
    }
    // for each case: write deploy.toml to t.TempDir, LoadDeploy, call ValidateDeploy(d, tc.plugins),
    // assert wantErr presence/absence.
}
```

- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit tests:** `test: cover ValidateDeploy cross-validation`.
- [ ] **Step 4 — implement `ValidateDeploy`:**

```go
// ValidateDeploy cross-checks a parsed Deploy against the flattened plugin
// list returned by LoadAll. Phase 1 caps each slug at one instance; the
// duplicate-service-name branch is kept as Phase 2 scaffold (structurally
// unreachable today, by design).
func ValidateDeploy(d *Deploy, plugins []*Plugin) error {
    if d == nil { return nil }
    flat := map[string]*Plugin{}
    flatNames := make([]string, 0, len(plugins))
    for _, p := range plugins {
        flat[p.Name] = p
        flatNames = append(flatNames, p.Name)
    }
    sort.Strings(flatNames)
    // deterministic iteration on slug for stable error ordering
    slugs := make([]string, 0, len(d.Plugins))
    for s := range d.Plugins { slugs = append(slugs, s) }
    sort.Strings(slugs)
    for _, slug := range slugs {
        instances := d.Plugins[slug]
        p, ok := flat[slug]
        if !ok {
            return fmt.Errorf("deploy.toml: plugins.%s: source not selected in project.toml (flattened: %s)",
                slug, strings.Join(flatNames, ", "))
        }
        if len(instances) > 1 {
            return fmt.Errorf("deploy.toml: plugins.%s: Phase 1 accepts at most one instance per slug; multi-instance lands in Phase 2", slug)
        }
        cvNames := map[string]bool{}
        for _, cv := range p.ConfigVars { cvNames[cv.Name] = true }
        expNames := map[string]bool{}
        for _, e := range p.Exports { expNames[e.Name] = true }
        seenSvc := map[string]int{}
        for i, inst := range instances {
            if strings.Contains(inst.ServiceName, "\n") {
                return fmt.Errorf("deploy.toml: plugins.%s[%d].service_name must be single-line", slug, i)
            }
            if inst.ServiceName != "" {
                if j, dup := seenSvc[inst.ServiceName]; dup {
                    return fmt.Errorf("deploy.toml: plugins.%s: duplicate service_name %q (entries %d and %d)",
                        slug, inst.ServiceName, j, i)
                }
                seenSvc[inst.ServiceName] = i
            }
            for k := range inst.Config {
                if !cvNames[k] {
                    return fmt.Errorf("deploy.toml: plugins.%s.config.%s unknown (not in %s/plugin.toml [[config_vars]])", slug, k, slug)
                }
            }
            for k := range inst.Exports {
                if !expNames[k] {
                    return fmt.Errorf("deploy.toml: plugins.%s.exports.%s unknown", slug, k)
                }
            }
        }
    }
    return nil
}
```

- [ ] **Step 5:** Re-run; PASS. Full suite PASS.
- [ ] **Step 6 — commit code:** `manifest: add ValidateDeploy cross-validator`.

### Task 2.3: Extend `LoadAll` to return `*Deploy`

This is the disruptive step. **Three** non-test callers exist:

1. `localmesh_src/cmd/localmesh/build.go:26`
2. `localmesh_src/cmd/localmesh/setup.go:56`
3. `localmesh_src/internal/render/render.go:22`

Plus **five** existing test sites in `localmesh_src/internal/manifest/manifest_test.go` (lines 124, 185, 205, 224, 238 per `grep -n 'LoadAll(' manifest_test.go`).

- [ ] **Step 1 — failing tests** for the new behavior:

```go
func TestLoadAll_returnsDeploy(t *testing.T) {
    // arrange: temp project with project.toml + service_catalog/redis/plugin.toml
    //  + deploy.toml at the project root.
    _, _, deploy, err := LoadAll(projFile, catalogRoot)
    if err != nil { t.Fatal(err) }
    if deploy == nil { t.Fatal("expected non-nil Deploy") }
    if len(deploy.Plugins["redis"]) != 1 { t.Fatalf("len(redis) = %d, want 1", len(deploy.Plugins["redis"])) }
}

func TestLoadAll_absentDeployReturnsEmpty(t *testing.T) {
    _, _, deploy, err := LoadAll(projFile, catalogRoot)
    if err != nil { t.Fatal(err) }
    if deploy == nil { t.Fatal("expected non-nil Deploy") }
    if len(deploy.Plugins) != 0 { t.Errorf("expected empty Plugins, got %v", deploy.Plugins) }
}

func TestLoadAll_invalidDeployErrors(t *testing.T) {
    // deploy.toml references unselected source
    _, _, _, err := LoadAll(projFile, catalogRoot)
    if err == nil { t.Fatal("expected error") }
    if !strings.Contains(err.Error(), "source not selected") {
        t.Errorf("err = %v, want substring 'source not selected'", err)
    }
}
```

- [ ] **Step 2:** Run new tests. They fail to compile because the existing `manifest_test.go` callers (lines 124, 185, 205, 224, 238) destructure `_, plugins, err := LoadAll(...)` with three returns. Add a temporary `//nolint` placeholder if needed, OR write the new tests in a separate `_test.go` file and update the existing five sites in the same TDD step. Cleanest path: just update those five sites to `_, _, _, err := LoadAll(...)` (or named) as part of "Step 4 — implement."

- [ ] **Step 3 — commit tests:** `test: cover LoadAll returning *Deploy with validation`.

- [ ] **Step 4 — implement signature change in `LoadAll`:**

Update existing `LoadAll` (currently at `manifest.go:140-155`):

```go
func LoadAll(projectFile, catalogRoot string) (*Project, []*Plugin, *Deploy, error) {
    proj, err := LoadProject(projectFile)
    if err != nil {
        return nil, nil, nil, err
    }
    out := make([]*Plugin, 0, len(proj.Plugins))
    seen := map[string]bool{}
    for _, name := range proj.Plugins {
        out, err = flatten(catalogRoot, name, out, seen, []string{})
        if err != nil {
            return nil, nil, nil, err
        }
    }
    // deploy.toml lives next to project.toml at the project root.
    deployPath := filepath.Join(filepath.Dir(projectFile), "deploy.toml")
    deploy, err := LoadDeploy(deployPath)
    if err != nil {
        return nil, nil, nil, err
    }
    if err := ValidateDeploy(deploy, out); err != nil {
        return nil, nil, nil, err
    }
    return proj, out, deploy, nil
}
```

(The existing `flatten()` helper at `manifest.go:158` remains as-is. No new `flattenPlugins` helper.)

- [ ] **Step 5 — update non-test callers:**

```bash
grep -n 'manifest\.LoadAll(' localmesh_src/cmd/localmesh/build.go \
                              localmesh_src/cmd/localmesh/setup.go \
                              localmesh_src/internal/render/render.go
```

Expected three hits. For `build.go:26` and `setup.go:56`, update to:

```go
proj, plugins, deploy, err := manifest.LoadAll(projectFile, catalogRoot)
```

…and hold onto `deploy` until Chunk 3 wires it into `WriteManaged`. Until then, mark with `_ = deploy` to satisfy the unused-variable check.

For `render.go:22`, the render package doesn't need the deploy data, so discard:

```go
proj, plugins, _, err := manifest.LoadAll(projectFile, catalogRoot)
```

- [ ] **Step 6 — update existing test callers in `manifest_test.go`:**

```bash
grep -n 'LoadAll(' localmesh_src/internal/manifest/manifest_test.go
```

Expected five hits (lines 124, 185, 205, 224, 238 against current HEAD; line numbers may shift after the new tests are added). Each is currently `proj, plugins, err := LoadAll(...)` or `_, plugins, err := ...` etc. Update each to receive the fourth return as `_` (or name it `deploy` if the test cares).

- [ ] **Step 7:** Run full suite + build CLI:

```bash
go test ./localmesh_src/...
go build ./localmesh_src/cmd/localmesh
```

Expected: PASS / clean.

- [ ] **Step 8 — commit code:** `manifest: extend LoadAll to return *Deploy`.

---

## Chunk 3: Envwriter rendering pipeline

The new step lives inside `envwriter.buildManaged` **after** the existing mint-once-preserve block (`LOCALMESH_OIDC_CLIENT_SECRET`) and **before** `return out, nil`. It consumes the `*Deploy` threaded through `WriteManaged`.

### Task 3.1: Template-context types + `coerceString` helper

**Files:**
- Modify: `localmesh_src/internal/envwriter/envwriter.go`
- Test: `localmesh_src/internal/envwriter/envwriter_test.go`

- [ ] **Step 1 — failing test** (test file ends in `_test.go` and declares `package envwriter`, same as existing tests — so calls the unexported `coerceString` directly):

```go
func TestCoerceString(t *testing.T) {
    cases := []struct{
        name string
        in   any
        want string
    }{
        {"string", "hi", "hi"},
        {"bool true", true, "true"},
        {"bool false", false, "false"},
        {"int", 42, "42"},
        {"int64", int64(7), "7"},
        {"float", 3.14, "3.14"},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if got := coerceString(tc.in); got != tc.want {
                t.Errorf("coerceString(%v) = %q, want %q", tc.in, got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit test:** `test: cover any→string coercion`.
- [ ] **Step 4 — implement** in `envwriter.go`:

```go
func coerceString(v any) string {
    switch x := v.(type) {
    case string:  return x
    case bool:    if x { return "true" }; return "false"
    case int:     return strconv.Itoa(x)
    case int64:   return strconv.FormatInt(x, 10)
    case float64: return strconv.FormatFloat(x, 'g', -1, 64)
    default:      return fmt.Sprint(x)
    }
}
```

Also add the context types (unexported):

```go
type exportCtx struct {
    Project     projectLite
    Plugin      pluginLite
    ServiceName string
    Service     serviceLite
    ConfigVars  map[string]string
}

type projectLite struct {
    Name, Namespace, LocalDomain, ExternalDomain, TechLead string
}

type pluginLite struct {
    Name, ModuleName, OwnedBy string
}

type serviceLite struct {
    Container        string
    Port             int
    ExposeViaIngress bool
}
```

- [ ] **Step 5:** Re-run; PASS. Full suite PASS.
- [ ] **Step 6 — commit code:** `envwriter: add exportCtx types + coerceString helper`.

### Task 3.2: `renderExportEnvName` + `renderExportValue` helpers

- [ ] **Step 1 — failing tests:**

```go
func TestRenderExportEnvName_default(t *testing.T) {
    ctx := exportCtx{ServiceName: "cache"}
    got, err := renderExportEnvName("{{ .ServiceName | upper }}_URL", ctx)
    if err != nil { t.Fatal(err) }
    if got != "CACHE_URL" { t.Errorf("got %q, want CACHE_URL", got) }
}

func TestRenderExportEnvName_overrideLiteral(t *testing.T) {
    ctx := exportCtx{ServiceName: "cache"}
    got, err := renderExportEnvName("CACHE_URL", ctx)
    if err != nil { t.Fatal(err) }
    if got != "CACHE_URL" { t.Errorf("got %q, want CACHE_URL", got) }
}

func TestRenderExportEnvName_overrideTemplated(t *testing.T) {
    ctx := exportCtx{ServiceName: "cache", Project: projectLite{Name: "demo"}}
    got, err := renderExportEnvName("{{ .Project.Name | upper }}_URL", ctx)
    if err != nil { t.Fatal(err) }
    if got != "DEMO_URL" { t.Errorf("got %q, want DEMO_URL", got) }
}

func TestRenderExportValue_simple(t *testing.T) {
    ctx := exportCtx{Service: serviceLite{Container: "redis", Port: 6379}}
    got, err := renderExportValue("redis://{{ .Service.Container }}:{{ .Service.Port }}", ctx)
    if err != nil { t.Fatal(err) }
    if got != "redis://redis:6379" { t.Errorf("got %q", got) }
}

func TestRenderExportValue_templateError(t *testing.T) {
    _, err := renderExportValue("{{ .Bogus }}", exportCtx{})
    if err == nil { t.Fatal("expected error") }
}
```

- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit tests:** `test: cover export template rendering`.
- [ ] **Step 4 — implement** in `envwriter.go`:

```go
func renderExportEnvName(tmpl string, ctx exportCtx) (string, error) {
    return renderTmpl("env", tmpl, ctx)
}

func renderExportValue(tmpl string, ctx exportCtx) (string, error) {
    return renderTmpl("template", tmpl, ctx)
}

func renderTmpl(label, body string, ctx exportCtx) (string, error) {
    t, err := template.New(label).Option("missingkey=error").
        Funcs(template.FuncMap{"upper": strings.ToUpper}).
        Parse(body)
    if err != nil { return "", fmt.Errorf("%s parse: %w", label, err) }
    var buf bytes.Buffer
    if err := t.Execute(&buf, ctx); err != nil {
        return "", fmt.Errorf("%s execute: %w", label, err)
    }
    return buf.String(), nil
}
```

`upper` is the one explicit FuncMap entry. No Sprig per spec §6.

- [ ] **Step 5:** Re-run; PASS.
- [ ] **Step 6 — commit code:** `envwriter: add export template rendering helpers`.

### Task 3.3: `resolveAndEmitInstances` — integrate into `buildManaged`

`WriteManaged` signature gains `*manifest.Deploy`. The new step lives **after** the mint-once-preserve block and **before** `return out, nil` (the function returns the same `map[string]string` named `out` that everything else writes to).

**Eight** existing callers in `envwriter_test.go` need a `nil` trailing arg (verify with `grep -n 'WriteManaged(' envwriter_test.go`). Per the project's commit-hygiene rule, that change goes in its own commit (Step 8).

- [ ] **Step 1 — failing integration tests** (the spec §10 envwriter table; each test builds a small fake project + plugin + deploy in-memory, calls `WriteManaged`, asserts on `.env` content). Examples:

```go
func TestWriteManaged_exportEmitsDefaultEnvName(t *testing.T) {
    // arrange: project plugins=["redis"], one redis plugin with one export
    //   env="{{ .ServiceName | upper }}_URL", template="redis://{{ .Service.Container }}:{{ .Service.Port }}"
    //   deploy.toml absent (deploy=&Deploy{}).
    // act: WriteManaged
    // assert: read repoRoot/.env; managed section contains REDIS_URL=redis://redis:6379
}
```

Other cases per spec §10: `deployOverridesEnvName`, `overrideEnvNameIsRendered`, `deployOverridesConfigValue`, `configValueCoercion`, `requiredExportMissing`, `exportTemplateError`, `exportCollision`, `absentDeployUsesDefaults`.

- [ ] **Step 2:** Run; FAIL.
- [ ] **Step 3 — commit tests:** `test: cover envwriter resolve + emit pipeline`.
- [ ] **Step 4 — extend `WriteManaged` signature:**

```go
func WriteManaged(repoRoot string, proj *manifest.Project, plugins []*manifest.Plugin, deploy *manifest.Deploy) error {
    ...
}
```

- [ ] **Step 5 — implement `resolveAndEmitInstances`** inside `buildManaged`, placed **after** the OIDC mint-once-preserve assignment and **before** `return out, nil`. Operates on the same `out` map:

```go
if deploy == nil {
    deploy = &manifest.Deploy{Plugins: map[string][]manifest.Instance{}}
}
for _, p := range plugins {
    instances := deploy.Plugins[p.Name]
    if len(instances) == 0 {
        instances = []manifest.Instance{{ServiceName: p.Name}}
    }
    for _, inst := range instances {
        config := map[string]string{}
        for _, cv := range p.ConfigVars { config[cv.Name] = cv.Value }
        for k, raw := range inst.Config { config[k] = coerceString(raw) }

        serviceName := inst.ServiceName
        if serviceName == "" { serviceName = p.Name }
        var svc serviceLite
        if len(p.Services) > 0 {
            s := p.Services[0]
            svc = serviceLite{Container: s.Container, Port: s.Port, ExposeViaIngress: s.ExposeViaIngress}
        }
        ctx := exportCtx{
            Project: projectLite{
                Name: proj.Name, Namespace: proj.Namespace,
                LocalDomain: proj.LocalDomain, ExternalDomain: proj.ExternalDomain,
                TechLead: proj.TechLead,
            },
            Plugin:      pluginLite{Name: p.Name, ModuleName: p.Identity.ModuleName, OwnedBy: p.Identity.OwnedBy},
            ServiceName: serviceName,
            Service:     svc,
            ConfigVars:  config,
        }

        for _, e := range p.Exports {
            envTmpl := e.Env
            if override, ok := inst.Exports[e.Name]; ok {
                envTmpl = override
            }
            envName, err := renderExportEnvName(envTmpl, ctx)
            if err != nil {
                return fmt.Errorf("plugin %s export %s: %w", p.Name, e.Name, err)
            }
            if envName == "" && e.Required {
                return fmt.Errorf("plugin %s export %s: required but rendered empty destination", p.Name, e.Name)
            }
            value, err := renderExportValue(e.Template, ctx)
            if err != nil {
                return fmt.Errorf("plugin %s export %s: %w", p.Name, e.Name, err)
            }
            if existing, dup := out[envName]; dup {
                return fmt.Errorf("env var name collision: %s declared twice (existing=%q, new from plugin %s export %s=%q)",
                    envName, existing, p.Name, e.Name, value)
            }
            out[envName] = value
        }

        prefix := upcaseSnake(p.Name)
        for name, value := range config {
            key := prefix + "_" + upcaseSnake(name)
            if existing, dup := out[key]; dup {
                return fmt.Errorf("env var name collision: %s declared twice (existing=%q, new=%q)", key, existing, value)
            }
            out[key] = value
        }
    }
}
```

(`upcaseSnake` already exists in `envwriter.go`. Reuse.)

- [ ] **Step 6:** Re-run integration tests; PASS. Full envwriter package; PASS.
- [ ] **Step 7 — commit code:** `envwriter: resolve config_vars + exports per instance`.

- [ ] **Step 8 — update existing test callers of WriteManaged.** `grep -n 'WriteManaged(' localmesh_src/internal/envwriter/envwriter_test.go`. Expected eight hits (current HEAD count; may shift after Step 3 added new tests). For each, append a trailing `nil` for the new `*Deploy` parameter unless the test is one of the new ones from Step 1 (which already pass a real Deploy). Single test-file edit. Commit:

```bash
git add localmesh_src/internal/envwriter/envwriter_test.go
git commit -m "test: update WriteManaged callers for *Deploy arg"
```

### Task 3.4: Update CLI callers to pass `*Deploy`

- [ ] **Step 1:** Update `localmesh_src/cmd/localmesh/build.go:32`:

```go
if err := envwriter.WriteManaged(repoRoot, proj, plugins, deploy); err != nil { ... }
```

(`deploy` came from `LoadAll` in Chunk 2 Task 2.3.)

- [ ] **Step 2:** Update `localmesh_src/cmd/localmesh/setup.go:82` the same way.
- [ ] **Step 3:** `go build ./localmesh_src/cmd/localmesh`. Expected: clean build.
- [ ] **Step 4:** `go test ./localmesh_src/...`. Expected: PASS.
- [ ] **Step 5 — commit:** `cli: pass *Deploy into WriteManaged`.

---

## Chunk 4: Worked example + docs + live acceptance

### Task 4.1: Commit the catalog deletions

- [ ] **Step 1:** `git status --short`. Confirm the deletions of `localmesh/service_catalog/{auth,base,postgres16,security}/` + `localmesh/bundled.compose.yaml` + `localmesh/developer_reference.md` are staged.
- [ ] **Step 2:** `go test ./localmesh_src/...`. Expected: PASS (tests use `testdata/sample_catalog/`).
- [ ] **Step 3 — commit deletions:**

```bash
# `-u` deliberately scopes to tracked-file updates so the untracked
# redis plugin (added in Task 4.2) stays out of this commit.
git add -u localmesh/
git commit -m "catalog: drop pre-inversion plugins"
```

### Task 4.2: Commit the redis plugin (worked example)

The worktree's `localmesh/service_catalog/redis/plugin.toml` currently has `value = false` (bool) on the `noevict` config_var. `ConfigVar.Value` is string-typed per spec §4. Fix before committing.

- [ ] **Step 1:** Edit `localmesh/service_catalog/redis/plugin.toml`. Change the `noevict` block's value:

```toml
[[config_vars]]
name        = "noevict"
description = "Noevict policy"
value       = "false"
```

- [ ] **Step 2:** Sanity-check the plugin loads:

```bash
go run ./localmesh_src/cmd/localmesh build 2>&1 | head -40
```

Expected: build runs without manifest-parse errors. (It may fail later at the multi-instance deploy.toml that's about to be fixed in Task 4.3 — that's expected at this stage.)

- [ ] **Step 3 — commit:**

```bash
git add localmesh/service_catalog/redis/
git commit -m "catalog: introduce redis plugin"
```

### Task 4.3: Trim `deploy.toml` to one instance, then commit

The worktree's `deploy.toml` has three `[[plugins.redis]]` blocks (cache, async_handler, a `localmesh add`-style sketch). Phase 1's cap rejects multi-instance. Trim to the `cache` instance; move the other two into comments documenting the Phase 2 shape so they aren't lost.

- [ ] **Step 1:** Edit `deploy.toml` so the live content is just the `cache` instance:

```toml
# Per-instance orders for plugins that have consumer-tunable config or
# exports. Plugins selected in project.toml that expose neither do not
# need a [[plugins]] block here.

[plugins]
[[plugins.redis]]
  service_name = "cache"            # logical instance identity; defaults to slug
  [plugins.redis.config]
  maxmemory = "512mb"               # overrides the manifest default ("256mb")
  [plugins.redis.exports]
  connection_url = "CACHE_URL"      # overrides the rendered default destination

# Phase 2 will allow multiple [[plugins.<slug>]] blocks; the shapes below
# document what that will look like. Phase 1's ValidateDeploy rejects them.
#
# [[plugins.redis]]
#   service_name = "async_handler"
#   [plugins.redis.config]
#   noevict = true
#   [plugins.redis.exports]
#   connection_url = "ASYNC_PUBSUB_URL"
#
# [[plugins.redis]]
#   service_name = "redis"
#   [plugins.redis.exports]
#   connection_url = "REDIS_URL"
```

- [ ] **Step 2:** Confirm `project.toml` has `plugins = ["redis"]`. Confirm `docker-compose.yml` references `${CACHE_URL}` on www.
- [ ] **Step 3:** Run `go run ./localmesh_src/cmd/localmesh build`. Expected: clean exit. `.env` regenerated with `CACHE_URL=redis://redis:6379` and `REDIS_MAXMEMORY=512mb`.
- [ ] **Step 4 — commit:**

```bash
git add deploy.toml project.toml docker-compose.yml
git commit -m "deploy: introduce deploy.toml for redis cache instance"
```

### Task 4.4: Commit `docs/CLEANUP.md`

- [ ] **Step 1:** `ls docs/CLEANUP.md`. Confirm content matches the spec §11 entries.
- [ ] **Step 2 — commit:**

```bash
git add docs/CLEANUP.md
git commit -m "docs: track config-inversion cleanup followups"
```

### Task 4.5: Update `plugin-conventions.md` §4

- [ ] **Step 1:** Verify current section numbering: `grep -nE '^##' docs/engineering/rules/plugin-conventions.md`. Confirm §4 is still "Overlap with `plugin.toml`" (or whatever it has become).
- [ ] **Step 2:** Edit §4 to add:
  - `[[config_vars]]` schema (name/description/value).
  - `[[exports]]` schema (name/description/template/env/required); note description is not validated.
  - Internal naming rule for Phase 1: `<UPCASE plugin slug>_<UPCASE config_var name>`.
  - Name-collision rule between `config_vars` and `exports`.
  - `.Service` singular semantics for Phase 1 multi-service plugins.
- [ ] **Step 3 — commit:**

```bash
git add docs/engineering/rules/plugin-conventions.md
git commit -m "docs: extend plugin-conventions §4 for config_vars + exports"
```

### Task 4.6: Update `DESIGN_DECISIONS.md` row 56

- [ ] **Step 1:** Edit `docs/stakeholder/DESIGN_DECISIONS.md` row 56 (Plugin manifest) to reference `config_vars` + `exports` + `deploy.toml`. Add prod-considerations.
- [ ] **Step 2:** Open section: add entries for Phase 2 (multi-instance), Phase 3 (`localmesh add`), and the dexseed/OIDC-mint cleanup (cross-link to `docs/CLEANUP.md`).
- [ ] **Step 3 — commit:**

```bash
git add docs/stakeholder/DESIGN_DECISIONS.md
git commit -m "docs: record plugin config inversion on row 56"
```

### Task 4.7: Add `[[config_vars]]` + `[[exports]]` to one fixture plugin

For golden-test coverage downstream.

- [ ] **Step 1:** `ls localmesh_src/testdata/sample_catalog/`. Confirm `alpha` and `beta` exist.
- [ ] **Step 2:** Add one `[[config_vars]]` and one `[[exports]]` to `alpha/plugin.toml`. Pick names that won't collide.
- [ ] **Step 3:** `go test ./localmesh_src/...`. Expected: PASS. If `internal/render` goldens drift, regenerate:

```bash
go test ./localmesh_src/internal/render -update
go test ./localmesh_src/internal/render
```

(`-update` is the convention; confirm with `grep -n 'update' localmesh_src/internal/render/render_test.go` — there's a `var update = flag.Bool(...)` definition.)

- [ ] **Step 4 — commit:**

```bash
git add localmesh_src/testdata/sample_catalog/alpha/plugin.toml localmesh_src/internal/render/testdata/expected.compose.yaml
git commit -m "testdata: add config_vars + exports to alpha fixture"
```

(Drop the second path from `git add` if the golden didn't actually shift.)

### Task 4.8: Live acceptance via `localmesh build`

`make setup` chains `localmesh setup` → which calls `dexseed.Seed` against the deleted `auth/dex.yaml.sample`. That's a known catalog-deletion consequence tracked in `docs/CLEANUP.md`; out of scope here. Use `localmesh build` directly for Phase 1 acceptance.

- [ ] **Step 1:** From the worktree root, run:

```bash
go run ./localmesh_src/cmd/localmesh build
```

Expected: clean exit. Stdout may name regenerated files.

- [ ] **Step 2:** Inspect `.env`. Expected lines in the managed section:

```
CACHE_URL=redis://redis:6379
REDIS_MAXMEMORY=512mb
```

(`grep -E '^(CACHE_URL|REDIS_MAXMEMORY)=' .env`.)

- [ ] **Step 3:** `docker compose up -d redis`. Expected: redis comes up. `docker compose ps redis` shows `healthy` within ~30s (the healthcheck has 3 retries × 10s).

- [ ] **Step 4:** Verify the consumer can read the URL via a one-off container on the same network:

```bash
PROJECT_NAME=$(grep '^PROJECT_NAME=' .env | cut -d= -f2)
CACHE_URL=$(grep '^CACHE_URL=' .env | cut -d= -f2)
docker run --rm --network "${PROJECT_NAME}_default" \
    redis:7-alpine redis-cli -u "$CACHE_URL" ping
```

Expected output: `PONG`. If the network name differs (compose v2 sometimes uses `<project>_default` vs `<project>-default`), find it with `docker network ls | grep "$PROJECT_NAME"`.

- [ ] **Step 5 — negative test (Phase 1 cap):** Temporarily uncomment one of the multi-instance blocks in `deploy.toml`. Re-run `go run ./localmesh_src/cmd/localmesh build`. Expected: fails with `Phase 1 accepts at most one instance per slug`. Restore the comment.

- [ ] **Step 6 — negative test (export fallback):** Temporarily remove the `[plugins.redis.exports]` block from `deploy.toml`. Re-run `localmesh build`. Expected: succeeds; `.env` now has `REDIS_URL=…` (the manifest-rendered default) instead of `CACHE_URL=…`. Restore.

- [ ] **Step 7 — commit any incidental fixes** that surfaced during acceptance, with focused messages.

### Task 4.9: Add dexseed/OIDC cleanup entry to `CLEANUP.md`

The catalog deletion left `setup.go`'s dexseed call + the OIDC-secret mint as dead-code-after-deletion. Track for a focused follow-up PR.

- [ ] **Step 1:** Append to `docs/CLEANUP.md` (open section):

```markdown
### 7. Remove setup.go's auth-plugin coupling

Source: config-inversion §[catalog deletion]. The catalog deletion left two
auth-coupled paths in `localmesh_src/cmd/localmesh/setup.go`:
- The `dexseed.Seed(samplePath, ...)` call at line 100-104 reads
  `localmesh/service_catalog/auth/dex.yaml.sample`, which no longer exists.
- The `LOCALMESH_OIDC_CLIENT_SECRET` mint-once-preserve emission in
  `envwriter.WriteManaged` exists solely so dexseed has a stable secret to
  read. With dexseed gone, this mint becomes dead.

Touches: `localmesh_src/cmd/localmesh/setup.go` (drop dexseed step +
import), `localmesh_src/internal/envwriter/envwriter.go` (drop the OIDC
mint block), `localmesh_src/internal/dexseed/` (consider removing the
package entirely once no caller remains).
```

- [ ] **Step 2 — commit:**

```bash
git add docs/CLEANUP.md
git commit -m "docs: track auth-coupling cleanup after catalog deletion"
```

### Task 4.10: Spec-vs-implementation drift check

- [ ] **Step 1:** Re-read spec `docs/superpowers/specs/2026-05-22-plugin-config-inversion-design.md` against the implementation. Confirm validators, error messages, rendering rules, and template-context fields match the spec's tables exactly.
- [ ] **Step 2:** If anything drifted, update the side that's wrong (don't leave drift).
- [ ] **Step 3 — commit any drift fix:** `<area>: align with spec on <specific point>`.

### Task 4.11: Final verification

- [ ] **Step 1:** `go test ./localmesh_src/...`. PASS.
- [ ] **Step 2:** `go vet ./localmesh_src/...`. Clean.
- [ ] **Step 3:** `gofmt -l localmesh_src/internal/{manifest,envwriter}/`. No output.
- [ ] **Step 4:** `goimports -l localmesh_src/internal/{manifest,envwriter}/`. No output. (CLAUDE.md notes `goimports` alongside `gofmt`.)
- [ ] **Step 5:** `git log --oneline main..HEAD`. Confirm commits read as a clean story end-to-end.

---

## Out of scope (deferred to follow-ups)

- Multi-instance rendering (Phase 2). Spec §8.
- `localmesh add` scaffolding (Phase 3). Spec §8.
- `[[services]].scheme` removal. Tracked in `docs/CLEANUP.md` §1.
- `postgres16` adoption + the `postgres`/`postgres16` slug mismatch. Tracked in `docs/CLEANUP.md` §2.
- README Environment-table generation. Tracked in `docs/CLEANUP.md` §3.
- `plugin-conventions.md` §4 refresh after Phase 1 lands. Tracked in `docs/CLEANUP.md` §4.
- `project.toml` ↔ `deploy.toml` sync linter. Tracked in `docs/CLEANUP.md` §5.
- Identity tuple rebind (Phase 2 trigger). Tracked in `docs/CLEANUP.md` §6.
- dexseed call + OIDC-secret mint removal. Tracked in `docs/CLEANUP.md` §7 (added in Task 4.9).
