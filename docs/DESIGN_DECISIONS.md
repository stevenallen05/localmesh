# Design Decisions

Current state of every important design choice, one line each. History in git. Technical detail (proto, SQL, env vars, code sketches) lives in [`docs/superpowers/specs/`](./superpowers/specs/).

> **Prod note:** in real life this is Grafana + OpenTelemetry + Loki. Choices below are scoped to the take-home.

## Settled

| Area | Choice |
|---|---|
| Orchestration | `docker compose`; Helm via katenary as a derivative |
| Agent language | Go (Docker SDK, cross-compile, `gopsutil`) |
| Agent collectors | `docker_stats`, `host`, `cloudflared`, `docker_logs` — pluggable |
| Agent config | TOML + env override via Koanf; precedence defaults < TOML < env |
| Collector toggle | Startup-only 3-state: `on` / `off` / `cel` (placeholder, stub today) |
| Reporting model | Server-initiated via bidi gRPC stream |
| Buffer policy | `MAX_BUFFER_SIZE` env (0 = unlimited, pause-on-full); `MIN_REPORTING_INTERVAL` env (default 300s, agent-side floor) |
| gRPC services | `MetricsIngest.Connect` (bidi, agent) + `MetricsQuery` (web) — split by access profile |
| Report shape | Single `Report` with `repeated ReportItem { oneof MetricSample \| LogEntry }` |
| Server | Rust + tonic; stateless (all durable state in Postgres) |
| Storage | TimescaleDB; unified `metrics` hypertable + separate `logs` hypertable; continuous aggregates as rollups |
| Log filter UI | Structured form + `ILIKE`; no DSL. `query_dsl` slot reserved on the wire. |
| Logs UI | Dedicated tab |
| User attribution | Forward-auth header passthrough, configurable, **not enforced** |
| Live demo | CloudFlare Zero Trust tunnel + Access |
| Agent identity | Config value `agent.id`; defaults to hostname |
| gRPC LB | **Deferred for take-home.** In prod: forward-auth + mTLS required; specific LB and auth-provider choice out of scope. |
| Containers | Short single-stage Dockerfiles (`./agent/Dockerfile`, `./server/Dockerfile`, `./www/Dockerfile`); minimal compose (katenary defaults, no extra hints unless conversion fails). Hardening (multi-stage, distroless, non-root USER, healthchecks) **TODO'd** pending infra/compliance decisions. |
| First-run cost | Minimized — single-stage builds, common base images, no upfront hardening. Cold start ~3–6 min, dominated by TimescaleDB pull + `cargo build --release`. |
| Metrics dashboard | **Basic HTML tables only** for V1. `TODO` comments mark where graphs/charts would expand. |
| Tests | `go test` in `agent/`, `cargo test` in `server/`, `npm test` in `www/`. High-value tests only. |
| Commit hygiene | Test changes and code changes **never** share a commit. Code commits require a green test tree. |
| Bonuses | All six in scope |

## Open

_None — all V1 decisions settled. New decisions surfaced during implementation get added back here._

## Deviations from `requirements.md`

- **Go agent** instead of Python — Core Requirements section allows "any language (Rust, Python, Go, etc.)" which supersedes the Tasks-section "python" bullet.
