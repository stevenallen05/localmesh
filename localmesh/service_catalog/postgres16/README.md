# Consumer wiring — `postgres16/`

## Overview

Postgres 16 with DataDog's `pg_tracing` extension. Consumers connect via the `DATABASE_URL` export (renamed in `deploy.toml`). pg_tracing emits parse/plan/exec spans to the OTel collector; consumers stitch them under their own trace via a `/*traceparent='…'*/` SQL comment.

## Environment

| Name | Sample value | Source |
|---|---|---|
| `DATABASE_URL` | `postgresql://app:app@postgres:5432/app` | plugin (renamed via `deploy.toml`) |

Demo credentials (`app/app/app`) are public-to-project. Production overlay swaps to a secrets-backend lookup.

## Volumes

None.

## depends_on

```yaml
depends_on:
  postgres:
    condition: service_healthy
```

## Labels

None.

## k8s rendering

None.

## Notes

SQL the consumer sends should include `/*traceparent='{trace_id}-{span_id}-{flags}'*/` as a comment so pg_tracing's spans inherit the active OTel context. Plain `psql` does not inject this — its spans appear in Tempo but as orphans (no parent). The Rust server's `sqlx` extension layer (when re-wired) injects automatically.

## Example app service block

```yaml
services:
  myservice:
    environment:
      DATABASE_URL: ${DATABASE_URL}
    depends_on:
      postgres:
        condition: service_healthy
```
