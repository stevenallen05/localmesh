# Consumer wiring — `database/`

## Overview

Postgres 16 + Datadog's `pg_tracing` extension. Consumers connect via `DATABASE_URL`. pg_tracing emits parse/plan/exec spans, stitched under the consumer's trace via a `/*traceparent='…'*/` SQL comment. The plugin self-ships a Grafana dashboard folder (`/etc/grafana/dashboards/database/postgres.json`) and a sidecar metrics path (`postgres-exporter` + `database-collector`). No consumer action required for either.

## Environment

| Name | Sample value | Source |
|---|---|---|
| `DATABASE_URL` | `postgres://app:app@postgres:5432/app` | plugin |

Demo creds (`app/app/app`) are public to the project. Prod overlay swaps to a secrets-backend lookup. See [`../../docs/stakeholder/PROJECT_SCOPE.md`](../../docs/stakeholder/PROJECT_SCOPE.md).

## Volumes

None.

## depends_on

```yaml
depends_on:
  postgres:
    condition: service_healthy
```

## Labels

None plugin-specific. App-tier identity tuple is still required per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md).

## k8s rendering

```yaml
labels:
  katenary.v3/secrets: |-
    - DATABASE_URL
  katenary.v3/values-from: |-
    DATABASE_URL: database.DATABASE_URL
```

## Notes

SQL the consumer sends should include `/*traceparent='{trace_id}-{span_id}-{flags}'*/` as a comment so pg_tracing's spans inherit the active OTel context. The Rust server's sqlx extension layer does this automatically.

## Example app service block

```yaml
services:
  myservice:
    environment:
      DATABASE_URL: postgres://app:app@postgres:5432/app
    depends_on:
      postgres:
        condition: service_healthy
    labels:
      katenary.v3/secrets: |-
        - DATABASE_URL
      katenary.v3/values-from: |-
        DATABASE_URL: database.DATABASE_URL
```
