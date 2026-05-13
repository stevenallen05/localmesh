# Design Decisions

Current state of every important design choice — one row each. Technical
implementation detail (specific crates, code layout, label syntax) lives in
the engineering-side docs and the spec at
[`docs/superpowers/specs/`](../superpowers/specs/).

> **Framing:** this repo is a **reference implementation of the
> microservice-template + service-catalogue pattern** an organization would
> adopt at the CI/CD layer. The Rust + Next.js parts are *one team's
> microservice*; observability is a *catalogue module* the team
> includes. Companion docs:
> - [`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md) — the design rules
>   that drove the choices below.
> - [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md) — what's in / out of scope.
> - [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md) —
>   stakeholder discovery framework for taking this to production.

## Settled

> The **prod-considerations** column lists *row-specific* signposts only.
> The full production assumption set lives in
> [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md); the conversations a real prod
> engagement would start with live in
> [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

| Area | Dev choice | Rationale | Prod-considerations (row-specific) |
|---|---|---|---|
| Vision | This repo = one team's microservice + a stub catalogue module. | Demonstrates an org-scale pattern where the platform team owns the catalogue and product teams own a compose file — not a Kubernetes deployment. | Catalogue becomes a real platform-team product. |
| Orchestration | Compose is the single source of truth; pre-commit hook generates the Helm chart automatically. | Compose is the practical ceiling of what most teams can wrangle on their own; the chart is build output, not hand-edited. | Chart generation runs in CI; charts published to an internal registry. |
| Module composition | The team's compose file *includes* a `modules/observability/` file from the catalogue. | One file the team controls; required and optional modules are added by reference, not copy-paste. | The include path swaps to a platform-team-published location in production. |
| Observability module | Open-source collector + host/container metrics exporters + time-series DB + dashboarding (all industry-standard). | Off-the-shelf components handle ingest, infra scraping, storage, and visualization. All battle-tested. | Swap the time-series DB for a managed/clustered equivalent if scale demands. |
| Server | Rust gRPC server, instrumented for traces and metrics; ships one demonstration endpoint for v0. | The demo endpoint preserves the original template's wire-shape; instrumentation makes its behavior visible in dashboards. | Demonstration endpoint replaces with real product gRPC methods. |
| Server → metrics DB | The server queries the metrics DB over **mutual TLS**, with certificates injected by the catalogue's secrets layer. | Worked example of the inter-service mTLS pattern the catalogue is built around. | File-mounted certificates replaced by mesh-issued, short-lived workload identities. |
| www | Next.js front-end, instrumented for traces. Template UX preserved. Dashboard panels embed inline. | Page renders, API routes, and middleware all emit traces. Grafana owns the rich dashboard chrome; www embeds it. | Iframe embeds wrapped by an authentication gateway. |
| gRPC load balancing | The Next.js back-end calls the Rust server directly — server-side, never browser. | Browser → HTTPS → Next.js → gRPC → Rust. Proof-of-concept; the gRPC hop stays server-side. | Direct call replaced by a mesh-aware load balancer or grpc-web bridge. |
| Wire schema | Single protocol buffer file shared between Rust and Next.js (one commits generated code, the other loads it at runtime). | Minimal wire; both languages need the same schema, only one needs code generation in the build. | Real product schema; lint and breaking-change tooling layered on top. |
| Local mTLS setup | A single `make mtls` command generates a dev certificate authority + per-service keypairs into a gitignored directory. | One command, zero friction; certificates are bind-mounted into the relevant containers. | Production replaces the local directory with a real secrets backend (see [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md)). |
| Secrets injection | Compose labels turn environment variables into Kubernetes Secrets and mount certificate files for the Helm path. | Team-facing interface is the compose file; the chart generates the right Secret + ConfigMap shapes automatically. | Same compose-level interface; secrets resolve from a real backend (Vault, External Secrets, etc.). |
| Volume classes | Dev uses a named local volume; the Helm chart exposes a values field for storage class. | Dev doesn't need named classes; each environment overlays the right one for the workload. | Named storage classes (`soc2_sensitive`, `fault_tolerant_cache`, etc.) provisioned by the cluster. |
| Containers | Single-stage Dockerfiles for the Rust and Next.js builds; everything else is off-the-shelf images. | Shortest build path; container hardening costs build time without demo value. | Hardened multi-stage builds, non-root user, real healthchecks, signed images. |
| Image tags | Hardcoded placeholder. | A tag is required for the Helm generator to render; this is a local-only PoC with no registry to push to. | CI-derived tag pushed to a real registry with provenance and SBOM. |
| Compose-to-Helm labels | Standard set of labels from [`docs/engineering/rules/katenary-top-seven.md`](../engineering/rules/katenary-top-seven.md). | Each label flips a specific compose→Helm translation. The catalogue-consumption label is what lets teams pull module env cleanly. | Possibly extended with org-specific custom labels. |
| README scope | `docker compose up` is the supported entrypoint; runs in GitHub Codespaces per the take-home requirements. | Codespaces is the most predictable demo environment for reviewers. | Deployment is via the org's standard CI/CD; per-team READMEs document the product, not the infrastructure. |
| Live demo | CloudFlare Zero Trust tunnel + Access pointing at the author's personal Kubernetes cluster. | Easy public URL with an authentication gate. No warranty for other clusters. | Real ingress with cert-manager + the org's identity provider. |
| Logging | Rust structured logs flow into the same telemetry pipe as metrics; off-the-shelf services log to stdout. | One observability surface for both metrics and logs — no parallel pipeline to maintain. | Add a logs catalogue module (Loki, etc.) for aggregation, retention, and audit. |
| Bonuses | All six items from `requirements.md` in scope. | The OpenTelemetry-everywhere posture makes "good logging" and "good error handling" naturally stronger; the catalogue argument carries the "good database design" defense. | Per-bonus mapping in the spec. |

### Process notes (not architecture)

- **Tests:** Rust unit/integration in `server/`; Next.js tests in `www/`. High-value tests only — PoC scope.
- **Commit hygiene:** test changes and code changes never share a commit; code commits require a green tree.

## Open

V0-scope design questions still in flux. Production-scope conversations
(governance, scale, multi-region, supply-chain, etc.) live in
[`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md).

- **Rust gRPC API surface beyond the demonstration endpoint** — health-check, metric-getter, streaming, auth model, error mapping. V0 ships just the demonstration; the rest is its own follow-up.
- **mTLS demonstration depth** — minimum: server logs successful connection at startup. Reviewer-visible: an on-demand gRPC method + UI button. Both are cheap.
- **Collector → metrics-DB mTLS on the dev path** — plain HTTP for simplicity; making it uniform with the Rust→DB hop is possible at small cost.

## Deviations from `requirements.md`

- **Agent → Server (literal arrow):** layered. The off-the-shelf collector receives metrics; the time-series DB stores them; the Rust server is the application API in front. *"The server,"* in the requirements' sense, is the layered tier. Justification: separating off-the-shelf storage from the custom application API is the right factoring.
- **Server in Rust** — satisfies the literal requirement.
- **Agent in Python** (Tasks-section bullet) — superseded by Core Requirements' "any language." Off-the-shelf collector instead, packaged as a *catalogue module* rather than a team-owned component.
