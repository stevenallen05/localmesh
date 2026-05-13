# Production discussions

Canonical agenda for the conversations that would precede taking this
architecture to production. **Not commitments, not decisions** — the
discussions we'd need to have, at the level of detail useful for a
project-shape conversation rather than implementation.

Pairs with:

- [`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md) — the rules each
  resulting decision should respect.
- [`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md) — the assumption set
  separating dev from prod.
- [`DESIGN_DECISIONS.md`](./DESIGN_DECISIONS.md) — the dev-side current
  state of each choice.

---

## Security, compliance & identity

- **SSO-scoped observability.** Production metrics and logs scoped by the
  same dimension that gates the source code (GitHub repo permissions,
  team membership, environment label) via SSO. *If you can't see the repo
  or environment, you can't see its data in the dashboards.* Scales
  horizontally with team count; produces a clean enterprise-customer
  compliance story without bespoke per-tenant work.
- **Embedded-dashboard auth.** How iframe-embedded panels inherit the
  user's session — forward-auth from the front-door identity provider,
  OIDC directly into Grafana, or a shared cookie domain.
- **Workload identity.** SPIFFE/SPIRE vs. cloud-native IAM-for-service-accounts
  vs. mesh-native primitives. Which CA trust chain is canonical; SVID
  rotation timing.
- **Service-to-service permissions.** Mesh-policy-backed RBAC; what the
  human-readable interface is (CRDs, Rego, mesh-native syntax).
- **Operator auth & audit.** Who can touch the cluster, registries,
  Vault, the catalogue. Where actions get logged. Integration with the
  org's SIEM.
- **Secrets backend.** ESO / Sealed Secrets / Vault CSI / cloud KMS —
  driven by what other workloads in the org already use.
- **PII redaction.** Where it happens in the pipeline (collector
  processor, log aggregator, application code). Policy-driven via tags
  vs. per-team contracts.
- **Per-workload compliance class.** A single human-reviewable label
  (`soc2_sensitive`, `hipaa`, `pci`, `gdpr`, `internal`) flows from team
  to storage class, audit retention window, and mesh policy strictness.

## Reliability, scale & multi-region

- **SLOs per layer.** Availability + latency budgets per service tier;
  error budgets; how budgets attach to alerting.
- **Multi-region posture.** Active-active / active-passive /
  single-region. Replication semantics for the TSDB and the catalogue.
- **Horizontal-scale topology.** Per-component replica/cluster patterns
  (collector replicas, TSDB cluster mode, stateless web tier behind CDN);
  scaling triggers.
- **Capacity planning model.** How teams estimate resource needs;
  whether the catalogue ships small/medium/large presets.
- **Cardinality budgets.** Per-team metric cardinality limits;
  enforcement model; what teams see when they exceed.
- **Rollback strategy.** Per-component — image rollback for app servers,
  config rollback for catalogue modules, schema-migration reversibility
  for stateful services.

## Storage, data lifecycle & residency

- **Named storage class registry.** Canonical list, what each class
  enforces (encryption, redundancy, audit, region pinning), who owns
  additions.
- **Retention policy.** Per data class — metrics, logs, traces —
  retention windows; downsampling thresholds; cold-storage handoff.
- **Backup & DR.** Cadence per workload class; restore-time objectives;
  tabletop frequency.
- **Tenant isolation.** Shared TSDB with label-based scoping vs.
  namespace-isolated vs. dedicated tenancy per compliance class.
- **Data residency.** Which regions hold which workload classes;
  cross-region transit gating.

## CI/CD & release management

- **Promotion path.** Dev → staging → prod environments; which gates
  automatic, which human.
- **Trigger conventions.** Push to main vs. tagged release vs. PR labels;
  per-team overrides.
- **Deployment strategy.** Canary / blue-green / rolling — per-component
  defaults; opt-outs.
- **Supply-chain security.** Image signing (cosign / sigstore); SBOM
  generation; provenance attestation; vulnerability-scan thresholds that
  block promotion.
- **Per-team CI contract.** What teams owe the catalogue's CI hooks
  (`make test`, `make build`, `make verify`, etc.) and what the catalogue
  provides in return.

## Catalogue governance

- **Module versioning.** SemVer; per-module release cadence; how
  breaking changes are flagged to consuming teams.
- **Migration paths.** When a module ships a breaking change, the
  consuming team's runway; codemod / automated-PR support.
- **Module deprecation.** Sunsetting policy; support window for
  deprecated modules; replacement discoverability.
- **Linter rule severity.** Which catalogue-compliance rules block merge
  vs. warn; how exemptions are granted and audited.
- **Ownership model.** Per module — platform team, SRE rotation, or a
  specific team. Handoff process when ownership changes.
- **Contribution flow.** How a team proposes a new module; how a module
  becomes "official."
- **Module dev↔prod swap mechanic.** URL include / OCI-distributed
  compose snippet / CI substitution / Helm chart dependency — which one
  is canonical; how it interacts with versioning.

## Observability hardening

- **Logs catalogue module.** Loki / ELK / vendor — packaged as a
  catalogue module alongside the metrics module.
- **Tracing backend.** Tempo / Jaeger / vendor; head- vs. tail-based
  sampling decisions; cross-service trace propagation expectations.
- **Alerting infrastructure.** Grafana Alerting / Alertmanager / vendor;
  how alert rules are versioned (alongside the dashboard module).
- **Dashboard provisioning.** Dashboards-as-code vs. UI-edited;
  per-team vs. catalogue-shared library; review path for changes.
- **On-call tooling.** PagerDuty / OpsGenie / vendor; how teams declare
  their rotation.

## Networking

- **Ingress strategy.** Mesh-native gateway / cloud LB / Kubernetes
  ingress controller. Per-environment defaults.
- **East-west traffic.** Mesh sidecar vs. ambient mesh; mTLS enforcement
  scope (everywhere vs. selected hops).
- **Egress controls.** Per-team allowlists for cluster-external
  destinations; how the catalogue exposes them.

## Provider & cost

- **Cloud / on-prem / hybrid.** Top-level provider posture;
  vendor-locked vs. portable services.
- **Managed vs. self-hosted.** Per-service decision matrix — when does
  managed Grafana / Postgres / Vault beat the self-hosted catalogue
  module.
- **Per-team chargeback.** Cost attribution model; team spend
  visibility; budget alarms.
- **Resource quotas.** Per-team limits on CPU, memory, storage, metric
  cardinality.
