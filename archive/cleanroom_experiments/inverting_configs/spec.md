# Plugin config inversion — Design

**Date:** 2026-05-22
**Status:** draft
**Supersedes:** `2026-05-22-plugin-env-vars-design.md` (parked; archived elsewhere)
**Touches:** `localmesh_src/internal/manifest/`, `localmesh_src/internal/envwriter/`, new `deploy.toml` at repo root, new `localmesh/service_catalog/redis/`, `docs/engineering/rules/plugin-conventions.md` (§4 addendum), `docs/stakeholder/DESIGN_DECISIONS.md` (row 56), `docs/CLEANUP.md` (new)
**Companion docs:** `docs/CLEANUP.md`, `docs/engineering/rules/plugin-conventions.md`

---

## 1. Goal

Invert plugin configuration to a catalog-order model. The plugin manifest is the menu. It declares input knobs the consumer may tune (`config_vars`) and outputs the plugin delivers (`exports`, each with a value template). A new `deploy.toml` is the order. Its top-level `[plugins]` table holds an array per plugin slug — `[[plugins.<slug>]]` — each entry an instance with optional knob overrides and destination renames. `localmesh build` resolves orders against the menu and lands rendered values in the `.env` managed section. The consumer reads them by interpolation (`${CACHE_URL}`).

Phase 1 lands the resolve-and-deliver loop on a single redis instance. The `deploy.toml` schema is multi-instance-shaped from the start (so Phase 2's rendering work doesn't require a schema break), but Phase 1 validates at most one instance per slug. Multi-instance rendering and `localmesh add` scaffolding are deferred.

## 2. Background

The precursor `2026-05-22-plugin-env-vars-design.md` proposed a flat `[[env_vars]]` array on each manifest, with values rendered into `.env` under a forced prefix (`POSTGRES16_HOST`, `POSTGRES16_PORT`). Four roadblocks made it unworkable:

- **Forced name prefix.** The dev could not choose the destination env-var name. The prefix mechanism stayed an unresolved Open. Every candidate (`<plugin-slug>_*`, `<container>_*`) lost legibility somewhere.
- **Granular pieces pushed assembly into app code.** Emitting `HOST` / `PORT` / `DATABASE` separately meant every consumer rebuilt the connection string itself. The plugin author knew the right format. The schema gave them no place to write it down.
- **Flat list conflated direction.** Values that configure the plugin's own container and values delivered to the consumer lived in the same array. The build couldn't tell which were inward and which were outward from the schema alone.
- **Single-instance only.** The forced prefix collapsed if two postgres instances shipped under one project. The prefix would collide.

The inversion below clears all four on the consumer-facing surface. Naming moves to the consumer (`deploy.toml`). Assembly moves to the plugin (`export.template`). Direction is in the schema (`config_vars` vs `exports`). Multi-instance is schema-supported on day one (`[[plugins.<slug>]]` array shape); Phase 1 validates N=1 and Phase 2 lifts that to render the rest.

The internal config_var surface still uses a plugin-keyed prefix (`<PLUGIN>_<CONFIG>`) because Phase 1 plugin composes are plain `.yml` passthrough — the variable name has to be deterministic from the plugin author's perspective alone. That's documented; Phase 2 changes it.

## 3. Files & roles

| File | Role | Authored by |
|---|---|---|
| `project.toml` | Selection. Which plugin sources are active. Charter unchanged; stays business-legible. | dev |
| `plugin.toml` | Manifest. Identity + services + new `[[config_vars]]` (inputs) + new `[[exports]]` (outputs with template + default env name + required). | plugin author |
| `deploy.toml` (new, repo root) | Per-instance orders. Top-level `[plugins]` table. For each plugin slug, an array-of-tables `[[plugins.<slug>]]` lists instances of that source. Each instance: `service_name`, `[plugins.<slug>.config]` knob overrides, `[plugins.<slug>.exports]` destination renames. | dev |
| `.env` | Compose interpolation source. Now also carries resolved export values. Managed section regenerated each build. | CLI (`localmesh build`) |

`project.toml.plugins` and `deploy.toml`'s `[plugins]` keys are kept in sync by hand for Phase 1. `localmesh add` (Phase 3) automates that.

## 4. Manifest additions

`manifest.Plugin` gains two slices:

```go
type Plugin struct {
    Name       string      `toml:"-"`
    Identity   Identity    `toml:"identity"`
    Plugins    []string    `toml:"plugins"`
    Services   []Service   `toml:"services"`
    ConfigVars []ConfigVar `toml:"config_vars"`   // new
    Exports    []Export    `toml:"exports"`       // new
}

type ConfigVar struct {
    Name        string `toml:"name"`
    Description string `toml:"description"`
    Value       string `toml:"value"`             // default; consumer may override
}

type Export struct {
    Name        string `toml:"name"`              // logical handle the consumer references in deploy.toml
    Description string `toml:"description"`
    Template    string `toml:"template"`          // gotmpl; renders the value
    Env         string `toml:"env"`               // gotmpl; renders the default destination env-var name
    Required    bool   `toml:"required"`
}
```

`Project` is not extended in Phase 1. Project-level config_vars / exports aren't motivated. The same shape lifts onto `Project` later if a use case appears.

### TOML shape (redis worked example)

```toml
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[services]]
container          = "redis"
port               = 6379
expose_via_ingress = false

[[config_vars]]
name        = "maxmemory"
description = "Memory cap before eviction kicks in"
value       = "256mb"

[[exports]]
name        = "connection_url"
description = "Full redis:// connection string the consumer dials"
template    = "redis://{{ .Service.Container }}:{{ .Service.Port }}"
env         = "{{ .ServiceName | upper }}_URL"
required    = true
```

Note `{{ .Service.Container }}` for the host. `service_name` is the logical instance identity (drives the default `env` name); the compose-network-resolvable host is the container name from `[[services]]`. Phase 2's per-instance compose rendering changes the host axis when it lands.

### Validation (at `LoadPlugin`)

| Rule | Error |
|---|---|
| `config_vars[i].name` empty or multiline | `<path>: config_vars[i].name required` / `must be single-line` |
| `config_vars[i].description` empty or multiline | same shape |
| `config_vars[i].value` empty | `<path>: config_vars[i].value required` |
| Duplicate `config_vars[i].name` | `<path>: config_vars: duplicate name "X"` |
| `exports[i].name` empty or multiline | same shape |
| `exports[i].template` empty | `<path>: exports[i].template required` |
| `exports[i].env` empty | `<path>: exports[i].env required` |
| Duplicate `exports[i].name` | `<path>: exports: duplicate name "X"` |
| `config_vars` name overlaps `exports` name | `<path>: name collision between config_vars and exports: "X"` |

Templates are NOT parsed at manifest load. Syntax errors surface at `localmesh build`, wrapped with the manifest path and the offending name. Matches the precursor's discipline.

### Multi-service `.Service` resolution

A plugin's manifest may declare multiple `[[services]]` entries (postgres+postgres-exporter, today). The Phase 1 template context exposes the **first** entry as `.Service` (singular). Plugins with multiple services must order `[[services]]` so the consumer-facing one is first, or use the explicit-key form in their template via `.Services` (a future addition; not in Phase 1). For now, redis has one service, postgres16 doesn't ship in Phase 1, so the ambiguity bites no current plugin.

## 5. `deploy.toml`

```toml
# Per-instance orders for plugins with consumer-tunable config or exports.
# Plugins selected in project.toml that expose neither do not need an entry.

[plugins]

[[plugins.redis]]
  service_name = "cache"            # logical instance identity; defaults to the slug
  [plugins.redis.config]
  maxmemory = "512mb"               # overrides the manifest default
  [plugins.redis.exports]
  connection_url = "CACHE_URL"      # overrides the rendered default destination
```

The TOML key path encodes the source: `[[plugins.redis]]` means "an instance of the `redis` plugin." No `source = "..."` field is needed. Multi-instance is the same shape with a second `[[plugins.redis]]` block.

### Schema

```go
type Deploy struct {
    Plugins map[string][]Instance `toml:"plugins"`  // slug -> instances
}

type Instance struct {
    ServiceName string            `toml:"service_name"`
    Config      map[string]any    `toml:"config"`    // any TOML scalar; coerced to string at render time
    Exports     map[string]string `toml:"exports"`   // destination override (gotmpl); see §6
}
```

`Config` is `map[string]any` so the dev can write `noevict = true` (bool) or `maxmemory = "512mb"` (string) naturally. The build coerces to string when emitting into `.env`.

### Loader split

```go
func LoadDeploy(path string) (*Deploy, error)             // parse only; empty Deploy if path missing
func ValidateDeploy(d *Deploy, plugins []*Plugin) error   // cross-validate against the FLATTENED plugin list
```

`LoadAll` now returns `(*Project, []*Plugin, *Deploy, error)`. The sequence: load project → flatten plugins → load deploy (optional) → validate deploy against the flattened list. `envwriter.WriteManaged` takes the same triple.

### Validation (split across `LoadDeploy` parse and `ValidateDeploy` cross-check)

Parse-time, from `LoadDeploy` (purely structural):

| Rule | Error |
|---|---|
| `[plugins]` not a table-of-arrays | TOML decoder error, surfaced as-is |
| `Instance.ServiceName` contains `\n` | `deploy.toml: plugins.<slug>[i].service_name must be single-line` |

Cross-validation, from `ValidateDeploy` against the flattened plugin list (R4 fix):

| Rule | Error |
|---|---|
| Slug not in the flattened plugin list | `deploy.toml: plugins.<slug>: source not selected in project.toml (flattened: a, b, c)` |
| Phase 1: more than one instance under one slug | `deploy.toml: plugins.<slug>: Phase 1 accepts at most one instance per slug; multi-instance lands in Phase 2` |
| Duplicate `service_name` within one slug's instances | `deploy.toml: plugins.<slug>: duplicate service_name "X" (entries i and j)` |
| `[plugins.<slug>.config].X` not declared in the source's `config_vars` | `deploy.toml: plugins.<slug>.config.X unknown (not in <slug>/plugin.toml [[config_vars]])` |
| `[plugins.<slug>.exports].X` not declared in the source's `exports` | `deploy.toml: plugins.<slug>.exports.X unknown` |

The "required export not set" check moves out of LoadDeploy entirely; it can only be decided after rendering (§6).

### Override values are rendered (R3 pin)

The destination override on `[plugins.<slug>.exports]` goes through `text/template` with the same `exportCtx` as the manifest's `Env` default. The manifest's value-template and the deploy override share one rendering lane. So a dev who wants `connection_url = "{{ .Project.Name | upper }}_CACHE_URL"` may write that. Literal strings like `"CACHE_URL"` also work (no template directives renders to itself).

`deploy.toml` may be absent. Every plugin then runs with manifest defaults. A `required = true` export with an empty rendered default still fails at render time (§6).

## 6. Resolution + rendering

`localmesh build` runs a new step inside `envwriter.buildManaged`, after the existing per-service `<CONTAINER>_*` emissions:

```
for each plugin P in manifest.LoadAll order:
    instances := deploy.Plugins[P.Name]
    if instances empty: instances = [{ServiceName: P.Name}]    # default single instance
    for each instance I in instances:
        config := {cv.Name: cv.Value for cv in P.ConfigVars}    # defaults
        for (name, raw) in I.Config:
            config[name] = coerceString(raw)                    # any -> string
        serviceName := I.ServiceName; if empty: serviceName = P.Name
        ctx := exportCtx{
            Project:     <projectLite>,
            Plugin:      <pluginLite>,
            ServiceName: serviceName,
            Service:     P.Services[0],   # Phase 1: first/only [[services]] entry
            ConfigVars:  config,
        }
        for each export E in P.Exports:
            envSrc := E.Env
            if override, ok := I.Exports[E.Name]; ok: envSrc = override
            envName := render(envSrc, ctx)              # override goes through render too (R3)
            if envName == "" and E.Required: error      # required-export validation lives here (B5)
            value := render(E.Template, ctx)
            collision-check envName against the whole managed section
            out[envName] = value
        for each (name, value) in config:
            out[upcase(P.Name) + "_" + upcase(name)] = value    # internal config_var env (Phase 1 prefix)
```

**Phase 1 simplification:** `.Service` reads `P.Services[0]` (first/only entry). Phase 2 generalises to per-instance per-service rendering.

### Template context

```go
type exportCtx struct {
    Project     projectLite                // Name, Namespace, LocalDomain, ExternalDomain, TechLead
    Plugin      pluginLite                 // Name (slug), ModuleName, OwnedBy
    ServiceName string                     // from deploy.toml; defaults to plugin slug
    Service     serviceLite                // Container, Port, ExposeViaIngress (locked: only these three)
    ConfigVars  map[string]string          // resolved name -> value (post-coercion)
}
```

`text/template` with `Option("missingkey=error")`. No Sprig in Phase 1. `.ConfigVars` is accessed by `index` to mirror the precursor's pattern: `{{ index .ConfigVars "maxmemory" }}`.

`serviceLite` exposes only `Container`, `Port`, `ExposeViaIngress`. The other `Service` fields (`Ingress`, `RequiresAuth`, `NeedsMTLSSidecar`) are not on the template context. Adding them is a future extension.

### Internal vs consumer-facing env naming

Two distinct naming rules, deliberate:

- **Internal (config_var-driven):** `<UPCASE plugin slug>_<UPCASE config_var name>`. The plugin's own compose interpolates these (e.g. `--maxmemory ${REDIS_MAXMEMORY}`). The plugin author owns this surface; rigidity is fine.
- **Consumer-facing (export-driven):** dev-chosen via `[plugins.<slug>.exports]` in `deploy.toml`, with the manifest's `Env` as fallback. The rigidity that broke the precursor only existed on the consumer side; it doesn't bind on the internal side.

Phase 1 is single-instance-by-construction on the internal side because the plugin compose's `${REDIS_MAXMEMORY}` is a literal interpolation string — it can't depend on the instance. Two instances under one slug would each want their own value at the same key; that's why Phase 1 caps it at one. Phase 2 changes the internal prefix to `<SERVICE_NAME>_<CONFIG>` and converts plugin composes to per-instance `.gotmpl` templates rendered with the instance identity in scope. The schema doesn't change. The plugin compose files do.

### Collision detection

Across the whole managed section. Two emissions to the same env name error with the existing collision shape from envwriter, extended to name the offending export.

## 7. Worked example — redis

`localmesh/service_catalog/redis/plugin.toml`: the §4 shape.

`localmesh/service_catalog/redis/docker-compose.yml`:

```yaml
services:
  redis:
    image: redis:7-alpine
    container_name: redis
    command:
      - redis-server
      - --maxmemory
      - ${REDIS_MAXMEMORY}
      - --maxmemory-policy
      - allkeys-lru
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
    labels:
      metrics.service_name: redis
      metrics.module_name:  redis
      metrics.owned_by:     sre@example.com
```

Stock `redis:7-alpine`. The `--maxmemory ${REDIS_MAXMEMORY}` reads the resolved config_var.

`deploy.toml` (repo root):

```toml
[plugins]

[[plugins.redis]]
  service_name = "cache"
  [plugins.redis.config]
  maxmemory = "512mb"
  [plugins.redis.exports]
  connection_url = "CACHE_URL"
```

After `localmesh build`, `.env`'s managed section gains:

```
CACHE_URL=redis://redis:6379
REDIS_MAXMEMORY=512mb
```

The dev wires the consumer:

```yaml
www:
  environment:
    REDIS_URL: ${CACHE_URL}
  depends_on:
    redis:
      condition: service_healthy
```

`TODO: needs_prod_decisions Redis AUTH/ACL + secret delivery`. In dev the mesh sidecar is the auth boundary. Redis runs auth-less behind it. Production needs a secrets concept (mint-once or external store) plus an ACL hook on the connection_url template. Out of scope here.

## 8. Phasing

| Phase | Scope | Status |
|---|---|---|
| 1 | The resolve-and-deliver loop. Manifest schema (`config_vars` + `exports`). `deploy.toml` shape (multi-instance-supporting) + loader + cross-validation. `envwriter` integration. redis plugin as the worked example. Single instance per slug enforced at validation time. | this spec |
| 2 | Multi-instance rendering. Internal env-var prefix shifts from `<PLUGIN>_<CONFIG>` to `<SERVICE_NAME>_<CONFIG>`. Plugin composes become per-instance `.gotmpl` rendered with the instance identity in scope. Certs / secrets / `OTEL_*` / labels keyed by `service_name`. `plugin-conventions.md` §2 rebinds: `service_name` is the per-instance identity axis; `module_name` stays the plugin-kind axis. **Phase 1 plugin composes that hardcode `${<PLUGIN>_<CONFIG>}` get rewritten as part of this work** — that's the load-bearing schema-break, not the `deploy.toml` shape. | future |
| 3 | `localmesh add <slug>`. Fetch + vendor + scaffold `deploy.toml` from the manifest's required + common-optional knobs. Keeps `project.toml` and `deploy.toml` in sync. | future |

Consumer auto-placement (CLI injecting env-var references onto the right app service) stays deferred indefinitely. The `.env` bridge plus dev-authored `${VAR}` references covers the demo.

## 9. Files touched (Phase 1)

**Modify:**

| Path | Change |
|---|---|
| `localmesh_src/internal/manifest/manifest.go` | Add `ConfigVar`, `Export` types; `Plugin.ConfigVars`, `Plugin.Exports` slices; `validateConfigVars`, `validateExports`, `validateConfigExportNameCollision`. `LoadPlugin` calls all three. Add `Deploy`, `Instance` types and `LoadDeploy(path)` (parse-only) and `ValidateDeploy(d, plugins)`. Extend `LoadAll` to return `(*Project, []*Plugin, *Deploy, error)`; it loads deploy.toml after flattening plugins and calls `ValidateDeploy`. |
| `localmesh_src/internal/manifest/manifest_test.go` | Tests for new validation rules and the loader/validator split. |
| `localmesh_src/internal/envwriter/envwriter.go` | New `resolveAndEmitInstances` step inside `buildManaged`, after the per-service flags. New `exportCtx`, `serviceLite`, `renderExportValue`, `renderExportEnvName`, `coerceString`, `emitConfigVarsInternal` helpers. `WriteManaged` signature gains the `*Deploy` argument. Collision check extended to name the offending export. |
| `localmesh_src/internal/envwriter/envwriter_test.go` | Tests: defaults flow through; deploy overrides win; missing override on required export errors at render time; collision across instances; template error wraps the export name; absent `deploy.toml` is non-fatal; override env-name template is rendered; `Config` value coercion (bool, int → string). |
| `project.toml` | Add `"redis"` to `plugins = [...]`. |
| `docs/engineering/rules/plugin-conventions.md` | §4 addendum: `config_vars` + `exports` schema; the internal naming rule (`<PLUGIN>_<CONFIG>`, Phase 1); the name-collision rule between the two arrays; `.Service` singular for Phase 1 multi-service plugins. |
| `docs/stakeholder/DESIGN_DECISIONS.md` | Row 56 (Plugin manifest) updates to reference `config_vars` + `exports`. Open section gets the Phase 2 / Phase 3 entries. |
| `localmesh_src/testdata/sample_catalog/**/plugin.toml` | The smallest fixture plugin (current candidate: `app-a` — verify with `ls localmesh_src/testdata/sample_catalog/` before editing) gains `[[config_vars]]` + `[[exports]]` so render-goldens cover the new emission. |

**Add:**

| Path | Content |
|---|---|
| `localmesh/service_catalog/redis/plugin.toml` | Identity + one `[[services]]` + one `[[config_vars]]` + one `[[exports]]` (the §4 shape). |
| `localmesh/service_catalog/redis/docker-compose.yml` | The §7 shape. |
| `localmesh/service_catalog/redis/README.md` | Per the `plugin-conventions.md` §3 template. Environment row points the consumer at the destination they chose in `deploy.toml`. |
| `deploy.toml` | The §7 shape. |
| `docs/CLEANUP.md` | The follow-up tracker. See §11. |

**Untouched (deliberately):**

- `[[services]].scheme` field. Removal is parked to `docs/CLEANUP.md` §1. The wire protocol is now named by `exports.template`, so `scheme` will fall out, but that cleanup is a separate PR.
- `postgres16/` plugin. Its mTLS connection string is gnarly and unblocks no Phase 1 scenario. Adoption tracked in `docs/CLEANUP.md` §2.
- The pre-existing `postgres` / `postgres16` slug mismatch in `project.toml` (the entry says `"postgres"` but the directory is `postgres16/`). Out of scope for this spec; tracked in `docs/CLEANUP.md` §2 alongside postgres16's full adoption.
- Multi-instance compose-template rendering, certs, identity tuple changes. Phase 2.

## 10. Testing

**Manifest tests:**

| Case | Asserts |
|---|---|
| `LoadPlugin_configVarsParse` | TOML `[[config_vars]]` blocks materialise on `Plugin.ConfigVars`. |
| `LoadPlugin_configVarMissingName` | error matches `config_vars[0].name required`. |
| `LoadPlugin_configVarDuplicateName` | error matches `config_vars: duplicate name "maxmemory"`. |
| `LoadPlugin_exportsParse` | `[[exports]]` materialise on `Plugin.Exports`. |
| `LoadPlugin_exportMissingTemplate` | error matches `exports[0].template required`. |
| `LoadPlugin_exportMissingEnv` | error matches `exports[0].env required`. |
| `LoadPlugin_nameCollisionAcrossArrays` | a name in both arrays errors. |
| `LoadDeploy_parses` | `deploy.toml` with one slug, one instance materialises on `Deploy.Plugins["redis"][0]`. |
| `LoadDeploy_parsesMultipleInstancesPerSlug` | two `[[plugins.redis]]` blocks materialise as `len(Deploy.Plugins["redis"]) == 2` (LoadDeploy itself is structural; the Phase 1 cap is in ValidateDeploy). |
| `LoadDeploy_absent` | missing file → empty `Deploy`, no error. |
| `ValidateDeploy_sourceNotSelected` | `plugins.foo` when `foo` isn't in the flattened plugin list → error names the flattened list. |
| `ValidateDeploy_phase1RejectsSecondInstance` | two `[[plugins.redis]]` blocks → `Phase 1 accepts at most one instance per slug`. |
| `ValidateDeploy_duplicateServiceName` | two instances of one slug sharing `service_name` → `duplicate service_name`. |
| `ValidateDeploy_unknownConfigKey` | `[plugins.redis.config].X` where `X` isn't declared → error. |
| `ValidateDeploy_unknownExportKey` | `[plugins.redis.exports].X` where `X` isn't declared → error. |

**Envwriter tests:**

| Case | Asserts |
|---|---|
| `WriteManaged_exportEmitsDefaultEnvName` | redis with no deploy override emits `REDIS_URL=redis://redis:6379` (default template render of `Env`). |
| `WriteManaged_deployOverridesEnvName` | deploy.toml `connection_url = "CACHE_URL"` emits `CACHE_URL=…`; no `REDIS_URL` line. |
| `WriteManaged_overrideEnvNameIsRendered` | deploy.toml `connection_url = "{{ .Project.Name | upper }}_URL"` renders against the project name. |
| `WriteManaged_deployOverridesConfigValue` | deploy.toml `maxmemory = "1gb"` emits `REDIS_MAXMEMORY=1gb`. |
| `WriteManaged_configValueCoercion` | deploy.toml `noevict = true` (bool) emits `REDIS_NOEVICT=true` (string). |
| `WriteManaged_requiredExportMissing` | required export with empty rendered destination errors at render time and names the export. |
| `WriteManaged_exportTemplateError` | malformed template errors and names the export. |
| `WriteManaged_exportCollision` | two emissions to the same env name error and name both sides. |
| `WriteManaged_absentDeployUsesDefaults` | no `deploy.toml` present; build succeeds with manifest defaults. |

**Live acceptance** (manual, after the implementation lands):

- `make setup` exits clean.
- `.env` contains `CACHE_URL=redis://redis:6379` and `REDIS_MAXMEMORY=512mb`.
- `docker compose up` brings redis up; healthcheck passes.
- The consumer service (e.g. `www` from the moving target) can resolve `${CACHE_URL}` to `redis://redis:6379` at runtime, and a `redis-cli -u $CACHE_URL ping` from inside that container returns `PONG`. This is the consumer-side validation; a direct `docker compose exec redis redis-cli ping` only proves redis is up.
- Adding a second `[[plugins.redis]]` block to `deploy.toml` fails `make setup` with the Phase 1 cap error.
- Removing the `[plugins.redis.exports]` block falls back to the manifest-rendered default destination name (e.g. `REDIS_URL=…`); build still succeeds.

## 11. CLEANUP inventory

A new `docs/CLEANUP.md` lands alongside this spec. It captures follow-up cleanups that this design either parks or creates. Entries:

1. **`[[services]].scheme` removal.** Validation-only field; wire protocol now named by `exports.template`. Parked from the precursor's scope and from this spec. Focused cleanup PR.
2. **postgres16 adoption.** Migrate `POSTGRES_DB`, `POSTGRES_USER`, `container_name`, the README-documented `DATABASE_URL`, and the `project.toml` slug mismatch (`"postgres"` vs `postgres16/`) onto `config_vars` + `exports`. The mTLS connection string is the hard bit (`sslmode=verify-full` + `sslcert` + `sslkey`).
3. **Per-plugin README Environment table.** Once `exports` is in place the Environment table is derivable from manifest data. Move toward generation or trim the section.
4. **`plugin-conventions.md` §4 env-var naming refresh.** Document the new `<PLUGIN>_<CONFIG>` internal-var rule and the dev-chosen consumer-facing rule once Phase 1 lands.
5. **`project.toml` ↔ `deploy.toml` sync.** Manual for Phase 1. Phase 3 (`localmesh add`) makes it automatic. A "selected-but-never-instantiated" warning in `LoadAll` could bridge the gap.
6. **Identity tuple rebind (Phase 2 trigger).** Multi-instance requires `service_name` to become the per-instance OTel `service.name` and per-instance container name; `module_name` stays the plugin-kind. `plugin-conventions.md` §2 needs the new binding.

## 12. Open decisions

- **Default `env` rendering when `service_name` defaults to the plugin slug.** `{{ .ServiceName | upper }}_URL` on redis produces `REDIS_URL` by default. The bet is that the dev names the destination in `deploy.toml` anyway, so the default only matters when `deploy.toml` is absent. Re-evaluate after first live use.
- **Empty `exports` with `required = false` and an empty rendered template.** Currently allowed silently; the resulting `<env>=` line is benign. Tests should pin the behaviour so future-us notices if it shifts.
- **Multi-service plugins on the template context.** Phase 1 exposes `.Service` (singular = first entry). When the first plugin with two consumer-relevant services lands (postgres16's adoption is the canonical case), decide whether to add `.Services` as a map keyed by container name (precursor pattern) or require `[[services]]` ordering as the contract.
