# observability_cleanroom

Parallel exploration of a unified observability suite: Grafana + Alloy + Prometheus + Tempo + Loki + Pyroscope + cadvisor, driven by a synthetic noise generator (`telemetrygen`). Side-by-side with the LocalMesh `service_catalog/observability/` and `service_catalog/logging/` plugins. Does not replace them.

Localdev only. No auth, no TLS, no retention. Spec: `docs/superpowers/specs/2026-05-20-observability-cleanroom-design.md`.

## Run

```bash
cd observability_cleanroom
docker compose up -d
```

Compose v2.20+ required (native `include:`).

## Endpoints

| Service | URL | Notes |
|---|---|---|
| Grafana | http://localhost:3001 | admin/admin |
| Alloy UI | http://localhost:12345 | Pipeline graph, debug |
| Prometheus | http://localhost:9090 | PromQL |
| Pyroscope | http://localhost:4040 | Native UI |
| OTLP gRPC | localhost:4317 | External app push |
| OTLP HTTP | localhost:4318 | External app push |

## Architecture

Alloy is the only collection agent. It terminates OTLP from the noise generator, runs spanmetrics + servicegraph connectors, tails container logs into Loki, scrapes its embedded host + blackbox exporters and cadvisor into Prometheus, and runs the eBPF profiler into Pyroscope.

```
noise-gen (OTLP) ─▶ Alloy ─┬─▶ Tempo      (traces)
                           ├─▶ Prometheus (metrics + span/servicegraph + host + cadvisor + blackbox)
                           ├─▶ Loki       (logs: container stdout + OTLP)
                           └─▶ Pyroscope  (eBPF profiles)
                                  │
                                  ▼
                               Grafana
```

## Smoke check

1. `docker compose ps` — all 9 services up.
2. http://localhost:12345 — Alloy UI, no red component nodes.
3. http://localhost:3001 → Connections → Data sources — Prometheus, Tempo, Loki, Pyroscope all green.
4. Grafana → Explore → each datasource → run any query → returns data within 30s.

## Tear down

```bash
docker compose down -v
```

Volumes are not preserved. Each run starts fresh.

## What this proves

One Grafana Alloy process replaces the split agent topology (otel-collector + Vector + standalone exporters) and feeds a Grafana-stack backend with all four signals plus host/container/synthetic metrics. The cleanroom evaluates the stack, not a realistic workload. The noise generator emits synthetic OTLP data with no inter-service correlation.

## GPU metrics

Scaffolded but disabled. See the commented DCGM block in `alloy/config.alloy`. Enable on a host with NVIDIA GPUs + the Container Toolkit.
