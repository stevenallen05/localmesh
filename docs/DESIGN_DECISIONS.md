# Design Decisions

Current state of every important design choice, one line each. History in git.
Technical detail (compose layout, OTel config, mTLS layout, proto, code
sketches) lives in [`docs/superpowers/specs/`](./superpowers/specs/).

> **Framing:** this repo is a **reference implementation of the
> microservice-template + service-catalogue pattern** an organization would
> adopt at the CI/CD layer. The Rust + Next.js parts are *one team's
> microservice*; observability is a *catalogue module* the team `include:`s.
> Full picture in
> [`docs/superpowers/specs/2026-05-13-microservice-template-design.md`](./superpowers/specs/2026-05-13-microservice-template-design.md).

## Settled

| Area | Choice |
|---|---|
| Vision | Reference implementation of org-scale CI/CD pattern: **ops** owns service mesh + SPIFFE/SPIRE-issued mTLS + service catalogue + named storage classes + top-level CI/CD; **teams** own a docker-compose-shaped microservice repo that `include:`s required (observability, auth) + optional (DB, cache) modules. Linter-enforced compliance on PRs. Catalogue modules ship dev-local stubs that get substituted at deploy time. |
| Orchestration | `docker compose` is the **single source of truth**; **katenary** auto-derives Helm charts via pre-commit hook (CI in prod). Compose → katenary → helm pipeline is **baked in** — no hand-edited charts. |
| Module composition | **`docker-compose include:`** for catalogue modules. Top-level compose = team's microservice (`server` + `www`); `modules/observability/docker-compose.yml` is the dev-local catalogue stub. In prod the include swaps to ops-published (URL include, registry compose snippet, or CI substitution). |
| Stack inside the observability module | **OTel Collector** (`otelcol-contrib`) receives OTLP from Rust + www, scrapes `node-exporter` + `cadvisor` via its prometheus receiver, `prometheusremotewrite`s to **VictoriaMetrics**. **Grafana** queries VM. vmagent dropped — collector covers both ingest paths. |
| Server | Rust + tonic. **Instrumented with the OpenTelemetry Rust SDK** (`opentelemetry`, `opentelemetry-otlp`, `opentelemetry_sdk`, `tracing`, `tracing-opentelemetry`). Emits OTLP gRPC to the collector. v0 ships `Greeter.Echo` (renamed from `SayHello` template); observable behavior via OTel counters/spans on each call. |
| Server → TSDB | Rust queries VM over **mTLS** as a worked example of the inter-service-mTLS pattern. `reqwest` + `rustls-tls`, client cert/key/CA paths injected via env (provided by the catalogue's secret-mount conventions). |
| www | Next.js Pages Router, template UX preserved. Instrumented with **`@vercel/otel`** via `instrumentation.ts`. Pages Router caveat: Next.js OTel docs are App-Router-flavored; the package works for both — page renders, API routes, middleware all emit traces. Grafana panels embed via iframe. |
| gRPC LB | **Deferred for take-home.** v0 wires the Next.js API route directly to Rust via `@grpc/grpc-js` (server-side; browser never speaks gRPC). Prod swaps in mesh-aware LB + SPIFFE mTLS. |
| Proto | `proto/hello.proto` — `Greeter.Echo` for v0. Rust commits `tonic-build` output under `server/src/proto/`; TS uses `@grpc/proto-loader` at runtime. |
| mTLS dev story | `make mtls` runs openssl with **force-override** to produce a dev root CA + per-service keypairs into `secrets/` (gitignored). VM configured to require client certs signed by the root CA. Rust loads paths from env. Compose: bind-mounts. Helm via katenary: secrets+configmap-files. **Prod**: External Secrets Operator / Sealed Secrets / Vault CSI / cloud KMS — none of which live in this repo; the catalogue swap covers it. |
| Secrets injection | Via katenary standards: `katenary.v3/secrets` (env → K8s Secrets); `katenary.v3/configmap-files` (mount cert files). Team services consume the module's secrets via `katenary.v3/values-from`. |
| Volume-class abstraction | Custom convention: PVC `storageClassName` comes from a values.yaml field (`<module>.volumeClass`) with sensible defaults (dev: empty/cluster-default). Prod overlays `longhorn-soc2-sensitive`, `fault_tolerant_cache`, `ephemeral_scratch`, etc. — **human-reviewable in PR labels**, not buried in YAML. Katenary doesn't carry a native abstraction; this is a documented pattern. |
| Containers | Single-stage Dockerfiles for `server` and `www`; off-the-shelf images for everything in modules. `build.context: .` so shared `proto/` is reachable. Multi-stage / non-root / distroless deferred. |
| Image tags | Hardcoded `0.1.0` for the local-only PoC (Rule 1 from [`docs/rules/katenary-top-seven.md`](./rules/katenary-top-seven.md)). Prod CI replaces with registry-pushed tags. |
| Katenary labels | The seven labels from [`docs/rules/katenary-top-seven.md`](./rules/katenary-top-seven.md) + honorable mentions (`values-from`, `configmap-files`). `main-app` on `server`. `values-from` is the load-bearing label for catalogue consumption — the team's compose pulls env from the observability module's services without duplicating credentials/hostnames. |
| README scope | `docker compose up` is the supported path; reproducible in GitHub Codespaces per requirements. The author's personal k8s cluster (with Longhorn) hosts the live demo; **no warranty for other k8s clusters** (the volume class names + cert authority + mesh configuration are author-specific). |
| Live demo | CloudFlare Zero Trust tunnel + Access. |
| Logging | Rust: `tracing` (structured) bridged to OTel via `tracing-opentelemetry`; spans/events flow with metrics through the same OTLP exporter. Off-the-shelf services: stdout/stderr to the compose log driver. Loki / Promtail as a future catalogue module. |
| Tests | `cargo test` in `server/`, `npm test` in `www/`. High-value tests only. |
| Commit hygiene | Test changes and code changes **never** share a commit. Code commits require a green test tree. |
| Bonuses | All six in scope. The OTel-everywhere posture makes "good logging" and "good error handling" naturally stronger than custom alternatives. Mapping in spec §10. |

## Open

- **Rust gRPC API surface beyond `Echo`** — `TsdbHealth()`, `GetMetric(name, range)`, `WatchMetric(name) → stream`, auth model, error mapping. v0 ships `Echo`; the rest is its own follow-up.
- **mTLS demonstration depth** — minimum: Rust startup health-check to VM, logs success. Reviewer-visible: `Greeter.TsdbHealth()` gRPC method + UI button. Both are cheap.
- **OTel Collector → VM mTLS** — plain HTTP in dev for simplicity; prod = mesh-enforced. Could be made uniform on the dev path at small cost.
- **Module dev↔prod swap mechanic** — URL include vs. CI substitution vs. helm chart dependency. Not blocking v0.
- **Linter rules for catalogue compliance** — custom PR linters enforcing module usage, mTLS wiring, volume-class registry. Out of scope for the take-home; logged for the bigger picture.
- **Loki / Promtail as a future catalogue module** — obvious next step after metrics observability lands.

## Deviations from `requirements.md`

- **Agent → Server (literal arrow):** layered. OTel Collector receives metrics; VM stores them; Rust is the application API. "The server," in the requirements' sense, is the layered tier (VM + Rust). Justification: separating off-the-shelf storage from the custom application API is the right factoring.
- **Server in Rust** — satisfies the literal requirement.
- **Agent in Python (Tasks-section bullet)** — superseded by Core Requirements' "any language." Off-the-shelf OTel Collector (Go binary) instead, packaged as a *catalogue module* rather than a team-owned component.
