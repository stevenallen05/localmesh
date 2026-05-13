# Design Decisions

Current state of every important design choice. History in git. Technical
detail (compose layout, OTel config, mTLS layout, proto, code sketches) lives
in [`docs/superpowers/specs/`](./superpowers/specs/).

> **Framing:** this repo is a **reference implementation of the
> microservice-template + service-catalogue pattern** an organization would
> adopt at the CI/CD layer. The Rust + Next.js parts are *one team's
> microservice*; observability is a *catalogue module* the team `include:`s.
> Full picture in
> [`docs/superpowers/specs/2026-05-13-microservice-template-design.md`](./superpowers/specs/2026-05-13-microservice-template-design.md).

## Settled

| Area | Dev choice | Rationale | Possible prod considerations & choices |
|---|---|---|---|
| Vision | This repo = one team's microservice + a stub catalogue module. | Showcases an org-scale CI/CD pattern where ops owns the catalogue and teams write compose files instead of becoming Kubernetes experts. | Catalogue is a real ops-team product: versioned modules, linter-enforced compliance on PRs, central registry. |
| Orchestration | `docker compose` is the single source of truth; pre-commit hook regenerates the Helm chart via katenary. | Compose is the practical ceiling of what most teams can wrangle on their own. The chart is generated, not hand-edited. | Regen + lint runs in CI rather than pre-commit; charts published to an OCI registry; per-environment values overlays. |
| Module composition | Compose `include:` pulls `modules/observability/docker-compose.yml`. | One file the team controls; required and optional catalogue modules are added by reference, not copy-paste. | Include path swaps to an ops-published location (URL include, OCI-distributed compose snippet, or CI substitution). |
| Observability module | OTel Collector (otelcol-contrib) + node-exporter + cadvisor + VictoriaMetrics + Grafana. | Off-the-shelf components handle ingest, infra scraping, time-series storage, and visualization. All battle-tested. | Same components, scaled (collector replicas, VM cluster mode, stateless Grafana); swap VM for Mimir/Influx if scale demands. |
| Server | Rust + tonic + OpenTelemetry Rust SDK. v0 ships `Greeter.Echo`. | Echo preserves the template wire; OTel counters and spans per call drive observable behavior visible in Grafana. | Real product logic and gRPC methods; same OTel SDK; outbound auth via mesh-issued identities, not file-mounted certs. |
| Server → TSDB | Rust queries VictoriaMetrics over mTLS using `reqwest` + `rustls-tls`; cert paths injected via env. | Worked example of the inter-service mutual-TLS pattern the catalogue is built around. | Mesh-enforced mTLS (SPIFFE/SPIRE workload SVIDs); file mounts replaced by mesh-injected, short-lived identities. |
| www | Next.js Pages Router + `@vercel/otel` via `instrumentation.ts`. Template UX preserved; Grafana panels embed via iframe. | Page renders, API routes, and middleware all emit traces. Grafana owns dashboard chrome; www links to it. | CDN/edge in front; iframe embeds wrapped by forward-auth or OIDC; SSR/RSC pattern as the team prefers. |
| gRPC LB | Next.js API route calls Rust directly via `@grpc/grpc-js`. | Server-side gRPC only; the browser never speaks the protocol. Proof-of-concept scope. | Envoy / grpc-web bridge / service mesh sidecar; mTLS-aware load balancing; browser stays on HTTPS to the edge. |
| Proto | `proto/hello.proto` with `Greeter.Echo`. Rust commits `tonic-build` output; TS loads at runtime via `@grpc/proto-loader`. | Minimal wire. Both languages need the proto; only one needs codegen committed to make builds hermetic. | Real product proto; same codegen split; `buf` for lint and breaking-change checks; proto distribution via a schema registry. |
| mTLS dev story | `make mtls` runs openssl to force-overwrite a dev root CA + per-service keypairs into gitignored `secrets/`. | Single command, zero friction; certs are bind-mounted into the relevant containers locally. | External Secrets Operator / Vault CSI / Sealed Secrets / cloud KMS. None of which live in this repo; the catalogue swap covers it. |
| Secrets injection | `katenary.v3/secrets` and `configmap-files` for the Helm path; team consumes module outputs via `values-from`. | Compose env and bind-mounts translate to Kubernetes Secrets/ConfigMaps via labels — no extra YAML to write. | Labels resolve to live secrets via the mechanisms above; same compose interface stays for the team's pull-request workflow. |
| Volume-class abstraction | Named volume; PVC has empty `storageClassName` (cluster default). A `values.yaml` field exposes the class. | Dev doesn't need named classes; each environment overlays the right one for its workload. | Named storage classes (`longhorn-soc2-sensitive`, `fault_tolerant_cache`, `ephemeral_scratch`); choices reviewable in PR labels. |
| Containers | Single-stage Dockerfiles for `server` and `www`; off-the-shelf images for modules. `build.context: .` for proto reach. | Shortest build path; multi-stage hardening costs build time without demo value. | Multi-stage builds; distroless base; non-root USER; signed images; SBOMs; vulnerability scanning; healthchecks. |
| Image tags | Hardcoded `0.1.0` placeholder; no registry prefix. | Katenary requires an `image:` tag to render Helm; the PoC has no registry to push to. | CI sets the tag from a git release; pushes to `ghcr.io` / ECR / etc. with provenance and SBOM. |
| Katenary labels | The seven mandatory labels + `values-from` + `configmap-files`. See [`docs/rules/katenary-top-seven.md`](./rules/katenary-top-seven.md). | Each label flips a specific compose→Helm translation. `values-from` is how teams cleanly consume catalogue modules. | Same labels; possibly extended with org-specific custom labels (e.g., a native `volume-class:` once the convention solidifies). |
| README scope | `docker compose up` is the supported entrypoint; reproducible in GitHub Codespaces per requirements. | Codespaces is the most predictable runtime for reviewers; per-team READMEs document only the team's product. | Deployment is via the org's standard CI/CD; per-team READMEs focus on the team's product, not infra. |
| Live demo | CloudFlare Zero Trust tunnel + Access pointing at the author's personal k8s cluster (Longhorn-equipped). | Easy public URL with an auth gate. No warranty for other k8s clusters — storage class and cert authority are author-specific. | Real ingress + cert-manager + mesh forward-auth + org IdP; published service URL with TLS from a real CA. |
| Logging | Rust `tracing` bridged to OTel via `tracing-opentelemetry`; off-the-shelf services log to stdout. | Spans and events flow through the same OTLP pipe as metrics — one observability surface, not two. | Add a Loki/Promtail catalogue module for aggregation, long-term retention, PII redaction, and audit trails. |
| Bonuses | All six items from `requirements.md` in scope. | OTel-everywhere makes "good logging" and "good error handling" naturally stronger; the catalogue argument carries "good database design." | Each bonus row above lists its production form. The spec §10 has the per-bonus mapping. |

### Process notes (not architecture)

- **Tests:** `cargo test` in `server/`, `npm test` in `www/`. High-value tests only — PoC scope.
- **Commit hygiene:** test changes and code changes **never** share a commit; code commits require a green tree.

## Open

- **Rust gRPC API surface beyond `Echo`** — `TsdbHealth()`, `GetMetric(name, range)`, `WatchMetric(name) → stream`, auth model, error mapping. v0 ships `Echo`; the rest is its own follow-up.
- **mTLS demonstration depth** — minimum: Rust startup health-check to VM, logs success. Reviewer-visible: `Greeter.TsdbHealth()` gRPC method + UI button. Both are cheap.
- **OTel Collector → VM mTLS** — plain HTTP in dev for simplicity; prod = mesh-enforced. Could be made uniform on the dev path at small cost.
- **Module dev↔prod swap mechanic** — URL include vs. CI substitution vs. Helm chart dependency. Not blocking v0.
- **Linter rules for catalogue compliance** — custom PR linters enforcing module usage, mTLS wiring, volume-class registry. Out of scope for the take-home; logged for the bigger picture.
- **Loki / Promtail as a future catalogue module** — obvious next step after metrics observability lands.

## Deviations from `requirements.md`

- **Agent → Server (literal arrow):** layered. The OTel Collector receives metrics; VM stores them; Rust is the application API. "The server," in the requirements' sense, is the layered tier (VM + Rust). Justification: separating off-the-shelf storage from the custom application API is the right factoring.
- **Server in Rust** — satisfies the literal requirement.
- **Agent in Python (Tasks-section bullet)** — superseded by Core Requirements' "any language." Off-the-shelf OTel Collector (a Go binary) instead, packaged as a *catalogue module* rather than a team-owned component.
