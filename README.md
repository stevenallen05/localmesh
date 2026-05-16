# Tensorwave take-home

> A reference implementation of the **microservice-template + service-catalogue**
> deployment pattern, demonstrated through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is
*one team's microservice repo* shaped the way an organization's CI/CD core
should support it: a top-level `docker-compose.yml` that `include:`s
catalogue modules — `observability/`, `security/`, `logging/` (required)
and `database/` (optional) — from a stub service catalogue. Helm charts
are generated from compose, not hand-maintained.

## Design intent

A framework for compartmentalized microservices — a working look at the
developer experience that emerges when off-the-shelf tooling is chained
together. Teams' day-to-day input is compose, so they handle their own
infra needs from a searchable catalogue. The output is Helm, so the
chart drops into any helm-speaking infra. The prescriptive middle layer
shifts SRE workload off per-team toil and onto larger infra and
catalogue investments.

[`project.toml`](./project.toml) is where team work, SRE oversight, and
business requirements intersect — one human-scale file that captures
all three, auditable and controllable at whatever level the context
demands. (Expanded elsewhere — see [`docs/stakeholder/`](./docs/stakeholder/).)

**Production-quality on dev becomes production.** Each catalogue module
ships the same shape ops would run in prod, just locally-hosted: dev
gets the *real* observability stack (OpenTelemetry collector, Tempo,
Loki, VictoriaMetrics, Grafana), the *real* logging contract, the
*real* identity tuple — the prod overlay swaps the storage backends and
the rest is the same wire. The reviewer's dashboard is the community
[Lightweight APM for OpenTelemetry](https://grafana.com/grafana/dashboards/22784)
(Grafana ID 22784, source [cyrille-leclerc/opentelemetry-service-dashboard](https://github.com/cyrille-leclerc/opentelemetry-service-dashboard))
imported wholesale — it works against this stack because the apps emit
OTel semantic conventions, and it'll work against any backend that
honors them.

## Read these first

Stakeholder-facing context — read these regardless of role:

- [`docs/stakeholder/ENGINEERING_RULES.md`](./docs/stakeholder/ENGINEERING_RULES.md) —
  the design rules tied to business needs. Read these to understand the *why*.
- [`docs/stakeholder/PROJECT_SCOPE.md`](./docs/stakeholder/PROJECT_SCOPE.md) —
  the boundary between what this take-home actually ships and what a
  production deployment would assume.
- [`docs/stakeholder/PRODUCTION_DISCUSSIONS.md`](./docs/stakeholder/PRODUCTION_DISCUSSIONS.md) —
  the **discovery framework** for a real prod engagement, sized by the
  complexity of your business.
- [`docs/stakeholder/PRODUCTION_DECISION_MATRIX.md`](./docs/stakeholder/PRODUCTION_DECISION_MATRIX.md) —
  empty research scaffold for evaluating candidate solutions to each of
  those decisions (3-star ratings × ≤7 dimensions).
- [`docs/stakeholder/DESIGN_DECISIONS.md`](./docs/stakeholder/DESIGN_DECISIONS.md) —
  current state of every design choice, one row each (dev choice,
  rationale, production-considerations signposts).

Engineering-side material (code-facing implementation rules):

- [`docs/engineering/rules/`](./docs/engineering/rules/) — stack-specific
  style notes for Rust and the compose→Helm tool (katenary).

## All project documentation

- [`requirements.md`](./requirements.md) — original take-home requirements (read-only reference).
- [`docs/stakeholder/`](./docs/stakeholder/) — stakeholder-facing documents (rules, scope, discussions, decisions).
- [`docs/engineering/`](./docs/engineering/) — engineer-facing implementation rules.
- [`docs/superpowers/`](./docs/superpowers/) — exploration artifacts, design specs, plans, and other agentic/research material. Detailed technical specs live under `docs/superpowers/specs/`.

## Run locally

Prereq: `pipx` (the `make setup` target uses it to run `pre-commit`).

First time only (or after wiping `grafana-data`):

```bash
make setup
```

This forces Grafana's admin password to `admin` and mints a service-account token the Rust server reads on startup. Then:

```bash
docker compose up -d --build
```

## What to look at

**Grafana** at `localhost:3001` (admin / admin), two provisioned dashboards:

- **Lightweight APM for OpenTelemetry** (`/d/apm`) — community dashboard
  [22784](https://grafana.com/grafana/dashboards/22784) by Cyrille Le
  Clerc ([source](https://github.com/cyrille-leclerc/opentelemetry-service-dashboard)),
  imported with three small patches: datasource defaults, identity-tuple
  defaults (`tw-demo`/`dev`), and template-var queries that source from
  `label_values()` directly because VictoriaMetrics 1.106 doesn't expose
  Prometheus 3.x's `keep_identifying_resource_attributes` knob. The fact
  that an unmodified dashboard from the broader OTel community works
  against this stack at all is the OTel-semconv contract paying off.
- **Cluster: size & health** (`/d/cluster-health`) — host/container
  resources (cadvisor + node-exporter) plus a module-roster panel listing
  every catalogued module's identity.

**www** at `localhost:3000` — three demo buttons (`PrintPostgresStats`,
`ListGrafanaDatasources`, `TestRPC`) that exercise the trace path
end-to-end (Next.js → tonic → Rust → postgres). Each click produces a
trace visible in the APM dashboard's Tempo waterfall panel.

**Tempo** + **Loki** are reachable via the APM dashboard's panels;
direct queries via Grafana's Explore.
