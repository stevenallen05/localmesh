# Consumer wiring — `redis/`

## Overview

Redis 7. Joins the mesh via the per-workload sidecar; the consumer dials it over the mesh, with mTLS terminated transparently. No password in dev — the mesh boundary is the auth. `TODO: needs_prod_decisions Redis AUTH/ACL + secret delivery`.

## Environment

| Name | Sample value | Source |
|---|---|---|
| `{{.ServiceName \| upper}}_URL` (default) | `redis://redis:6379` | plugin export `connection_url` |

The consumer renames the destination by setting `[plugins.exports].connection_url = "<DEST>"` on its `deploy.toml` instance. Example: `connection_url = "CACHE_URL"` lands `CACHE_URL` in `.env`.

## Volumes

None.

## depends_on

```yaml
depends_on:
  redis:
    condition: service_healthy
```

## Labels

None plugin-specific. App-tier identity tuple is still required per `docs/engineering/rules/plugin-conventions.md`.

## k8s rendering

```yaml
labels:
  katenary.v3/secrets: |-
    - <DEST_URL>
```

## Notes

The connection_url assembled by this plugin assumes the consumer reaches Redis via the compose service name (`redis`). When Phase 2 lands multi-instance rendering, the host axis switches to a per-instance compose service name keyed by `service_name`.

## Example app service block

```yaml
services:
  myservice:
    environment:
      REDIS_URL: ${CACHE_URL}
    depends_on:
      redis:
        condition: service_healthy
```
