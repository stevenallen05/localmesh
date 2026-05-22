# Plugin conventions

The schema reference for plugin authors and the lint checklist `localmesh build` enforces. For the worked walkthrough that teaches the shape from scratch, start at [`../PLUGIN_AUTHORING.md`](../PLUGIN_AUTHORING.md) and come back here for field-by-field detail.

Long-form rationale: [`../../superpowers/specs/2026-05-18-localmesh-namespacing-and-plugin-exports-design.md`](../../superpowers/specs/2026-05-18-localmesh-namespacing-and-plugin-exports-design.md).

## 0. Data-source rule (highest priority)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

A plugin or tool reads only the inputs explicitly designated for it. Compose files are runtime, not build inputs. Labels carry meaning for their declared consumer only — do not scrape them from another layer. When data isn't where you expected, do not invent an ingestion path. Stop, surface the question, and wait for direction.

This rule is repeated in `CLAUDE.md`, `docs/engineering/rules/golang-basics.md`, and `docs/engineering/rules/rust-basics.md`.

## 1. Plugin file layout

Every plugin at `localmesh/service_catalog/<plugin>/` ships exactly three files:

- `plugin.toml` — manifest (§4).
- `docker-compose.yml` or `docker-compose.yaml.gotmpl` — the container(s) the plugin runs (§5 for templating).
- `README.md` — the consumer-wiring template (§3).

A meta-package — a plugin that declares `plugins = [...]` and ships no `[[services]]` of its own — keeps all three files. Its compose is the stub:

```yaml
# Meta-package — services come from the plugins listed in plugin.toml.
services: {}
```

`base/` is the canonical meta-package; `localmesh build` flattens its `plugins = [...]` transitively. TOML trap: `plugins = [...]` must precede any `[identity]` table header, or it parses as `identity.plugins` and the loader never sees it.

## 2. Identity tuple

Every service in this project — app-tier or catalog plugin — declares an **identity tuple** of three values: `service_name`, `module_name`, `owned_by`.

| Field | Meaning | Source |
|---|---|---|
| `service_name` | OTel `service.name`. One per container role. | `[[services]].container` in `plugin.toml`; OTel SDK reports the same value. |
| `module_name` | LocalMesh namespace axis. `app` for app-tier services. `<plugin-slug>` for catalog plugins. Slug cannot be `app`. | `[identity].module_name` in `plugin.toml`. |
| `owned_by` | Contact email. | App-tier: `${TECH_LEAD_EMAIL}` from `.env`. Plugins: `[identity].owned_by`. |

The tuple reaches the running system over two channels:

- `metrics.service_name` / `metrics.module_name` / `metrics.owned_by` docker labels — Vector reads these for log enrichment (see [`./logging-platform.md`](./logging-platform.md)).
- `OTEL_SERVICE_NAME` + `OTEL_RESOURCE_ATTRIBUTES` — the OTel SDK reads these for traces and metrics.

The two channels are populated by hand today; collapsing them into one is tracked as the *Identity source consolidation* Open item in [`../../stakeholder/DESIGN_DECISIONS.md`](../../stakeholder/DESIGN_DECISIONS.md).

`service.namespace` is tenant-level identity (interpolates from `${PROJECT_NAME}`), carried on every app-tier service's `OTEL_RESOURCE_ATTRIBUTES`. Not part of the per-service identity tuple.

## 3. Consumer wiring (`README.md`)

Every catalog plugin ships a `README.md` next to its compose file. The file follows this strict template. Sections that do not apply write `None` rather than being omitted — the consumer's eye lands at a predictable spot every time.

```markdown
# Consumer wiring — `<plugin>/`

## Overview
One paragraph. What the plugin runs, what it gives the consumer.

## Environment
Markdown table with columns `Name | Sample value | Source`.
- `Name`: the env var the consumer sets on its service.
- `Sample value`: literal or template-string example.
- `Source`: `plugin` if the value is plugin-owned and copy-pasted as-is; `app` if the value is app-specific and the app provides it following the documented shape.

## Volumes
List of volume mounts the consumer must add to its service. `None` if not applicable.

## depends_on
The exact YAML block to paste. `None` if the plugin does not need to be up before the consumer.

## Labels
`metrics.*` and `katenary.v3/*` labels the consumer must add specifically for this plugin. Identity-tuple labels the app already declares (per §2) are mentioned only if the plugin needs a specific value.

## k8s rendering
katenary labels for secret promotion / `values-from` references. `None` if the plugin exposes no sensitive env.

## Notes
Free-form. Format contracts, runtime obligations, trace-context expectations, caveats, links to deeper specs.

## Example app service block
Drop-in YAML composing every section above into one consumer service definition. Copy-paste-ready.
```

Worked example for `postgres16/`: [`../../../localmesh/service_catalog/postgres16/README.md`](../../../localmesh/service_catalog/postgres16/README.md). Worked example for `redis/`: walked end-to-end in [`../PLUGIN_AUTHORING.md`](../PLUGIN_AUTHORING.md).

## 4. `plugin.toml` schema

`plugin.toml` owns identity (`module_name`, `owned_by`), service definitions (`container`, `port`, `scheme`), exposure (`expose_via_ingress`, `ingress`), and auth gating (`requires_auth`, default `true` — services that opt out of the ingress auth gate set `requires_auth = false`; the IdP itself is the canonical opt-out). `README.md` cites these values by reference and never redeclares them. Plugin-internal env vars derived from `plugin.toml` by `localmesh build` do not appear in `README.md`'s Environment table:

- From `[identity]`, keyed by **plugin slug**: `<PLUGIN>_MODULE_NAME`, `<PLUGIN>_OWNED_BY`.
- From `[[services]]`, keyed by **container slug**: `<CONTAINER>_PORT`, `<CONTAINER>_EXPOSE_VIA_INGRESS`, `<CONTAINER>_INGRESS`.

Mesh membership is `needs_mtls_sidecar` in each `[[services]]` entry (default `true`; `false` opts a service out of the mesh data plane). `kuma-cp`, `ingress`, `dex`, `postgres`, and `postgres-exporter` set it to `false`. The CLI populates `.Services[*].Meshed` from this field; the `security/` plugin's template ranges over `.Services` and emits a `<name>-mesh` kuma-dp sidecar for each `Meshed` entry.

`README.md`'s Environment table is for env vars the **consumer app** sets, not values exported into the platform's `.env`.

### 4.1. `[[config_vars]]` — plugin-author knobs

Each `[[config_vars]]` entry is a consumer-tunable default. Fields all required and all single-line strings (`name`, `description`, `value`). Names unique within the plugin. Default `value` lives in `plugin.toml`; `deploy.toml` overrides per instance.

Resolved values land in `.env` keyed by **plugin slug + config name**: `<UPCASE plugin>_<UPCASE name>`. So `redis/`'s `maxmemory` becomes `REDIS_MAXMEMORY`. The plugin's compose interpolates the resolved value via `${REDIS_MAXMEMORY}`. Phase 2 switches the prefix to the per-instance `service_name` when multi-instance lands.

### 4.2. `[[exports]]` — values the plugin delivers to consumers

Each `[[exports]]` entry is one consumer-facing env var the plugin publishes. Required fields: `name` (unique), `template` (the value, e.g. `redis://{{ .Service.Container }}:{{ .Service.Port }}`), `env` (the destination env-var name template, e.g. `{{ .ServiceName | upper }}_URL`). Optional: `description`, `required` (default `false`). Validation is asymmetric to `[[config_vars]]`: `description` is **not** validated on exports (it's documentation for plugin authors, not a runtime contract).

Templates render through `text/template` with `Option("missingkey=error")` and one registered function (`upper`). No Sprig — the export-render path is intentionally narrower than the `.gotmpl` compose path (§5). The template `.` is an `exportCtx` with `.Project`, `.Plugin`, `.ServiceName`, `.Service` (singular — see §4.4), and `.ConfigVars` (the resolved per-instance config map).

`deploy.toml` may override `env` per instance (`[plugins.<slug>.exports] name = "<DEST>"`) to rename the destination. The plugin author owns the value; the consumer owns the destination name.

### 4.3. `config_vars` and `exports` share one namespace

Within a plugin, a `[[config_vars]].name` and a `[[exports]].name` cannot collide. The CLI rejects it at `LoadPlugin`. This keeps the per-plugin namespace flat so the `deploy.toml` author isn't guessing which array a given name belongs to.

### 4.4. Phase 1: `.Service` is singular

A plugin with multiple `[[services]]` entries still gets a single `.Service` in the export-render context — the first declared service wins. Plugins that need to template against a non-first service should keep export templates simple (or wait for Phase 2's per-instance rendering, which keys `.Service` by `service_name`).

## 5. Template author reference

`.gotmpl`-suffixed plugin files render through Go's `text/template` with the [Masterminds/sprig](https://masterminds.github.io/sprig/) function library. Plain `.yaml` / `.yml` files pass through verbatim — no templating, no substitution. The renderer runs with `Option("missingkey=error")`: referencing a key that doesn't exist fails the build loudly instead of emitting `<no value>`.

### Render context

Templates receive a `*Context` (`localmesh_src/internal/template/template.go`) with four fields:

- `.Project` — the typed `project.toml` view. Fields: `.Project.Name`, `.Project.Namespace`, `.Project.TechLead`, `.Project.ExternalDomain`, `.Project.LocalDomain`, `.Project.Plugins`, `.Project.Services` (the app-tier `[[services]]`).
- `.Plugin` — this plugin's own typed `plugin.toml`. Fields: `.Plugin.Name` (the directory-derived slug), `.Plugin.Identity.ModuleName`, `.Plugin.Identity.OwnedBy`, `.Plugin.Plugins` (meta-dependencies; empty for leaf plugins), `.Plugin.Services`.
- `.Env` — `map[string]string` of the `.env`-managed values the CLI emitted (`internal/envwriter`). Carries CA / OIDC material, not the per-plugin `<PLUGIN>_*` vars from §4 — those reach compose through `${}` interpolation at runtime, not the template map.
- `.Services` — `[]ServiceCtx` (`Name`, `Port`, `Meshed`, `ExposeViaIngress`) of every registry-known service (project `[[services]]` + every plugin `[[services]]`), sorted by name. `Meshed` is true when `needs_mtls_sidecar` is true (the default). `ExposeViaIngress` mirrors `expose_via_ingress`. The `security/` template ranges over `.Services` to emit a `<name>-mesh` kuma-dp sidecar for each `Meshed` entry and a `MeshHTTPRoute` for each `ExposeViaIngress` entry.

### Custom functions

Two functions are registered on top of sprig (`localmesh_src/internal/template/funcs.go`):

- `spiffeURI <container> <project> <local_domain>` → `spiffe://<container>.<project>.<local_domain>`. Built from the container name, never the plugin slug — renames like `database` → `postgres16` do not move the SPIFFE identity.
- `identityLabels <plugin> <service_name>` → a three-key YAML block (`metrics.service_name`, `metrics.module_name`, `metrics.owned_by`) drawn from the plugin's identity tuple. The caller indents the result.

### Data-source discipline

A template may only read from the context fields above. Reaching into a sibling plugin's files, scraping a runtime artifact, or parsing labels meant for another consumer is forbidden (§0). If the value you need isn't on the context, stop and ask — do not invent an ingestion path.

## 6. Lint checklist

`localmesh build` enforces these at manifest-load time and fails loudly when violated. Items marked **documented** are conventions the build does not yet enforce; treat them as obligations regardless.

Items marked **enforced** fail the build with a clear error. Items marked **documented** are conventions the build does not yet check; treat them as obligations regardless.

| Rule | Status | Failure mode |
|---|---|---|
| `[identity].module_name` non-empty | enforced | `identity.module_name required` |
| `[identity].owned_by` non-empty | enforced | `identity.owned_by required` |
| Every `[[services]].scheme` is one of `grpc`, `http`, `https`, `tcp`, `postgresql` | enforced | `container "X" has invalid scheme "Y"` |
| Every `[[services]]` has a `scheme` | enforced | `scheme required on container "X"` |
| `[[config_vars]]` has `name`, `description`, `value`; `name` and `description` single-line | enforced | `config_vars[i].<field> required` |
| `[[exports]]` has `name`, `template`, `env`; `name` single-line | enforced | `exports[i].<field> required` |
| `[[config_vars]].name` unique within the plugin | enforced | `config_vars: duplicate name "X"` |
| `[[exports]].name` unique within the plugin | enforced | `exports: duplicate name "X"` |
| `[[config_vars]].name` does not collide with any `[[exports]].name` in the same plugin (§4.3) | enforced | `name collision between config_vars and exports: "X"` |
| `.gotmpl` templates only reference fields documented in §5 | enforced | `text/template`'s `missingkey=error`. |
| `[[services]].container` is unique across **all** loaded plugins | documented | Render-time scalar conflict if values differ; silent if identical. |
| Plugin slug is not the literal `app` | documented | The slug `app` is reserved for app-tier (§2). Not checked by the loader. |
| Meta-package: top-level `plugins = [...]` appears **before** any `[identity]` header | documented | Silent parse-as-`identity.plugins`, loader sees no deps. |
| All three files present: `plugin.toml`, compose, `README.md` | documented | Missing `README.md` does not fail the build today. |

The four most common build-time errors a plugin author hits:

1. **Identity missing** — empty `module_name` or `owned_by`. Fill in `plugin.toml`.
2. **Invalid scheme** — a `[[services]]` entry has `scheme = "udp"` or similar. Use one of the five.
3. **Shared-namespace collision** — same `name` used in `[[config_vars]]` and `[[exports]]` within one plugin (§4.3). Rename one of them.
4. **Template missing key** — `{{ .Plugin.Foo }}` references a field that doesn't exist. Cross-check against §5's context shape.

## See also

- [`../PLUGIN_AUTHORING.md`](../PLUGIN_AUTHORING.md) — the worked walkthrough.
- [`./katenary-top-seven.md`](./katenary-top-seven.md) — the `katenary.v3/*` labels referenced from `README.md` examples.
- [`./logging-platform.md`](./logging-platform.md) — the JSON envelope contract apps must emit; depends on the identity-tuple labels above.
