> A reference implementation of the **microservice-template + service-catalogue**
> deployment pattern, demonstrated through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is
*one team's microservice repo* shaped the way an organization's CI/CD core
should support it: a top-level `docker-compose.yml` that `include:`s an
observability module from a stub service catalogue. Helm charts are
generated from compose, not hand-maintained. Architecture record:
[`docs/DESIGN_DECISIONS.md`](./docs/DESIGN_DECISIONS.md). Full design:
[`docs/superpowers/specs/2026-05-13-microservice-template-design.md`](./docs/superpowers/specs/2026-05-13-microservice-template-design.md).

## Design rules

Heuristics for choosing A vs B in the decisions below. They assume an
organization large enough that no single platform team can hand-hold every
product team's deploy; in a smaller org, the trade-offs invert.

**1. Compose-as-ceiling.** Prefer designs that bottom out at a
`docker-compose.yml` the team owns, over designs that require teams to
maintain Kubernetes manifests, Helm charts, or mesh policy directly.
- *Assumes:* product teams let infra they don't understand rot.
- *Decides:* `katenary` auto-derives the Helm chart from compose; teams never
  hand-edit charts.

**2. Catalogue over copy-paste.** Prefer published, versioned, ops-maintained
modules that teams `include:` over per-team reinvention of the same stack.
- *Assumes:* without a shared source, *N* teams converge on *N* divergent
  forks of the same observability/auth/cache layer.
- *Decides:* observability lives in `modules/observability/` as a catalogue
  module, not inline in the team's compose.

**3. Prescription beats flexibility for composable things.** Prefer a fixed
module shape — compose snippet + katenary labels + env interface + version —
over "module = whatever you call a module."
- *Assumes:* without prescription, every team renegotiates the integration
  contract from scratch.
- *Decides:* the catalogue is opinionated about a module's shape; teams use
  the shape, they don't redesign it.

**4. Defaults over knobs.** Prefer one sensible default that 80% of teams
accept over an exhaustive configuration surface that 100% of teams
misconfigure.
- *Assumes:* default-setters (ops) have more context than default-consumers
  (product teams).
- *Decides:* volume classes are *named* (`soc2_sensitive`,
  `fault_tolerant_cache`); image tags are *one* placeholder; mTLS has *one*
  dev workflow (`make mtls`).

**5. Labels over YAML for review surface.** Prefer surfacing important
decisions as labels next to the service definition over leaving them in
generated chart output.
- *Assumes:* reviewer attention is finite; what they read is what they
  catch.
- *Decides:* `katenary.v3/*` labels carry storage class, secret-vs-configmap
  routing, ingress, and mTLS wiring. The generated chart is build output.

**6. Off-the-shelf for undifferentiated; custom for differentiated.** Prefer
mature OSS where the team adds no value by building.
- *Assumes:* every dependency you maintain is one you pay for later, in
  on-call.
- *Decides:* the observability stack is bought (OpenTelemetry Collector,
  VictoriaMetrics, Grafana, the standard infra exporters); the Rust gRPC API
  is built — that's the team's product.

**7. Defer over-determined choices.** Prefer the battle-tested default over
making the "right" call with insufficient production context.
- *Assumes:* premature optimization compounds badly; reversibility is
  cheaper than precision.
- *Decides:* the TSDB ships unconfigured; schema, retention, and tenancy
  choices belong in a follow-up driven by real volume — not in the
  onboarding burden.

## Project documentation

- [`requirements.md`](./requirements.md) — original take-home requirements (read-only reference).
- [`docs/DESIGN_DECISIONS.md`](./docs/DESIGN_DECISIONS.md) — terse summary of the current state of each design choice. **Read first** to get the shape of the system in under a minute.
- [`docs/superpowers/`](./docs/superpowers/) — exploration artifacts, design specs, plans, and other agentic/research material. Detailed technical specs live under `docs/superpowers/specs/`.
