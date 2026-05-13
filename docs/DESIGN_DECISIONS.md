# Design Decisions

Current state of every important design choice, one line each. History in git.
Technical detail (compose layout, scrape config, proto, code sketches) lives in
[`docs/superpowers/specs/`](./superpowers/specs/).

> **Prod note:** in real life this is Grafana + OTel + a Prometheus-compatible
> TSDB. Choices below are the same shape, scoped to a take-home.

## Settled

| Area | Choice |
|---|---|
| Orchestration | `docker compose` is the **single source of truth**; **katenary** auto-derives Helm charts via pre-commit hook (CI in prod). Compose-to-helm pipeline is **baked in** — no hand-edited charts. |
| Stack shape | Four slots: **agent** (off-the-shelf collector), **TSDB** (Prometheus-compatible), **dashboards** (Grafana), **application API** (Rust + tonic). The www frontend embeds dashboard panels and calls the application API over gRPC via a Next.js API-route shim. |
| Agent | `vmagent` scrapes `node-exporter` (host metrics) + `cadvisor` (per-container metrics) and `remote_write`s to VM. Scrape config in YAML, mounted via katenary `configmap-files` for the Helm path. |
| TSDB | **VictoriaMetrics** single-node. Prometheus-compatible HTTP API consumed by both Grafana and the Rust server. Battle-tested, low-overhead, native `remote_write` receiver (no flag flip). |
| Dashboards | **Grafana** queries VM directly; panels iframe-embedded into www. Anonymous-org auth for the PoC; forward-auth / OIDC for prod (deferred). |
| Server | Rust + tonic. Stateless. Queries VM via PromQL HTTP. v0 ships the template-shape `Greeter.SayHello` gRPC POC; the API surface beyond that (`GetMetric`, `WatchMetric`, auth, error mapping) is open — its own spec. |
| Proto | `proto/hello.proto` (Greeter.SayHello) for v0. Rust commits `tonic-build` output under `server/src/proto/` so reviewers see the wire in diffs and Docker builds stay hermetic; TS uses `@grpc/proto-loader` at runtime (no JS codegen step). |
| www | Next.js Pages Router, template UX preserved. Page is client-rendered (`pages/index.tsx`); `pages/api/test-rpc.ts` does the actual gRPC call via `@grpc/grpc-js`. Grafana panels embed via iframe. |
| gRPC LB | **Deferred for take-home.** v0 wires the Next.js API route straight to the Rust server via `@grpc/grpc-js` (server-side; browser never speaks gRPC). Prod swaps in a proper LB (envoy / grpc-web bridge / service mesh) plus forward-auth + mTLS. |
| Containers | Single-stage Dockerfiles for `server` and `www` only; agent/TSDB/dashboards are off-the-shelf images. Minimal compose with **`image:` overlays on every `build:`** so katenary references real registry tags rather than a build target it can't execute. `server` and `www` use `build.context: .` so the shared `proto/` is reachable. Hardening (multi-stage, distroless, non-root USER, healthchecks) and `depends_on: { condition: service_healthy }` **TODO'd** pending infra/compliance decisions. |
| Katenary labels | The seven labels from [`docs/rules/katenary-top-seven.md`](./rules/katenary-top-seven.md): `main-app` on `server` (drives `appVersion`), `ports` on every `depends_on` target, `map-env` to rewrite cross-service hostnames for cluster DNS, `secrets` on sensitive env, `ingress` on `www` and `grafana`, `configmap-files` for the vmagent scrape config (and Grafana provisioning when it lands). Rules 1 and 7 (`image:` pinning, `ignore`) handled inline. |
| First-run cost | Minimized — single-stage builds, off-the-shelf images, no upfront hardening. Cold start dominated by VM + Grafana pulls + `cargo build --release`. |
| Live demo | CloudFlare Zero Trust tunnel + Access. |
| Logging | Rust: `tracing` to stdout. Off-the-shelf services: stdout/stderr to the compose log driver. Aggregation (Loki / Promtail) deferred. |
| Tests | `cargo test` in `server/`, `npm test` in `www/`. High-value tests only. |
| Commit hygiene | Test changes and code changes **never** share a commit. Code commits require a green test tree. |
| Bonuses | All six in scope; see spec §8 for the off-the-shelf-feature mapping. |

## Open

- **Rust server's gRPC API surface beyond `Greeter.SayHello`** — endpoints
  (`GetMetric`, `WatchMetric`, `ListMetrics`?), auth surface, error mapping,
  streaming vs. unary. Spec follow-up — see
  [`docs/superpowers/specs/2026-05-13-metrics-stack-design.md`](./superpowers/specs/2026-05-13-metrics-stack-design.md) §7.1.
- **Grafana embed details** — kiosk-style panel-only vs. full-dashboard
  iframe; auth model for embedded panels in prod (forward-auth / shared
  session with www).
- **node-exporter / cadvisor under katenary** — both conventionally
  DaemonSets / privileged DaemonSets in k8s; katenary produces single
  Deployments. PoC chart fine; prod overlay swaps to the official Helm
  charts. Captured for reviewer awareness.
- **VictoriaMetrics persistence** — named volume / PVC. Retention,
  backups, cluster-mode upgrade path all deferred.
- **Remote interval adjustment (bonus)** — vmagent supports `SIGHUP` /
  `POST /-/reload`; how (or whether) this gets surfaced through the Rust
  API to www is open.
- **Horizontal scaling (bonus)** — vmagent fleet, VM cluster mode,
  stateless Grafana / Rust. Sketched, not implemented.

## Deviations from `requirements.md`

- **Agent → Server (literal arrow):** split. vmagent submits to the storage
  layer (VictoriaMetrics) via `remote_write`; the Rust server is the
  application API layer. "The server" in the requirements' sense is the
  layered tier (VM + Rust). The literal agent→Rust arrow would have Rust
  reinventing what vmagent + VM already do; the layered factoring keeps the
  storage engine off-the-shelf and the API custom.
- **Agent language** — Tasks-section says Python; Core Requirements override
  with "any language." We satisfy the latter with an off-the-shelf Go binary
  (vmagent).
