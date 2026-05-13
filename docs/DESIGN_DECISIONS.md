# Design Decisions

Current state of every important design choice. History in git. Technical
detail (compose layout, OTel config, mTLS layout, proto, code sketches) lives
in [`docs/superpowers/specs/`](./superpowers/specs/).

> **Framing:** this repo is a **reference implementation of the
> microservice-template + service-catalogue pattern** an organization would
> adopt at the CI/CD layer. The Rust + Next.js parts are *one team's
> microservice*; observability is a *catalogue module* the team `include:`s.
> Design rules in [`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md); dev/prod
> scope boundary in [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md); full design
> detail in
> [`docs/superpowers/specs/2026-05-13-microservice-template-design.md`](./superpowers/specs/2026-05-13-microservice-template-design.md).

## Settled

> The **prod-considerations** column lists *row-specific* signposts only.
> Cross-cutting infrastructure assumptions (service mesh, secrets backend,
> OCI registry, container hardening, CI/CD pipeline, ingress with cert-manager,
> named storage classes, scaling topology, gRPC LB, observability hardening,
> per-service charts, linter rules, provider choices) all live in
> [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md).

| Area | Dev choice | Rationale | Prod-considerations (row-specific) |
|---|---|---|---|
| Vision | This repo = one team's microservice + a stub catalogue module. | Showcases an org-scale CI/CD pattern where ops owns the catalogue and teams write compose files instead of becoming Kubernetes experts. | Catalogue becomes a real ops-team product. |
| Orchestration | `docker compose` is the single source of truth; pre-commit hook regenerates the Helm chart via katenary. | Compose is the practical ceiling of what most teams can wrangle on their own. The chart is generated, not hand-edited. | Regen + lint runs in CI rather than pre-commit. |
| Module composition | Compose `include:` pulls `modules/observability/docker-compose.yml`. | One file the team controls; required and optional catalogue modules are added by reference, not copy-paste. | Include path swaps to ops-published location. |
| Observability module | OTel Collector (otelcol-contrib) + node-exporter + cadvisor + VictoriaMetrics + Grafana. | Off-the-shelf components handle ingest, infra scraping, time-series storage, and visualization. All battle-tested. | Swap VM for Mimir/Influx if scale demands. |
| Server | Rust + tonic + OpenTelemetry Rust SDK. v0 ships `Greeter.Echo`. | Echo preserves the template wire; OTel counters and spans per call drive observable behavior visible in Grafana. | Real product logic replaces Echo; same OTel SDK. |
| Server → TSDB | Rust queries VictoriaMetrics over mTLS using `reqwest` + `rustls-tls`; cert paths injected via env. | Worked example of the inter-service mutual-TLS pattern the catalogue is built around. | File-mounted certs replaced by mesh-injected identities. |
| www | Next.js Pages Router + `@vercel/otel` via `instrumentation.ts`. Template UX preserved; Grafana panels embed via iframe. | Page renders, API routes, and middleware all emit traces. Grafana owns dashboard chrome; www links to it. | Iframe embeds wrapped by forward-auth. |
| gRPC LB | Next.js API route calls Rust directly via `@grpc/grpc-js`. | Server-side gRPC only; the browser never speaks the protocol. Proof-of-concept scope. | Direct dial replaced by mesh-aware LB. |
| Proto | `proto/hello.proto` with `Greeter.Echo`. Rust commits `tonic-build` output; TS loads at runtime via `@grpc/proto-loader`. | Minimal wire. Both languages need the proto; only one needs codegen committed to make builds hermetic. | `buf` for lint and breaking-change checks; schema-registry distribution. |
| mTLS dev story | `make mtls` runs openssl to force-overwrite a dev root CA + per-service keypairs into gitignored `secrets/`. | Single command, zero friction; certs are bind-mounted into the relevant containers locally. | Real secrets backend replaces the `secrets/` directory. |
| Secrets injection | `katenary.v3/secrets` and `configmap-files` for the Helm path; team consumes module outputs via `values-from`. | Compose env and bind-mounts translate to Kubernetes Secrets/ConfigMaps via labels — no extra YAML to write. | Labels resolve to live secrets; compose interface unchanged. |
| Volume-class abstraction | Named volume; PVC has empty `storageClassName` (cluster default). A `values.yaml` field exposes the class. | Dev doesn't need named classes; each environment overlays the right one for its workload. | Named classes provisioned by the cluster. |
| Containers | Single-stage Dockerfiles for `server` and `www`; off-the-shelf images for modules. `build.context: .` for proto reach. | Shortest build path; multi-stage hardening costs build time without demo value. | Hardened multi-stage builds. |
| Image tags | Hardcoded `0.1.0` placeholder; no registry prefix. | Katenary requires an `image:` tag to render Helm; the PoC has no registry to push to. | CI-derived tag, pushed to a real registry. |
| Katenary labels | The seven mandatory labels + `values-from` + `configmap-files`. See [`docs/rules/katenary-top-seven.md`](./rules/katenary-top-seven.md). | Each label flips a specific compose→Helm translation. `values-from` is how teams cleanly consume catalogue modules. | Possibly extended with org-specific custom labels. |
| README scope | `docker compose up` is the supported entrypoint; reproducible in GitHub Codespaces per requirements. | Codespaces is the most predictable runtime for reviewers; per-team READMEs document only the team's product. | Deployment via the org's standard CI/CD. |
| Live demo | CloudFlare Zero Trust tunnel + Access pointing at the author's personal k8s cluster (Longhorn-equipped). | Easy public URL with an auth gate. No warranty for other k8s clusters. | Real ingress with org IdP. |
| Logging | Rust `tracing` bridged to OTel via `tracing-opentelemetry`; off-the-shelf services log to stdout. | Spans and events flow through the same OTLP pipe as metrics — one observability surface, not two. | Add a logs catalogue module (Loki, etc.). |
| Bonuses | All six items from `requirements.md` in scope. | OTel-everywhere makes "good logging" and "good error handling" naturally stronger; the catalogue argument carries "good database design." | Per-bonus mapping in spec §10. |

### Process notes (not architecture)

- **Tests:** `cargo test` in `server/`, `npm test` in `www/`. High-value tests only — PoC scope.
- **Commit hygiene:** test changes and code changes **never** share a commit; code commits require a green tree.

## Open

- **Rust gRPC API surface beyond `Echo`** — `TsdbHealth()`, `GetMetric(name, range)`, `WatchMetric(name) → stream`, auth model, error mapping. v0 ships `Echo`; the rest is its own follow-up.
- **mTLS demonstration depth** — minimum: Rust startup health-check to VM, logs success. Reviewer-visible: `Greeter.TsdbHealth()` gRPC method + UI button. Both are cheap.
- **OTel Collector → VM mTLS in dev** — plain HTTP for simplicity; uniform mTLS on the dev path is possible at small cost.
- **Module dev↔prod swap mechanic** — URL include vs. CI substitution vs. Helm chart dependency. Not blocking v0.
- **Linter rules for catalogue compliance** — see [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md). Out of scope for the take-home.
- **Logs catalogue module** — obvious next step after metrics observability lands.

## Deviations from `requirements.md`

- **Agent → Server (literal arrow):** layered. The OTel Collector receives metrics; VM stores them; Rust is the application API. "The server," in the requirements' sense, is the layered tier (VM + Rust). Justification: separating off-the-shelf storage from the custom application API is the right factoring.
- **Server in Rust** — satisfies the literal requirement.
- **Agent in Python (Tasks-section bullet)** — superseded by Core Requirements' "any language." Off-the-shelf OTel Collector (a Go binary) instead, packaged as a *catalogue module* rather than a team-owned component.
