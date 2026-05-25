# Plugin authoring

A plugin is a catalogue entry: a `plugin.toml` manifest, a compose file, and a `README.md`, all living under `localmesh/service_catalog/<plugin>/`.

## Writing a plugin

The manifest has four sections.

**`[identity]`** — who owns it and how LocalMesh names it.

**`[[services]]`** — one entry per container the plugin runs. Required fields: `container`, `port`, `scheme`, `expose_via_ingress`. `scheme` is one of `grpc | http | https | tcp | postgresql` (the app's loopback bind protocol). The field is marked for removal in [`CLEANUP.md`](../CLEANUP.md) §1 but is still required today.

**`[[config_vars]]`** — defaults the consumer may override in `deploy.toml`. Each entry has `name`, `description`, and `value`. The resolved value lands in `.env` as `<UPCASE plugin>_<UPCASE name>` — so `redis`'s `maxmemory` becomes `REDIS_MAXMEMORY`. The plugin's compose interpolates it via `${REDIS_MAXMEMORY}`.

**`[[exports]]`** — values the plugin delivers to the consumer. Each entry has `name`, `template` (renders the value), and `env` (renders the default destination env-var name). `required` is optional and defaults to `false`. The consumer renames the destination in `deploy.toml`.

Full field semantics for both sections: `docs/engineering/rules/plugin-conventions.md` §4.

### Example — `redis/plugin.toml`

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

## Using a plugin

List it in `project.toml`:

```toml
plugins = ["localmesh", "redis"]
```

Override config defaults and rename export destinations in `deploy.toml`:

```toml
[[plugins.redis]]
  service_name = "cache"
  [plugins.redis.config]
  maxmemory = "512mb"
  [plugins.redis.exports]
  connection_url = "CACHE_URL"    # renames the default REDIS_URL → CACHE_URL in .env
```

A plugin with no consumer-tunable `[[config_vars]]` or `[[exports]]` does not need a `[[plugins.<slug>]]` block in `deploy.toml`.

## Build and run

```sh
localmesh build
docker compose up -d
```
