# observability_cleanroom (Jaeger experiment)

Experiment branch: Jaeger replaces Grafana as the UI. Stack is Jaeger + Alloy + Prometheus + Loki + Pyroscope + cadvisor, driven by a synthetic noise generator (`telemetrygen`). Side-by-side with the LocalMesh `service_catalog/observability/` and `service_catalog/logging/` plugins. Does not replace them.

Jaeger is trace-first. It views traces and, via its SPM "Monitor" tab, RED metrics read from Prometheus. It has no UI for logs or profiles. Loki and Pyroscope still ingest, but there is no pane of glass for them in this variant. That gap is the experiment's main finding.

Localdev only. No auth, no TLS, no retention. Spec: `docs/superpowers/specs/2026-05-21-jaeger-ui-swap-design.md`.

## Run

```bash
cd observability_cleanroom
docker compose up -d
```

Compose v2.20+ required (native `include:`).

## Endpoints

Host ports are "1"-prefixed to avoid collisions with other dockerized workloads. The two 5-digit UIs (Alloy, Jaeger) keep native ports because a "1" prefix overflows 65535.

| Service | URL | Notes |
|---|---|---|
| Jaeger UI | http://localhost:16686 | Traces + Monitor (SPM) tab |
| Alloy UI | http://localhost:12345 | Pipeline graph, debug |
| Prometheus | http://localhost:19090 | PromQL |
| Pyroscope | http://localhost:14040 | Native UI (no Jaeger pane) |
| OTLP gRPC | localhost:14317 | External app push |
| OTLP HTTP | localhost:14318 | External app push |

## Architecture

Alloy is the only collection agent. Only its trace exporter changed (Tempo → Jaeger). Everything else is unchanged from the suite variant.

```
noise-gen (OTLP) ─▶ Alloy ─┬─▶ Jaeger      (traces; own memory store; UI :16686)
                           ├─▶ Prometheus  (metrics + spanmetrics + servicegraph + host + cadvisor + blackbox)
                           ├─▶ Loki        (logs; ingesting, no UI)
                           └─▶ Pyroscope   (eBPF profiles; ingesting, no UI)

Jaeger Monitor tab ──reads RED (calls_total / duration_*)── Prometheus
```

Alloy's spanmetrics connector runs with an empty namespace so series are `calls_total` / `duration_*` — the names Jaeger SPM queries (with `normalize_calls` / `normalize_duration`).

## Smoke check

1. `docker compose ps` — 9 services up (no grafana, no tempo; jaeger present).
2. http://localhost:12345 — Alloy UI, no red component nodes.
3. http://localhost:16686 → Search → service `noisegen-traces` returns traces.
4. http://localhost:16686 → Monitor tab → RED panels populate for `noisegen-traces`.
5. Logs (headless): `curl 'http://localhost:19090/...'` shows spanmetrics; Loki still ingests via Alloy.

## Tear down

```bash
docker compose down -v
```

Volumes are not preserved. Each run starts fresh.

## What this experiment shows

Jaeger covers traces and RED (via SPM from Prometheus) with a trace-first UI. It does not cover logs or profiles — those backends keep ingesting but have no viewer here. So Jaeger can stand in for the trace-and-RED slice of Grafana, but not for the unified four-signal pane. Swapping Grafana for Jaeger is a trade, not a drop-in.

## GPU metrics

Scaffolded but disabled. See the commented DCGM block in `alloy/config.alloy`. Enable on a host with NVIDIA GPUs + the Container Toolkit.
