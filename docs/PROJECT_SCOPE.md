# Project scope

A boundary between **what this take-home actually ships** and **what a
production deployment of the same architecture would assume.** Read this
before treating any source-doc caveat ("PoC shortcut," "dev-only,"
"deferred") as a gap.

## In scope

A developer-facing reference implementation of the **team microservice +
service catalogue** pattern, runnable via `docker compose up` (including
in GitHub Codespaces per the take-home requirements). The author maintains
a personal Kubernetes cluster hosting a live demo of the generated chart;
the chart works on that cluster, with **no warranty for any other
cluster** (storage class names, certificate authorities, and mesh
configuration are author-specific).

## Out of scope — the production assumption set

Production deployment of the same architecture assumes infrastructure this
take-home does not ship:

- **Service mesh** (Istio / Linkerd / Cilium) with workload-identity-issued
  mTLS (SPIFFE/SPIRE or equivalent) for cross-service authentication.
- **Real secrets backend** (External Secrets Operator, Sealed Secrets,
  Vault CSI, or cloud KMS) replacing the bind-mounted dev `secrets/`
  directory.
- **OCI registry** for image distribution, with signed builds, SBOMs, and
  provenance attestation.
- **Hardened container images** — multi-stage builds, distroless base,
  non-root `USER`, real healthchecks.
- **CI/CD pipeline** that regenerates the Helm chart on each merge and
  publishes signed artifacts.
- **Cluster ingress** with cert-manager and an organizational identity
  provider.
- **Named storage classes** (e.g., `soc2_sensitive`,
  `fault_tolerant_cache`) provisioned by Longhorn or a managed equivalent.
- **Horizontal-scale topology** — multiple collector replicas, TSDB
  cluster mode, stateless web tier behind a CDN.
- **Real gRPC load balancer** — Envoy, grpc-web bridge, or mesh sidecar —
  replacing the dev direct-dial.
- **Structured observability backends** with retention, audit, and PII
  redaction (e.g., a logs catalogue module).
- **Per-service chart structure** — each service its own chart, top-level
  chart as meta — once Conway's-law boundaries solidify.
- **Linter rules** enforcing catalogue compliance on team PRs.
- **Provider choices** — cloud vs. on-prem, managed services for the data
  tier, regional posture, compliance class.

Provider choices in particular would be the decisions a production
engagement *starts* with, not ends with.

## Implications for reviewers

- Anywhere the source docs say "PoC shortcut," "dev-only," "deferred," or
  reference a production posture, the cross-cutting detail lives here.
- The single-stage Dockerfiles, hardcoded image tags, plaintext dev
  passwords, force-overriding mTLS dev keys, placeholder hostnames, and
  absent real healthchecks are **deliberate**, not omissions. They are
  the simplest path that exercises the pattern in a developer environment.
- `DESIGN_DECISIONS.md` has a "Possible prod considerations & choices"
  column. Those entries are **row-specific signposts**, not commitments —
  the cross-cutting infrastructure assumptions all bottom out here.
