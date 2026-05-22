# Plugin conventions

Three conventions that bind every catalog plugin and every app-tier service in this project. They are not optional. New plugins land on the same shape; new app-tier services declare the same identity.

The conventions exist informally in the current compose files. This file makes them canonical so plugin authors and app teams stop pattern-matching from neighbouring code.

Long-form rationale: [`../../superpowers/specs/2026-05-18-localmesh-namespacing-and-plugin-exports-design.md`](../../superpowers/specs/2026-05-18-localmesh-namespacing-and-plugin-exports-design.md).

## 0. Data-source rule (highest priority)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

A plugin or tool reads only the inputs explicitly designated for it. Compose files are runtime, not build inputs. Labels carry meaning for their declared consumer only — do not scrape them from another layer. When data isn't where you expected, do not invent an ingestion path. Stop, surface the question, and wait for direction.

This rule is repeated in `CLAUDE.md`, `docs/engineering/rules/golang-basics.md`, and `docs/engineering/rules/rust-basics.md`.

## 1. Bare-minimum plugin contents

Every plugin in `localmesh/service_catalog/<plugin>/` ships three files:

- `docker-compose.yml` (or `docker-compose.yaml.gotmpl` — see §5 for the templating rules).
- `plugin.toml` (the manifest — see §4 for the schema, §2 for the identity tuple it carries).
- `README.md` (the consumer-wiring template — see §3).

Meta-packages — plugins that declare a `plugins = [...]` dependency list but no `[[services]]` of their own — still ship all three. Their compose is the stub:

```yaml
# Meta-package — services come from the plugins listed in plugin.toml.
services: {}
```

`localmesh/` is the canonical meta-package; its `plugins = [...]` is flattened transitively by `localmesh build`. One TOML trap: `plugins = [...]` must precede the `[identity]` table header, or it is parsed as `identity.plugins` and the loader never sees it.

This rule is documented, not currently CLI-linted.

## 2. Identity tuple

Every service in this project — app-tier or catalog plugin — declares an **identity tuple** of three values: `service_name`, `module_name`, `owned_by`. The tuple already exists in the compose files; this rule binds it.

- `service_name` — the OTel `service.name` for the running service. One per container role (`server`, `www`, `postgres`, `otel-collector`, ...). Matches whatever the OTel SDK reports.
- `module_name` — the LocalMesh namespace axis. `app` for app-tier services. `<plugin-slug>` for catalog-plugin services, where the slug matches the `localmesh/service_catalog/<plugin>/` directory name (`postgres16`, `observability`, `logging`, `security`, `auth`, `localmesh`). The single value `app` is reserved for app-tier services. Plugin slugs cannot be `app`.
- `owned_by` — contact email for ownership. App-tier uses `${TECH_LEAD_EMAIL}` (sourced from `.env`, mirroring `project.toml`'s `tech_lead_email`). Plugins use the platform team's email.

Each value lives in two places. `plugin.toml` is now the source of truth for the per-plugin halves; the `.env` interpolation bridge written by `localmesh build` (`internal/envwriter`) is the working substitute for a proper compose-extension (tracked as the *Identity source consolidation* Open item in [`../../stakeholder/DESIGN_DECISIONS.md`](../../stakeholder/DESIGN_DECISIONS.md)). The two channels carrying identity:

- `metrics.service_name` / `metrics.module_name` / `metrics.owned_by` docker labels — Vector reads these to enrich log events. See [`./logging-platform.md`](./logging-platform.md).
- `service.name` (via `OTEL_SERVICE_NAME`) and `module_name` / `owned_by` (via `OTEL_RESOURCE_ATTRIBUTES`) — the OTel SDK reads these for traces and metrics.

`service.namespace` is also part of OTel resource attributes but is tenant-level identity (interpolates from `${PROJECT_NAME}`), not module-level. It is not part of the identity tuple this rule binds; it is a separate axis carried on every app-tier service's `OTEL_RESOURCE_ATTRIBUTES`.

### Example — app-tier service

```yaml
server:
  environment:
    OTEL_SERVICE_NAME: server
    OTEL_RESOURCE_ATTRIBUTES: "service.namespace=${PROJECT_NAME},deployment.environment.name=dev,module_name=app,owned_by=${TECH_LEAD_EMAIL}"
  labels:
    metrics.service_name: server
    metrics.module_name: app
    metrics.owned_by: ${TECH_LEAD_EMAIL}
```

### Example — catalog plugin service (`postgres16/`)

```yaml
postgres:
  environment:
    OTEL_SERVICE_NAME: postgres
    OTEL_RESOURCE_ATTRIBUTES: "module_name=postgres16,owned_by=sre@example.com"
  labels:
    metrics.service_name: postgres
    metrics.module_name: postgres16
    metrics.owned_by: sre@example.com
```

## 3. Consumer wiring (`README.md`)

Every catalog plugin with a consumer-facing surface ships a `README.md` next to its `docker-compose.yml`. The file follows this strict template. Consumer apps include the plugin, read `README.md`, copy-paste the wiring into the relevant app-tier service.

No new YAML keys, no compose `extends:`, no build step. Discipline lives in the docs; compliance is documented now and lintable later.

### Template

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
`metrics.*` and `katenary.v3/*` labels the consumer must add specifically for this plugin. Identity-tuple labels the app already declares (per §2 above) are mentioned only if the plugin needs a specific value.

## k8s rendering
katenary labels for secret promotion / `values-from` references. `None` if the plugin exposes no sensitive env.

## Notes
Free-form. Format contracts (logging's JSON envelope), runtime obligations (logging's emit-to-stdout requirement), trace-context expectations (database's `/*traceparent='…'*/` SQL comment), caveats, links to deeper specs.

## Example app service block
Drop-in YAML composing every section above into one consumer service definition. Copy-paste-ready.
```

Sections that do not apply for a given plugin write `None` rather than being omitted — the dev's eye lands at a predictable spot every time.

Worked example for the `postgres16/` plugin lives at [`../../../localmesh/service_catalog/postgres16/README.md`](../../../localmesh/service_catalog/postgres16/README.md).

## 4. Overlap with `plugin.toml`

`plugin.toml` owns identity (`module_name`, `owned_by`), service definitions (`container`, `port`, `scheme`), exposure (`expose_via_ingress`, `ingress`), and auth gating (`requires_auth`, default `true` — services that opt out of the ingress auth gate set `requires_auth = false`; the IdP itself is the canonical opt-out). `README.md` cites these values by reference (link to §2 above or to `plugin.toml` itself) and never redeclares them. Plugin-internal env vars derived from `plugin.toml` by `localmesh build` do not appear in `README.md`'s Environment table:

- From `[identity]`, keyed by **plugin slug**: `<PLUGIN>_MODULE_NAME`, `<PLUGIN>_OWNED_BY`.
- From `[[services]]`, keyed by **container slug**: `<CONTAINER>_PORT`, `<CONTAINER>_EXPOSE_VIA_INGRESS`, `<CONTAINER>_INGRESS`.

Mesh membership is the `needs_mtls_sidecar` field in each `[[services]]` entry (default `true`; `false` opts a service out of the mesh data plane). `kuma-cp`, `ingress`, `dex`, `postgres`, and `postgres-exporter` set it to `false`. The CLI reads the field at build time to populate `.Services[*].Meshed` in the template context; the `security/` plugin's compose template iterates that list to emit sidecars.

`README.md`'s Environment table is for env vars the **consumer app** sets, not values exported into the platform's `.env`.

### 4.1. `[[config_vars]]` — plugin-author knobs

Each `[[config_vars]]` entry is a consumer-tunable default the plugin author publishes. Fields are all required and all single-line strings (`name`, `description`, `value`). Names are unique within the plugin. The default `value` lives in `plugin.toml`; `deploy.toml` overrides per instance.

Resolved values land in `.env` keyed by **plugin slug + config name**: `<UPCASE plugin>_<UPCASE name>`. So `redis/`'s `maxmemory` becomes `REDIS_MAXMEMORY`. The plugin's compose interpolates the resolved value via `${REDIS_MAXMEMORY}`. Phase 2 will switch the prefix to the per-instance `service_name` when multi-instance lands.

### 4.2. `[[exports]]` — values the plugin delivers to consumers

Each `[[exports]]` entry is one consumer-facing env var the plugin publishes. Required fields are `name` (unique), `template` (the value, e.g. `redis://{{ .Service.Container }}:{{ .Service.Port }}`), and `env` (the destination env-var name template, e.g. `{{ .ServiceName | upper }}_URL`). Optional: `description`, `required` (default `false`). Validation is asymmetric to `[[config_vars]]`: `description` is **not** validated on exports (it's documentation for plugin authors, not a runtime contract).

Templates render through `text/template` with `Option("missingkey=error")` and one registered function (`upper`). No Sprig — the export-render path is intentionally narrower than the `.gotmpl` compose path (§5). The template `.` is an `exportCtx` with `.Project`, `.Plugin`, `.ServiceName`, `.Service` (singular — see below), and `.ConfigVars` (the resolved per-instance config map).

`deploy.toml` may override `env` per instance (`[plugins.<slug>.exports] name = "<DEST>"`) to rename the destination. The plugin author owns the value; the consumer owns the destination name.

### 4.3. `config_vars` and `exports` share one namespace

Within a plugin, a `[[config_vars]].name` and a `[[exports]].name` cannot collide. The CLI rejects it at `LoadPlugin`. This keeps the per-plugin namespace flat so the `deploy.toml` author isn't guessing which array a given name belongs to.

### 4.4. Phase 1: `.Service` is singular

A plugin with multiple `[[services]]` entries still gets a single `.Service` in the export-render context — the first declared service wins. Plugins that need to template against a non-first service should keep export templates simple (or wait for Phase 2's per-instance rendering, which keys `.Service` by `service_name`).

## 5. Template author reference

`.gotmpl`-suffixed plugin files are rendered through Go's `text/template` with the [Masterminds/sprig](https://masterminds.github.io/sprig/) function library. Plain `.yaml` / `.yml` files pass through verbatim — no templating, no substitution. The renderer runs with `Option("missingkey=error")`: referencing a key that doesn't exist fails the build loudly instead of emitting `<no value>`.

### Render context

Templates receive a `*Context` (`localmesh_src/internal/template/template.go`) with four fields:

- `.Project` — the typed `project.toml` view. Fields: `.Project.Name`, `.Project.Namespace`, `.Project.TechLead`, `.Project.ExternalDomain`, `.Project.LocalDomain`, `.Project.Plugins`, `.Project.Services` (the app-tier `[[services]]`).
- `.Plugin` — this plugin's own typed `plugin.toml`. Fields: `.Plugin.Name` (the directory-derived slug), `.Plugin.Identity.ModuleName`, `.Plugin.Identity.OwnedBy`, `.Plugin.Plugins` (meta-dependencies; empty for leaf plugins), `.Plugin.Services`.
- `.Env` — `map[string]string` of the `.env`-managed values the CLI emitted (`internal/envwriter`). This carries CA / OIDC material, not the per-plugin `<PLUGIN>_*` vars from §4 — those reach compose through `${}` interpolation at runtime, not the template map.
- `.Services` — `[]ServiceCtx` (`Name`, `Port`, `Meshed`, `ExposeViaIngress`) of every registry-known service (project `[[services]]` + every plugin `[[services]]`), sorted by name. `Meshed` is true when the service's `needs_mtls_sidecar` field is true (the default). `ExposeViaIngress` mirrors `expose_via_ingress`. Plugins that don't need the list ignore it. The `security/` template is the canonical consumer: it ranges over `.Services`, emits a `<name>-mesh` kuma-dp sidecar for each `Meshed` entry, and emits a `MeshHTTPRoute` for each `ExposeViaIngress` entry.

### Custom functions

Two functions are registered on top of sprig (`localmesh_src/internal/template/funcs.go`):

- `spiffeURI <container> <project> <local_domain>` → `spiffe://<container>.<project>.<local_domain>`. Built from the container name, never the plugin slug — renames like `database` → `postgres16` do not move the SPIFFE identity.
- `identityLabels <plugin> <service_name>` → a three-key YAML block (`metrics.service_name`, `metrics.module_name`, `metrics.owned_by`) drawn from the plugin's identity tuple. The caller indents the result.

### Data-source discipline

A template may only read from the context fields above. Reaching into a sibling plugin's files, scraping a runtime artifact, or parsing labels meant for another consumer is forbidden (§0). If the value you need isn't on the context, stop and ask — do not invent an ingestion path.

## See also

- [`./katenary-top-seven.md`](./katenary-top-seven.md) — the `katenary.v3/*` labels referenced from README.md examples.
- [`./logging-platform.md`](./logging-platform.md) — the JSON envelope contract apps must emit; depends on the identity-tuple labels above.
