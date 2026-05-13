# Production discussions

This document is a **discovery framework** for stakeholders. It surfaces the
conversations that would precede taking this architecture to production —
not as a list of work items, but as a map of decisions to weigh against
the size and complexity of your business.

**This is not a project plan.** None of the topics below are committed work
in this take-home. They are the prompts you'd use to start a kickoff
conversation about taking this pattern to production.

**Companion docs:**
[`PROJECT_SCOPE.md`](./PROJECT_SCOPE.md) — what's in / out of scope.
[`ENGINEERING_RULES.md`](./ENGINEERING_RULES.md) — the rules that drove
the architecture. [`DESIGN_DECISIONS.md`](./DESIGN_DECISIONS.md) — current
state of each decision.

**How to read each topic:**

- The **bold headline** is the question you'll need to answer.
- The plain text is the context — what's at stake, what the common
  answers look like.
- The *italic line* at the end tells you when that question stops being
  trivial.

---

## When do these conversations matter?

Most of these conversations have a near-trivial answer for a small,
simple organization, and a load-bearing answer for an enterprise. The
same architecture serves both — what scales is the **depth of
conversation** behind each topic.

| Your situation | What this document looks like for you |
|---|---|
| **Small / simple** (1–3 teams, single region, internal users only) | Most topics have an obvious default. The conversations are short. Focus on Security, Reliability, and Provider & cost. |
| **Mid-stage** (10–50 teams, B2B customers, some regulatory exposure) | Half the topics become load-bearing. Catalogue Governance and CI/CD & Release Management start mattering at this stage. |
| **Enterprise** (100+ teams, multi-tenant SaaS, regulated, multi-region) | Every topic is on the table. None of the answers are nominal. |

**Scaling factors that drive conversation depth:**

- **Team count.** A few teams can share defaults; many teams need a
  catalogue and governance.
- **Customer profile.** Internal-only is forgiving; enterprise customers
  demand compliance proofs.
- **Regulatory exposure.** GDPR, HIPAA, SOC2, PCI each add their own
  required answers.
- **Geographic spread.** Single-region is simple; multi-region adds
  residency, replication, and failover.
- **Workload variety.** One product is a different shape from a platform
  hosting many.

Use this to triage which conversations to invest in first.

---

## Security, compliance & identity

- **Who can see whose data?** Reuse the same SSO that gates source-code
  access (GitHub, Okta, Active Directory, etc.) to gate observability —
  *if you can't see the repo, you can't see its metrics or logs*. Scales
  horizontally with team count and produces a clean enterprise-compliance
  story without bespoke per-tenant work. *— Matters more with team count
  and customer mix.*
- **How does a user log into an embedded dashboard?** Iframed dashboards
  need an auth handoff: forward-auth from the front-door identity
  provider, direct SSO into the dashboard tool, or a shared cookie
  domain. *— Matters more for customer-facing dashboards.*
- **How does a service prove its identity to another?** Service-to-service
  authentication needs a substrate: mesh-issued workload identity, cloud
  IAM-for-service-accounts, or mesh-native primitives. *— Matters more
  with zero-trust posture and regulatory exposure.*
- **Which service is allowed to call which?** Per-service authorization
  rules expressed as mesh policy or policy-as-code. *— Matters more with
  service count and blast-radius concerns.*
- **Who has the keys to the kingdom?** Operator access to clusters,
  registries, secrets vaults, and the catalogue. Where those actions are
  logged. Integration with the security team's existing SIEM. *— Matters
  more with operator count and regulatory exposure.*
- **Where do secrets live?** Pick a backend that matches what other
  workloads in the organization already use — Vault, External Secrets,
  Sealed Secrets, cloud KMS. Avoid running multiple. *— Matters more
  with workload count and audit requirements.*
- **What gets redacted, where, and by whom?** PII filtering happens
  somewhere in the logging pipeline — at the collector, the aggregator,
  or in the application. Policy-driven or per-team contract. *— Matters
  more with user-data sensitivity and regulatory exposure.*
- **What compliance tier is each workload?** A single human-readable
  label (`soc2`, `hipaa`, `pci`, `internal`, etc.) on each team's
  workload, flowing automatically to storage class, audit retention, and
  policy strictness. *— Matters more with regulatory diversity.*

## Reliability, scale & multi-region

- **What service levels do customers expect?** Per-tier availability and
  latency targets; how error budgets shape alerting and release
  cadence. *— Matters more with customer SLA commitments.*
- **What's the geographic posture?** Active-active, active-passive, or
  single-region. Replication semantics for storage and the catalogue.
  *— Matters more with geographic customer spread and regulatory
  residency.*
- **How does the stack scale horizontally?** Per-component patterns —
  collector replicas, time-series clustering, stateless web tier behind
  a CDN. Auto-scaling triggers. *— Matters more with traffic volume and
  team count.*
- **How do teams know how big to ask?** Catalogue presets (small /
  medium / large) vs. team-driven estimation. *— Matters more with
  resource pressure and team count.*
- **How much metric cardinality can each team consume?** Per-team
  budgets and enforcement; what teams see when they exceed.
  *— Matters more with shared-infrastructure pressure.*
- **How do we roll back when something breaks?** Per-component — image
  rollback for stateless services, config rollback for catalogue
  modules, schema reversibility for stateful ones. *— Matters more with
  deploy frequency and change-failure tolerance.*

## Storage, data lifecycle & residency

- **What named storage classes does the catalogue offer?** Canonical
  list (e.g., `soc2_sensitive`, `fault_tolerant_cache`,
  `ephemeral_scratch`); what each enforces (encryption, replication,
  audit, region); who can add new classes. *— Matters more with workload
  variety and compliance class diversity.*
- **How long do we keep what?** Per-data-type retention — metrics, logs,
  traces — windows, downsampling, cold-storage handoff. *— Matters more
  with regulatory retention and storage cost.*
- **What's the backup and DR posture?** Cadence per workload class;
  restore-time objectives; tabletop exercise frequency. *— Matters more
  with revenue-at-risk and compliance obligations.*
- **How are tenants isolated?** Label-scoped on a shared store,
  namespace-isolated, or dedicated tenancy per compliance class.
  *— Matters more with multi-tenancy and customer-data segregation
  requirements.*
- **Which data lives in which region?** Region pinning per workload
  class; gating cross-region transit. *— Matters more with geographic
  customer base and regulatory residency.*

## CI/CD & release management

- **What's the promotion path?** Dev → staging → prod environments;
  automatic gates vs. human gates. *— Matters more with deploy
  frequency and change-risk profile.*
- **What triggers a deploy?** Push to main, tagged release, PR
  label-driven, scheduled. Per-team overrides. *— Matters more with
  team variety.*
- **What deployment strategy?** Canary, blue-green, rolling — per
  component, with opt-outs. *— Matters more with change-failure cost.*
- **What does the supply chain require?** Image signing, SBOMs,
  provenance attestation, vulnerability-scan promotion gates.
  *— Matters more with regulatory exposure and enterprise customer
  demands.*
- **What's the contract between team and catalogue?** What teams owe the
  catalogue's CI hooks (`make test`, `make build`, `make verify`) and
  what the catalogue provides back. *— Matters more with team count.*

## Catalogue governance

- **How are catalogue modules versioned?** SemVer; per-module release
  cadence; how breaking changes are communicated to consuming teams.
  *— Matters more with module count and team count.*
- **How do teams migrate when a module breaks?** Runway for breaking
  changes; codemod or automated-PR support. *— Matters more with
  breaking-change frequency.*
- **How do modules retire?** Deprecation policy; support windows;
  replacement discoverability. *— Matters more with module-lineage
  complexity.*
- **Which catalogue rules block merge?** Severity model — what's a
  hard-stop vs. a warning; how exemptions are granted and audited.
  *— Matters more with compliance enforcement.*
- **Who owns each module?** Platform team, SRE rotation, or a specific
  team. Handoff process when ownership changes. *— Matters more with
  module count and personnel churn.*
- **How do teams contribute new modules?** Proposal flow; promotion to
  "official." *— Matters more with desire for democratic platform
  evolution.*
- **How do dev includes become prod includes?** URL-based includes,
  OCI-distributed snippets, CI substitution, Helm chart dependencies.
  *— Matters more with multi-environment promotion strictness.*

## Observability hardening

- **Where do logs go?** A logs catalogue module — Loki / ELK / vendor —
  packaged the same way as the metrics module. *— Matters more with
  audit and debugging volume.*
- **Where do traces go?** Tempo / Jaeger / vendor; head- vs.
  tail-based sampling. *— Matters more with service count.*
- **How are alerts authored and versioned?** Alongside dashboards-as-code
  or separate. Integration with the on-call system. *— Matters more
  with on-call team size and SLO strictness.*
- **Are dashboards code or UI-edited?** Per-team vs. shared library;
  review path for changes. *— Matters more with dashboard count and
  governance requirements.*
- **How do humans get paged?** PagerDuty, OpsGenie, vendor. How teams
  declare their rotation. *— Matters more with team count and SLA
  commitments.*

## Networking

- **What's the ingress strategy?** Mesh-native gateway, cloud load
  balancer, or Kubernetes ingress controller. *— Matters more with
  public-facing surface area.*
- **Where does mTLS get enforced?** Mesh sidecar everywhere, ambient
  mesh, or selectively at certain hops. *— Matters more with zero-trust
  posture.*
- **Can teams reach external services?** Per-team egress allowlists;
  how the catalogue exposes them. *— Matters more with security and
  compliance enforcement.*

## Provider & cost

- **Cloud, on-prem, or hybrid?** Top-level provider posture; vendor-lock
  tolerance; portability requirements. *— Matters more with regulatory
  or commercial constraints on hosting.*
- **What do we run, what do we buy?** Per-service managed vs.
  self-hosted decision matrix — managed Grafana, managed Postgres,
  managed Vault. *— Matters more with operational headcount and total
  cost of ownership.*
- **How is cost attributed back to teams?** Chargeback model; team
  spend visibility; budget alarms. *— Matters more with team count and
  finance-team scrutiny.*
- **What can a team consume?** Per-team CPU, memory, storage, and
  cardinality quotas. *— Matters more with shared-cluster pressure.*

---

## What to do with this document

In a kickoff conversation with stakeholders, walk the sections relevant
to your situation (use the *— Matters more with* tags to triage), and
capture which questions have known answers, which need investment, and
which can be deferred. The output becomes a real project plan; this
document remains the source-of-truth checklist the plan can be reviewed
against.
