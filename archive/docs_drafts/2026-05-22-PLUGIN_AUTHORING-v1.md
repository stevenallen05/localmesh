# Authoring a LocalMesh plugin

This guide teaches the shape of a plugin by walking `redis/`, the smallest live example. After reading it, you should be able to fork that directory, swap in a different image, and have a working plugin.

The schema, identity-tuple spec, README template, and lint checklist live in [`./rules/plugin-conventions.md`](./rules/plugin-conventions.md). This guide does not repeat them. When the conventions doc says "§4.2", land there for the field-by-field reference and come back.

## What a plugin is

A plugin packages one runtime concern (a cache, a database, a sidecar mesh, an observability stack) so the platform owner adds it to a project by listing its slug in `project.toml`:

```toml
plugins = ["observability", "redis"]
```

Each slug resolves to `localmesh/service_catalog/<slug>/`. `localmesh build` flattens the list, validates every plugin's manifest, and emits a single `localmesh/bundled.compose.yaml`. The user runs `docker compose -f localmesh/bundled.compose.yaml up`. The CLI is not in the runtime path.

The plugin interface is deliberately small. Three files. No registration call. No build hooks. Add a directory, add a slug, run build.

## The three files

Every plugin ships exactly three files at `localmesh/service_catalog/<slug>/`:

| File | What it is |
|---|---|
| `plugin.toml` | Manifest. Identity, services, knobs, exports. |
| `docker-compose.yml` (or `.gotmpl`) | The container(s) the plugin runs. |
| `README.md` | The copy-paste consumer-wiring template. |

### `plugin.toml`

Redis declares its full surface in 30 lines.

```toml
[identity]
module_name = "redis"
owned_by    = "sre@example.com"

[[services]]
container          = "redis"
port               = 6379
scheme             = "tcp"
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

Four blocks, four jobs:

- `[identity]` — the plugin's namespace axis. `module_name` defaults to the slug. `owned_by` is the on-call email.
- `[[services]]` — every container role the plugin runs. Each entry needs a `scheme` (one of `grpc`, `http`, `https`, `tcp`, `postgresql`); the CLI rejects anything else.
- `[[config_vars]]` — knobs a consumer may tune. Each one resolves to `<UPCASE plugin>_<UPCASE name>` in the managed `.env` block (so `REDIS_MAXMEMORY` here). The plugin's compose interpolates `${REDIS_MAXMEMORY}` at runtime.
- `[[exports]]` — values the plugin delivers to consumer apps. `template` renders the value. `env` renders the destination env-var name. The consumer renames the destination from `deploy.toml`.

A meta-package — a plugin that depends on other plugins instead of shipping containers of its own — adds one extra top-level field: `plugins = ["a", "b"]`. Its `[[services]]` array is empty. `base/` is the canonical example. The TOML trap: `plugins = [...]` must appear **before** any `[identity]` table header, or it parses as `identity.plugins` and the loader never sees it.

### `docker-compose.yml`

Plain compose. The plugin owns the container, the healthcheck, the labels.

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
      timeout: 3s
      retries: 5
    labels:
      metrics.service_name: redis
      metrics.module_name:  redis
      metrics.owned_by:     sre@example.com
```

Two points worth knowing:

- The identity-tuple labels duplicate values from `plugin.toml`. Vector reads the labels at runtime; the platform reads `plugin.toml` at build time. The duplication is the cost of not yet having compose extensions wired (Open item in `docs/stakeholder/DESIGN_DECISIONS.md`).
- The healthcheck is real. Consumers `depends_on: { redis: { condition: service_healthy } }`. A placeholder healthcheck silently breaks dependent boot order. If your plugin's base image is distroless (no `wget`, `curl`, or `/bin/sh`), drop the healthcheck and have dependents fall back to `service_started` — see the observability plugin's alloy + tempo for the worked case.

If your plugin needs to template — iterate over user services, inject env vars, read render-context fields — rename to `docker-compose.yml.gotmpl`. The render context is documented in `plugin-conventions.md §5`. The observability plugin's `range .Project.Services` block is the canonical example of injecting env vars onto every user service.

### `README.md`

The consumer-wiring template. The redis one is 50 lines. That is the right ceiling for most plugins. Every plugin's README has the same eight sections in the same order: Overview, Environment, Volumes, depends_on, Labels, k8s rendering, Notes, Example. Sections that do not apply write `None` and stay in place — the consumer's eye lands at a predictable spot each time.

The template lives in `plugin-conventions.md §3`. Do not improvise. Do not omit sections.

## What `localmesh build` does

Four phases. None of them are user-extensible.

1. **`manifest.LoadAll`** — reads `project.toml`, expands `plugins = [...]` (with transitive deps from any meta-package), and parses every named plugin's `plugin.toml`. Schema validation runs here: container collisions, identity-tuple shape, `config_vars`/`exports` name collisions inside one plugin.
2. **`envwriter.WriteManaged`** — writes the managed block of `.env`. CA material, OIDC client secret, per-plugin `<PLUGIN>_<NAME>` config vars, per-service `<CONTAINER>_PORT` and friends.
3. **`render.Run`** — for each plugin, reads `docker-compose.{yaml,yml}{,.gotmpl}`, renders the template, rewrites relative paths from the plugin dir into the bundle output dir, and deep-merges into `localmesh/bundled.compose.yaml`. Mappings deep-merge last-wins. Sequences concat. Scalar conflicts fail the build loudly.
4. **`devref.Write`** — generates `localmesh/developer_reference.md` from `plugin.toml` exports. This is what the consumer team reads alongside each plugin's `README.md` to wire their app.

A successful build prints nothing useful. A failed build prints the validation error and exits non-zero.

## Verifying your plugin

After dropping your plugin into `localmesh/service_catalog/<slug>/` and adding `"<slug>"` to `project.toml`'s `plugins` array, run:

```
go run ./localmesh_src/cmd/localmesh build
docker compose -f localmesh/bundled.compose.yaml config --quiet
docker compose -f localmesh/bundled.compose.yaml up -d
docker compose -f localmesh/bundled.compose.yaml ps
```

`config --quiet` catches malformed YAML before docker tries to start anything. `ps` should show every container `running (healthy)` — anything stuck at `starting` is usually a wrong healthcheck command. The four common build-time failures are listed at the bottom of `plugin-conventions.md`.

## Reference shelf

- [`./rules/plugin-conventions.md`](./rules/plugin-conventions.md) — schema, identity-tuple spec, README template, template author reference, lint checklist.
- [`./rules/katenary-top-seven.md`](./rules/katenary-top-seven.md) — the `katenary.v3/*` labels referenced from README templates.
- [`./rules/logging-platform.md`](./rules/logging-platform.md) — the JSON envelope the platform consumes.
- `docs/stakeholder/DESIGN_DECISIONS.md` — why the shape is what it is.
