> A reference implementation of the **microservice-template + service-catalogue**
> deployment pattern, demonstrated through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is
*one team's microservice repo* shaped the way an organization's CI/CD core
should support it: a top-level `docker-compose.yml` that `include:`s an
observability module from a stub service catalogue. Helm charts are
generated from compose, not hand-maintained. Architecture record:
[`docs/DESIGN_DECISIONS.md`](./docs/DESIGN_DECISIONS.md). Full design:
[`docs/superpowers/specs/2026-05-13-microservice-template-design.md`](./docs/superpowers/specs/2026-05-13-microservice-template-design.md).

## Design posture

A few rules, applied throughout:

- **If you want people to do it, you have to make it easy.** Compose is the
  practical ceiling of what most product teams will reasonably maintain on
  their own. Asking them to also own Helm charts, Kubernetes manifests, mesh
  policy, Vault roles, and Grafana datasources is how platform teams burn
  out trying to onboard new ones. Compose `include:` lets a team write
  *one* file and pull pre-built pieces from a catalogue.

- **Making complexity easy requires being prescriptive.** A "catalogue
  module" is a defined shape — compose snippet, katenary labels, env
  interface, published version — not "whatever you want to call a module."
  The prescription is what makes the result composable across teams instead
  of N bespoke observability stacks growing in parallel.

- **Sensible defaults over infinite configuration.** Volume classes are
  named (`soc2_sensitive`, `fault_tolerant_cache`), not free-form. mTLS has
  *one* dev workflow (`make mtls`). Image tags get *one* placeholder until CI
  knows better. Every defaulted choice can be overridden; almost none should
  need to be.

- **Human-reviewable in PR labels, not buried in YAML.** The decisions that
  matter — what storage class this volume needs, which secrets are
  sensitive, which services get ingress, which need mTLS — surface as
  `katenary.v3/*` labels next to the service definition. A reviewer reads
  the labels; the generated chart is build output.

- **Off-the-shelf where possible; custom where it differentiates.** The
  observability stack is entirely off-the-shelf (OpenTelemetry Collector,
  VictoriaMetrics, Grafana, node-exporter, cadvisor). The Rust gRPC API is
  custom — that's the team's product. Build-vs-buy is decided per slot, not
  per repo.

- **Storage decisions defer to where production context exists.** Picking a
  TSDB schema without knowing volume, retention SLA, query patterns, or
  tenancy model is premature. The catalogue ships a battle-tested default;
  the specifics are an ops-team conversation, not a team-onboarding burden.

## Project documentation

- [`requirements.md`](./requirements.md) — original take-home requirements (read-only reference).
- [`docs/DESIGN_DECISIONS.md`](./docs/DESIGN_DECISIONS.md) — terse summary of the current state of each design choice. **Read first** to get the shape of the system in under a minute.
- [`docs/superpowers/`](./docs/superpowers/) — exploration artifacts, design specs, plans, and other agentic/research material. Detailed technical specs live under `docs/superpowers/specs/`.
