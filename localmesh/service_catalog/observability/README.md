# Consumer wiring — `observability/`

## Overview

LGTM stack for local dev. Loki + Grafana + Tempo + Mimir-via-Prometheus, plus Alloy as the single collection agent, plus cadvisor for container metrics. Apps with no observability config of their own appear in Grafana — topology, RED metrics, logs, container metrics — on first `docker compose up`.

This package is **omakase**: one delivery shape. The user-facing surface is three compose labels.

## Environment

This plugin emits no consumer-facing exports. Instead, it injects OTel env vars into every service declared in `project.toml [[services]]`:

| Variable | Value |
|---|---|
| `OTEL_SERVICE_NAME` | `<container>` (your service's container name) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://alloy:4317` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` |

An app that uses the OTel SDK auto-discovers these. An app that doesn't ignores them.

## Volumes

In-container tmpfs only — `/loki`, `/prometheus`, `/var/tempo`, `/var/lib/grafana`. Data does not survive `docker compose down`. `TODO: needs_prod_decisions retention + persistent volumes`.

## depends_on

Apps do **not** `depends_on` the observability stack. Observability outage is not application outage; OTel SDKs buffer and retry.

Within the stack, the dependency order is:

```yaml
alloy:
  depends_on:
    tempo:      { condition: service_started }  # distroless — no probe binary
    loki:       { condition: service_healthy }
    prometheus: { condition: service_healthy }

grafana:
  depends_on:
    tempo:      { condition: service_started }  # distroless — no probe binary
    loki:       { condition: service_healthy }
    prometheus: { condition: service_healthy }
```

## Labels

Container labels read at runtime by alloy. The CLI never reads them.

| Label | Required when | Default | Effect |
|---|---|---|---|
| `com.localmesh.scrape` | opting into scrape | unset | Set to `"true"` to enable Prometheus scrape discovery |
| `com.localmesh.scrape.port` | scrape opted in | unset | Container port exposing `/metrics`. Missing → drop (fail-closed) |
| `com.localmesh.scrape.path` | scrape opted in | `/metrics` | Path under that port |
| `com.localmesh.observability.exempt` | always optional | `"false"` | `"true"` removes the container from log tailing and scrape discovery |

The six observability containers ship `com.localmesh.observability.exempt: "true"` so alloy ignores itself.

App-tier identity tuple (`metrics.service_name`, `metrics.module_name`, `metrics.owned_by`) is still required per `docs/engineering/rules/plugin-conventions.md`.

## Container-name soft API

These container DNS names are part of this plugin's external surface — apps and other plugins may dial them by name on the docker network. Renaming is a breaking change.

| Name | Purpose |
|---|---|
| `alloy` | OTLP ingress (gRPC `:4317`, HTTP `:4318`) |
| `prometheus` | metrics store + remote-write receiver (`:9090`) |
| `tempo` | trace store + OTLP receiver (`:4317`, `:3200` HTTP) |
| `loki` | log store + OTLP receiver (`:3100`) |
| `grafana` | UI (`:3000`) |
| `cadvisor` | container metrics scrape target (`:8080`) |

## Host port plan

`"1"`-prefix on cleanroom ports, except where overflow forces native:

| Host port | Container | Notes |
|---|---|---|
| 13000 | grafana | UI; admin/admin |
| 12345 | alloy | UI + pipeline graph (5-digit; `"1"`-prefix overflows 65535) |
| 14317 | alloy | OTLP gRPC ingress (host-resident apps) |
| 14318 | alloy | OTLP HTTP ingress |
| 19090 | prometheus | UI (debug) |
| 13100 | loki | API (debug) — not currently published |
| 13200 | tempo | API (debug) — not currently published |

## Filter seam

`alloy/config.alloy` ships one `otelcol.processor.filter "policy"` block, empty by default, placed between the OTLP receiver and the batch processor. OTTL syntax. SRE distributions swap in policy at this block. Every OTLP-received signal flows through it; container log tailing, cadvisor scrape, host-metrics scrape, blackbox scrape, and the docker-label scrape pool sit outside this seam.

## k8s rendering

`TODO: needs_prod_decisions observability k8s rendering`. This plugin's compose contains privileged + bind-mount paths (docker.sock, host root, host proc/sys) that don't translate cleanly to a workload chart. Production observability is a separate cluster-scope concern.

## Notes

The cleanroom `noise-generator` is intentionally not ported. The demo arc lives on a real user app. Pyroscope is not ported either — the fourth pillar drops from this distribution.

## Example app service block
None. Apps do not wire to the observability stack directly. OTel env vars are injected by the plugin template; the scrape labels are the only consumer-side surface (see Labels above).
