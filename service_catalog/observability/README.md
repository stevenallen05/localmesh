# Consumer wiring — `observability/`

## Overview

Off-the-shelf OpenTelemetry collector + Grafana + Loki + Victoria Metrics + Tempo. Consumer apps emit OTLP via standard SDK env vars; the collector translates and forwards to the right backend per signal. The plugin self-ships APM + cluster-health dashboards into Grafana with no consumer action required.

## Environment

| Name | Sample value | Source |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4318` | plugin |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` | plugin |
| `OTEL_SERVICE_NAME` | `server` | app, per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) |
| `OTEL_RESOURCE_ATTRIBUTES` | `service.namespace=${PROJECT_NAME},deployment.environment.name=dev,module_name=app,owned_by=${TECH_LEAD_EMAIL}` | app, per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) |
| `OTEL_SEMCONV_STABILITY_OPT_IN` | `http` | app (Node-only — the legacy `http.server.duration` metric is renamed to `http.server.request.duration` on opt-in; required for the APM dashboard's RED panels. Rust SDKs use stable semconv by default) |

## depends_on

```yaml
depends_on:
  otel-collector:
    condition: service_started
```

## Labels

None plugin-specific. App-tier identity tuple is required per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) — observability adds nothing on top.

## k8s rendering

None today. The bearer-token path is file-mounted from a named volume; no `katenary.v3/secrets` or `values-from` is wired in this iteration. The volume becomes a `Secret` mount once a real secrets backend lands.

## Notes

- Per-stack OTel init recipes live alongside the code: [`../../server/src/telemetry.rs`](../../server/src/telemetry.rs) (Rust) and [`../../www/instrumentation.ts`](../../www/instrumentation.ts) (Node). Both read the env vars above; no per-service tracing setup.
- `OTEL_SEMCONV_STABILITY_OPT_IN=http` is Node-only. Rust's `opentelemetry-rust` SDK uses stable semconv by default.
- `OTEL_RESOURCE_ATTRIBUTES`'s `service.namespace` and `owned_by` interpolate from the project's `.env` (`PROJECT_NAME`, `TECH_LEAD_EMAIL`) — keep them as `${…}` rather than hardcoding so the chart-gen step picks up overrides.

## Example app service block

```yaml
services:
  myservice:
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4318
      OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
      OTEL_SERVICE_NAME: myservice
      OTEL_RESOURCE_ATTRIBUTES: "service.namespace=${PROJECT_NAME},deployment.environment.name=dev,module_name=app,owned_by=${TECH_LEAD_EMAIL}"
      # Node only:
      # OTEL_SEMCONV_STABILITY_OPT_IN: http
    depends_on:
      otel-collector:
        condition: service_started
```
