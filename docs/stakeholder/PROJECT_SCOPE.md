# Project scope

Boundary between what this take-home actually ships and what a production deployment of the same architecture would assume.

Pairs with [`PRODUCTION_DISCUSSIONS.md`](./PRODUCTION_DISCUSSIONS.md). **Scope is the *what*; Discussions is the *what to talk through*.**

## In scope

A reference implementation of the **team microservice + service catalogue** pattern, runnable in one command (`docker compose up`) and reproducible in GitHub Codespaces per the take-home requirements.

The author also runs a personal Kubernetes cluster hosting a live demo of the auto-generated Helm chart. The chart works there, **no warranty for other clusters** — storage classes, CAs (Certificate Authorities), and mesh configuration are author-specific.

## Out of scope — the production assumption set

A production deployment of the same architecture assumes infrastructure this take-home does not ship:

- **Service mesh** with auto-mTLS (mutual TLS) service-to-service auth keyed to workload identity. (Istio / Linkerd / Cilium with SPIFFE-SPIRE.)
- **Real secrets backend** replacing the local `secrets/` directory. (Vault / External Secrets / Sealed Secrets / cloud KMS (Key Management Service).)
- **Container registry** with signed builds, SBOM (Software Bill of Materials), and provenance attestation.
- **Hardened images** — slim, non-root, real readiness/liveness probes.
- **CI/CD (Continuous Integration / Continuous Delivery) pipeline** that regenerates the Helm chart on every merge and publishes signed artifacts.
- **Cluster ingress** wired to an org IdP (Identity Provider) with real-CA TLS, replacing the CloudFlare-tunnel demo.
- **Named storage classes** (`soc2_sensitive`, `fault_tolerant_cache`, etc.) provisioned at the cluster level. (Longhorn / cloud storage / managed.)
- **Horizontal scaling** — collector replicas, clustered TSDB (Time-Series Database), stateless web tier behind a CDN (Content Delivery Network).
- **Production gRPC load balancing** between front-end and back-end, replacing the direct call.
- **Logs and traces backends** with retention, audit, and PII (Personally Identifiable Information) redaction — typically a logs catalogue module alongside the metrics module.
- **Per-service chart structure** once Conway's-law team boundaries solidify, rather than this PoC's (Proof of Concept) single meta-chart.
- **Linter rules** enforcing catalogue compliance on team PRs.
- **Provider choices** — cloud vs. on-prem, managed vs. self-hosted, regional posture, compliance class.

Provider choices in particular are decisions a production engagement *starts* with, not ends with.

## Implications for reviewers

Anywhere the source docs say *"PoC shortcut," "dev-only,"* or *"deferred,"* the production answer is in the list above. The single-stage container builds, placeholder image tags, plaintext dev password, force-overwriting local CA, and placeholder hostnames are **deliberate** — the simplest path that exercises the pattern in a developer environment.
