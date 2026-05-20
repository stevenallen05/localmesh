# Plugin conventions

Three conventions that bind every catalog plugin and every app-tier service in this project. They are not optional. New plugins land on the same shape; new app-tier services declare the same identity.

The conventions exist informally in the current compose files. This file makes them canonical so plugin authors and app teams stop pattern-matching from neighbouring code.

Long-form rationale: [`../../superpowers/specs/2026-05-18-localmesh-namespacing-and-plugin-exports-design.md`](../../superpowers/specs/2026-05-18-localmesh-namespacing-and-plugin-exports-design.md).

## 0. Data-source rule (highest priority)

**I will define the variables and files you are permitted to pull datapoints from. If I have not told you explicitly where to get the data, you must stop immediately.**

A plugin or tool reads only the inputs explicitly designated for it. Compose files are runtime, not build inputs. Labels carry meaning for their declared consumer only — do not scrape them from another layer. When data isn't where you expected, do not invent an ingestion path. Stop, surface the question, and wait for direction.

This rule is repeated in `CLAUDE.md`, `docs/engineering/rules/golang-basics.md`, and `docs/engineering/rules/rust-basics.md`.

## 1. Identity tuple

Every service in this project — app-tier or catalog plugin — declares an **identity tuple** of three values: `service_name`, `module_name`, `owned_by`. The tuple already exists in the compose files; this rule binds it.

- `service_name` — the OTel `service.name` for the running service. One per container role (`server`, `www`, `postgres`, `otel-collector`, ...). Matches whatever the OTel SDK reports.
- `module_name` — the LocalMesh namespace axis. `app` for app-tier services. `<plugin-slug>` for catalog-plugin services, where the slug matches the `service_catalog/<plugin>/` directory name (`database`, `observability`, `logging`, `mesh`, `auth`). The single value `app` is reserved for app-tier services. Plugin slugs cannot be `app`.
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

### Example — catalog plugin service (`database/`)

```yaml
postgres:
  environment:
    OTEL_SERVICE_NAME: postgres
    OTEL_RESOURCE_ATTRIBUTES: "module_name=database,owned_by=sre@example.com"
  labels:
    metrics.service_name: postgres
    metrics.module_name: database
    metrics.owned_by: sre@example.com
```

## 2. Consumer wiring (`README.md`)

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
`metrics.*` and `katenary.v3/*` labels the consumer must add specifically for this plugin. Identity-tuple labels the app already declares (per §1 above) are mentioned only if the plugin needs a specific value.

## k8s rendering
katenary labels for secret promotion / `values-from` references. `None` if the plugin exposes no sensitive env.

## Notes
Free-form. Format contracts (logging's JSON envelope), runtime obligations (logging's emit-to-stdout requirement), trace-context expectations (database's `/*traceparent='…'*/` SQL comment), caveats, links to deeper specs.

## Example app service block
Drop-in YAML composing every section above into one consumer service definition. Copy-paste-ready.
```

Sections that do not apply for a given plugin write `None` rather than being omitted — the dev's eye lands at a predictable spot every time.

Worked example for the `database/` plugin lives at [`../../../service_catalog/database/README.md`](../../../service_catalog/database/README.md).

## 3. Overlap with `plugin.toml`

`plugin.toml` owns identity (`module_name`, `owned_by`), service definitions (`container`, `port`, `scheme`), exposure (`expose_via_ingress`, `ingress`), and auth gating (`requires_auth`, default `true` — services that opt out of the ingress auth gate set `requires_auth = false`; the IdP itself is the canonical opt-out). `README.md` cites these values by reference (link to §1 above or to `plugin.toml` itself) and never redeclares them. Plugin-internal env vars derived from `plugin.toml` by `localmesh build` do not appear in `README.md`'s Environment table:

- From `[identity]`, keyed by **plugin slug**: `<PLUGIN>_MODULE_NAME`, `<PLUGIN>_OWNED_BY`.
- From `[[services]]`, keyed by **container slug**: `<CONTAINER>_PORT`, `<CONTAINER>_EXPOSE_VIA_INGRESS`, `<CONTAINER>_INGRESS`.

Mesh-exempt status (`mesh.exempt: "true"` compose label) is declared on each service's compose `labels:` block, not in `plugin.toml`. The localmesh CLI's `internal/mesh/` package parses each plugin's compose to derive the per-container exempt set, which drives cert-skip in `mtls mint` (TODO; see `docs/TODO.md`) and edge filtering in the envoy render path.

`README.md`'s Environment table is for env vars the **consumer app** sets, not values exported into the platform's `.env`.

## See also

- [`./katenary-top-seven.md`](./katenary-top-seven.md) — the `katenary.v3/*` labels referenced from README.md examples.
- [`./logging-platform.md`](./logging-platform.md) — the JSON envelope contract apps must emit; depends on the identity-tuple labels above.
