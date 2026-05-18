# Consumer wiring — `caddy/`

## Overview

Required north-south ingress for the LocalMesh stack. Terminates browser
HTTPS on port 8443; reverse-proxies to upstream services via mTLS (for
mesh-participating workloads) or plaintext (for `mesh_exempt = true`
plugins). Browser URL shape is
`https://<container>.${PROJECT_NAME}.${LOCAL_DOMAIN}:8443`.

The route table is auto-generated from every `[[services]]` entry across
`project.toml` and `service_catalog/*/plugin.toml` whose
`expose_via_ingress = true`. Re-run `make certs` to regenerate.

## Environment

| Name              | Sample value             | Source  |
|-------------------|--------------------------|---------|
| `PROJECT_NAME`    | `metrics-collector`      | plugin  |
| `LOCAL_DOMAIN`    | `lvh.me`          | plugin  |
| `CADDY_MODULE_NAME` | `caddy`                | plugin  |
| `CADDY_OWNED_BY`  | `sre@example.com`        | plugin  |

All values flow from `project.toml` / `plugin.toml` into `.env` via
`make certs`. Consumer apps reference them in the standard interpolated
positions on their compose service definitions.

## Volumes

For workloads that expose a port via the ingress and aren't mesh-exempt:

```yaml
volumes:
  - ./.secrets/certs/<container>:/run/<container>:ro
```

The directory contains `trust.ca.crt`, `id.crt`, `id.key`. Consumer code
loads `id.crt`/`id.key` for its TLS listener and trusts `trust.ca.crt`
for verifying inbound peer certs.

## depends_on

Caddy uses `depends_on` against every upstream so its routes only resolve
once upstreams are healthy:

```yaml
depends_on:
  caddy:
    condition: service_healthy
```

Consumer services do not need to declare a `depends_on` on Caddy; Caddy
is the listener, not the dependency.

## Labels

`metrics.*` labels per `docs/engineering/rules/plugin-conventions.md` §1.
No Caddy-specific labels needed on consumer services.

## k8s rendering

`katenary.v3/ports: |-\n  - 8443` on the Caddy service for chart-time
init-container probes. No additional katenary labels needed on consumers.

## Notes

- The `Caddyfile` checked into the plugin imports
  `Caddyfile.generated`, which `scripts/secrets-gen.py` writes from the
  TOML manifests. Direct edits to either file are overwritten on the
  next `make certs` run.
- Wildcard cert `*.${PROJECT_NAME}.${LOCAL_DOMAIN}` is minted for the
  `caddy` workload. `TODO: needs_prod_decisions per-host certs via
  cert-manager IngressRoute in prod`.
- Caddy admin API (`localhost:2019`) is exposed only on the docker host's
  loopback — `step certificate install` doesn't need it, but it's there
  for diagnostics (`docker compose exec caddy curl http://localhost:2019/config/`).

## Example app service block

```yaml
services:
  www:
    image: …
    volumes:
      - ./.secrets/certs/www:/run/www:ro
    environment:
      WWW_PORT: ${WWW_PORT}              # picked up by https.createServer
    healthcheck:
      test: ["CMD", "wget", "--no-check-certificate", "-qO-", "https://localhost:${WWW_PORT}/"]
```
