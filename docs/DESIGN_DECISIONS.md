# Design Decisions

Current state of every important design choice, one line each. History in git. Technical detail (proto, SQL, env vars, code sketches) lives in [`docs/superpowers/specs/`](./superpowers/specs/).

> **Prod note:** in real life this is Grafana + OpenTelemetry + Loki. Choices below are scoped to the take-home.

## Settled

| Area | Choice |
|---|---|
| Orchestration | `docker compose` is the source of truth; **katenary** auto-derives Helm charts via pre-commit hook (CI in prod). Compose is fast to author and review — well-suited to small teams, or large teams operating under Conway's law where each service owns its own deploy story. Hand-rolled charts are daunting by comparison. Mapping services to managed offerings (DBs → RDS/Cloud SQL) and minimum resource configs is a separate exercise driven by prod infra factors. |
| Agent language | Go (Docker SDK, cross-compile, `gopsutil`) |
| Agent collectors | `docker`, `host`, `cloudflared`, `logs` — pluggable (V1: hardcoded `if cfg.collectors.X.enabled` branches) |
| Agent config | **All** env config flows through Koanf (no raw `os.Getenv`). [`agent/agent.toml`](./agent/agent.toml) is the canonical defaults file; env vars prefixed `AGENT_` override. Precedence: defaults < TOML < env. |
| Collector toggle | Startup-only 3-state: `on` / `off` / `cel` (placeholder, stub today) |
| Reporting model | Server-initiated via bidi gRPC stream |
| Buffer policy | `AGENT_MAX_BUFFER_SIZE` (0 = unlimited, pause-on-full); `AGENT_MIN_REPORTING_INTERVAL` (default 300s, agent-side floor) |
| gRPC services | `MetricsIngest.Connect` (bidi, agent) + `MetricsQuery` (web) — split by access profile |
| Report shape | Single `Report` with `repeated ReportItem { oneof MetricSample \| LogEntry }` |
| Server | Rust + tonic; stateless (all durable state in Postgres). v0 ships a single `Greeter.SayHello` to exercise the NextJS-server → Rust path; `MetricsIngest` / `MetricsQuery` land in their own spec. |
| Storage | TimescaleDB; unified `metrics` hypertable + separate `logs` hypertable; continuous aggregates as rollups. v0 compose runs the image with no schema and no server connection — schema + lazy connect land with `MetricsIngest`. |
| Log filter UI | Structured form + `ILIKE`; no DSL. `query_dsl` slot reserved on the wire. |
| Logs UI | Dedicated tab |
| User attribution | Forward-auth header passthrough, configurable, **not enforced** |
| Live demo | CloudFlare Zero Trust tunnel + Access |
| Agent identity | Config value `agent.id`; defaults to hostname |
| gRPC LB | **Deferred for take-home.** v0 wires the NextJS Server Component straight to the Rust server via `@grpc/grpc-js` — server-side only, no client browser ever touches gRPC. Prod swaps in a proper LB (envoy / grpc-web bridge / service mesh) plus forward-auth + mTLS; specific LB and auth-provider choice out of scope. |
| Containers | Short single-stage Dockerfiles (`./agent/Dockerfile`, `./server/Dockerfile`, `./www/Dockerfile`); minimal compose with **`image:` overlays on every `build:`** so katenary references real registry tags rather than a build target it can't execute. Image *tag* defaults to `${IMAGE_TAG:-dev}` — a pre-commit hook (`post-checkout`/`post-merge`/`post-rewrite`) writes `IMAGE_TAG=<current-branch>` to `.env` on every checkout, so `docker compose build` produces branch-named images by default; CI overrides with a release tag. `server` and `www` use `build.context: .` so the shared `proto/` is reachable at build/runtime; agent stays `build: ./agent`. Hardening (multi-stage, distroless, non-root USER, healthchecks) and `depends_on: { condition: service_healthy }` **TODO'd** pending infra/compliance decisions. |
| Katenary labels | Compose carries the seven labels from [`docs/katenary-top-seven.md`](./katenary-top-seven.md): `main-app` on `server` (drives `appVersion`), `ports` on depends_on targets (server's 50051, required or katenary rc6 emits an empty chart), `map-env` on `www.SERVER_ADDR` to rewrite `server` → `{{ .Release.Name }}-server` for k8s DNS, `secrets` on `db.POSTGRES_PASSWORD` so it lands in a Secret rather than a ConfigMap, `ingress` on `www` (compose `ports:` only gets ClusterIP otherwise). Rules 1 and 7 (`image:` pinning, `ignore`) handled inline. |
| Proto codegen | Rust: `tonic-build` writes `server/src/proto/hello.rs` (committed) so reviewers see the wire shape in diffs and Docker builds stay hermetic. TypeScript: `@grpc/proto-loader` reads `proto/hello.proto` at runtime — no codegen step, no extra dep. Split is asymmetric on purpose; ts-proto would be a third dep just to mirror Rust's behavior. |
| First-run cost | Minimized — single-stage builds, common base images, no upfront hardening. Cold start ~3–6 min, dominated by TimescaleDB pull + `cargo build --release`. |
| Metrics dashboard | **Basic HTML tables only** for V1. `TODO` comments mark where graphs/charts would expand. |
| Tests | `go test` in `agent/`, `cargo test` in `server/`, `npm test` in `www/`. High-value tests only. |
| Commit hygiene | Test changes and code changes **never** share a commit. Code commits require a green test tree. |
| Agent shutdown | Graceful on SIGTERM/SIGINT: stop collectors → final drain report (bypasses `AGENT_MIN_REPORTING_INTERVAL`) → notify offline → exit. 10s soft deadline (`AGENT_SHUTDOWN_TIMEOUT_SECONDS`). Force-exit code `2` on timeout with data-loss log line. |
| Bonuses | All six in scope |

## Open

- **Agent in-flight data durability** — V1 buffer is in-memory only. Deployment-level durability (blue-green agent replacement) is solved by standard Helm/TF patterns; **in-flight buffer survival across an individual agent's crash/restart is the open question**. Adding WAL-to-disk or a sidecar queue (NATS / Kafka / Redis Streams) depends on compliance requirements, volume, and the related deferrals in spec §9.4 / §9.5. Revisited once production infra context lands. See spec §2.6.

## Deviations from `requirements.md`

- **Go agent** instead of Python — Core Requirements section allows "any language (Rust, Python, Go, etc.)" which supersedes the Tasks-section "python" bullet.
