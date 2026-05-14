# Design decisions

Current state of every design choice — one row each. Implementation specifics (crates, code layout, label syntax) live in [`docs/engineering/`](../engineering/) and the spec at [`docs/superpowers/specs/`](../superpowers/specs/).

Companion: [`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md), [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md), [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

This repo is a reference implementation of the **microservice-template + service-catalogue pattern** an org would adopt at the CI/CD (Continuous Integration / Continuous Delivery) layer. The Rust + Next.js parts are one team's microservice; observability is a catalogue module the team includes.

## Settled

**Prod-considerations** is row-specific only. The full production assumption set is in [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md).

| Area | Dev choice | Rationale | Prod-considerations |
|---|---|---|---|
| Vision | One team's microservice + a stub catalogue module. | Demonstrates the org-scale pattern: platform owns catalogue, product owns compose. | Catalogue becomes a real platform product. |
| Orchestration | Compose is source of truth; pre-commit hook generates the Helm chart. | Compose is the practical ceiling for self-service teams; chart is build output. | Chart generation runs in CI; published to an internal registry. |
| Module composition | Compose `include:` pulls `service_catalog/observability/` from the catalogue. | Required and optional modules added by reference, not copy-paste. | Include path swaps to a platform-published location. |
| Observability module | Off-the-shelf collector + exporters + TSDB (Time-Series Database) + dashboards. | All battle-tested; team adds no value building these. | TSDB swaps to a managed/clustered equivalent at scale. |
| Server | Rust gRPC. Greeter.SayHello (trace-stitching reference) + Catalog (List/Get for datasources + metrics; full CRUD for dashboards, single-panel single-query model). Traces (application metrics pending re-design). | Preserves the original template wire-shape; trace instrumentation visible end-to-end. | Demo endpoints replaced by real product methods; metric surface added once chosen. |
| Server → metrics DB | mTLS (mutual TLS), certificates injected by the catalogue's secrets layer. | Worked example of the inter-service mTLS pattern. | File-mounted certs replaced by mesh-issued short-lived identities. |
| www | Next.js front-end, traces enabled, dashboard panels embedded inline. | Pages, API routes, middleware all emit traces; Grafana owns dashboard chrome. | Iframe embeds wrapped by an auth gateway. |
| gRPC load balancing | Next.js backend calls Rust directly, server-side. | Browser → HTTPS → Next.js → gRPC → Rust. The gRPC hop stays server-side. | Mesh-aware LB (Load Balancer) or grpc-web bridge. |
| Wire schema | Single `.proto` shared between Rust and Next.js. | One language commits generated code; the other loads at runtime. | Real product schema; lint + breaking-change tooling layered on top. |
| Local mTLS setup | `make mtls` generates a dev CA (Certificate Authority) + per-service keys into a gitignored dir. | One command, zero friction; certs bind-mounted into containers. | Replaced by a real secrets backend. |
| Secrets injection | Compose labels turn env vars into K8s (Kubernetes) Secrets and mount cert files. | Team-facing interface stays compose; the chart generates the right Secret/ConfigMap shapes. | Same interface; secrets resolve from a real backend. |
| Volume classes | Named local volume in dev; chart exposes a storage-class values field. | Dev doesn't need named classes; each env overlays the right one. | Named classes (`soc2_sensitive`, etc.) provisioned by the cluster. |
| Containers | Single-stage Dockerfiles for Rust and Next.js; everything else off-the-shelf. | Shortest build path; hardening costs build time without demo value. | Multi-stage, non-root, real healthchecks, signed images. |
| Image tags | Hardcoded placeholder. | The Helm generator needs a tag; no registry to push to locally. | CI-derived tag pushed with provenance + SBOM (Software Bill of Materials). |
| Compose-to-Helm labels | Standard label set from [`katenary-top-seven.md`](../engineering/rules/katenary-top-seven.md). | Each label flips one specific compose→Helm translation. | May extend with org-specific custom labels. |
| README scope | `docker compose up`; runs in GitHub Codespaces per the take-home requirements. | Codespaces is the most predictable demo environment for reviewers. | Deployment runs via the org's standard CI/CD. |
| Live demo | CloudFlare Zero Trust tunnel + Access to author's K8s cluster. | Easy public URL with an auth gate; no warranty for other clusters. | Real ingress with cert-manager + org IdP (Identity Provider). |
| Logging | Rust prints lifecycle events to stdout; off-the-shelf services log to stdout. | Minimal surface; one observability pipe via traces, no separate logs pipeline. | Add a logs catalogue module (Loki, etc.) for retention/audit; structured logs added if/when their absence bites. |
| Bonuses | All six items from `requirements.md` in scope. | OTel (OpenTelemetry)-everywhere carries logging and error handling; catalogue argument carries data design. | Per-bonus mapping in the spec. |
| Catalog wire shape | AIP-130 standard methods on DataSource (read-only) and Dashboard (full CRUD); tagged-union panel body (Timeseries/Stat/Table). Paths only — www prepends `NEXT_PUBLIC_GRAFANA_URL`. | Symmetric resource shape per AIP; client-side URL composition keeps the server environment-agnostic. | Add `page_size`/`page_token` (AIP-158) if data sets grow. |
| Grafana auth | Static service-account token, env-injected. Dev: minted at first boot by `grafana-bootstrap` one-shot container, written to a shared named volume. | Production-shaped credential (Grafana 9+ replaces API keys with service accounts); single token, Bearer header. | Replace bootstrap with a Vault Agent / secrets-backend sidecar that writes the same file. Rust code unchanged. |

### Process notes (not architecture)

- **Tests:** Rust unit/integration in `server/`; Next.js tests in `www/`. High-value tests only — PoC (Proof of Concept) scope.
- **Commit hygiene:** test changes and code changes never share a commit; code commits require a green tree.

## Open

V0 questions still in flux. Production-scope questions live in [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

- **mTLS demonstration depth.** Minimum: server logs successful connection at startup. Reviewer-visible: on-demand gRPC method + UI button.
- **Collector → TSDB mTLS on the dev path.** Plain HTTP for now; uniform with the Rust→TSDB hop is possible at small cost.
- **Application metrics on the Rust + www path.** v0 had `say_hello_total` + `say_hello_duration_seconds`; both removed in chunk 5b. Re-design pending — likely per-method gRPC duration + status distribution.
- **Panel types beyond Timeseries.** Stat and Table variants stubbed in `metrics.proto` — handler returns `UNIMPLEMENTED`. Wired when a reviewer use-case appears; panel JSON differs only in `type` + a couple of option fields per type, so it's mostly mechanical.

## Deviations from `requirements.md`

- **Agent → Server arrow is layered.** Off-the-shelf collector receives; TSDB stores; Rust server is the application API in front. *"The server"* in the requirements' sense is the layered tier.
- **Server in Rust** — satisfies the literal requirement.
- **Agent in Python** (Tasks section) — superseded by Core Requirements' "any language." Off-the-shelf collector instead, packaged as a catalogue module.
