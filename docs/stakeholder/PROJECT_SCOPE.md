# Project scope

A boundary between **what this take-home actually ships** and **what a
production deployment of the same architecture would assume**. Read this
before treating any source-doc caveat ("PoC shortcut," "dev-only,"
"deferred") as a gap.

Pairs with [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md),
which carries the conversations a production engagement would start
with. **Scope is the *what*; Discussions is the *what to talk through*.**

## In scope

A developer-facing reference implementation of the **team microservice +
service catalogue** pattern, runnable in one command (`docker compose up`)
and reproducible in GitHub Codespaces per the take-home requirements.

The author also maintains a personal Kubernetes cluster that hosts a live
demo of the auto-generated Helm chart; the chart works on that cluster,
**with no warranty for any other cluster** — storage classes, certificate
authorities, and mesh configuration on that cluster are author-specific.

## Out of scope — the production assumption set

Production deployment of the same architecture assumes a set of
infrastructure that this take-home **does not ship**:

- **Service mesh** providing automatic, mutual-TLS-secured service-to-service
  authentication based on workload identities. (Common implementations:
  Istio, Linkerd, Cilium with SPIFFE/SPIRE.)
- **Real secrets management** — a vendored or self-hosted secrets backend
  that replaces the local `secrets/` directory used during development.
  (Common implementations: HashiCorp Vault, External Secrets Operator,
  Sealed Secrets, or a cloud KMS.)
- **Container image registry** with signed builds, software-bill-of-materials
  (SBOM) generation, and provenance attestation.
- **Hardened container images** — slim, non-root, with real readiness and
  liveness probes — replacing the simpler PoC images.
- **CI/CD pipeline** that regenerates the Helm chart on every merge and
  publishes signed deployment artifacts.
- **Cluster ingress** wired to an organizational identity provider with
  real-CA TLS certificates (replacing the local CloudFlare tunnel demo).
- **Named storage classes** (`soc2_sensitive`, `fault_tolerant_cache`, etc.)
  provisioned at the cluster level — usually via Longhorn, a cloud-native
  storage provider, or a managed equivalent.
- **Horizontal scaling** — multiple collector replicas, clustered metrics
  database, stateless web tier behind a CDN.
- **Production-grade gRPC load balancing** between the front-end and back-end
  (replacing the development direct-call).
- **Structured logging and tracing backends** with retention policies,
  audit trails, and PII redaction (typically a logs catalogue module
  alongside the metrics module).
- **Per-service chart structure** — each service eventually owning its own
  chart once Conway's-law team boundaries solidify (rather than the single
  meta-chart this PoC ships).
- **Linter rules** that enforce catalogue compliance on team pull
  requests.
- **Provider choices** — cloud vs. on-prem, managed vs. self-hosted data
  tier, regional posture, compliance class.

**Provider choices** in particular would be the decisions a production
engagement *starts* with, not ends with.

## Implications for reviewers

- Anywhere the source docs say "PoC shortcut," "dev-only," or "deferred"
  — the production answer is in the list above.
- The single-stage container builds, the placeholder image tags, the dev
  password sitting in plaintext, the force-overwriting local certificate
  authority, and the placeholder hostnames are **deliberate**, not
  omissions. They are the simplest path that exercises the pattern in a
  developer environment.
- [`DESIGN_DECISIONS.md`](./DESIGN_DECISIONS.md) has a *"prod-considerations
  (row-specific)"* column for each design choice. Those entries are
  **row-specific signposts**, not commitments — the full cross-cutting
  infrastructure assumption set is here.
