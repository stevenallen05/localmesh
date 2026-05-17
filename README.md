# Tensorwave take-home

**LocalMesh turns your `docker-compose.yaml` into the helm chart your prod cluster runs.** Teams write compose; SRE maintains a catalogue of one-line `include:` plugins (postgres, queues, observability, secrets). Local dev = prod by construction — no drift, no platform tickets.

> A reference implementation of the **microservice-template + service-catalogue**
> deployment pattern, demonstrated through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is
*one team's microservice repo* shaped the way an organization's CI/CD core
should support it: a top-level `docker-compose.yml` that `include:`s
catalogue modules — `observability/`, `security/`, `logging/` (required)
and `database/` (optional) — from a stub service catalogue. Helm charts
are generated from compose, not hand-maintained.

## LocalMesh

**LocalMesh is your local development environment, compiled into production.**
A service mesh is the network layer between your services — encryption,
identity, routing, observability — and if you're running k8s, you already
have one, deliberately or not. LocalMesh is the framework that turns the
`docker-compose.yaml` your team already writes into the helm chart your
prod cluster already runs. The trick is omakase: SRE maintains a small,
opinionated catalogue of infrastructure plugins, each with sensible
defaults solving the hard parts (mTLS, identity propagation, log
structure, metric naming, cert rotation) the way every team would
otherwise reinvent. Devs get a two-tier interface: one mandatory
`include:` line at the top of compose pulls the baseline SRE has decided
every project gets (observability, the mesh, the company's required MQ,
feature flags), and one more line per plugin the team picks out of the
catalogue. The catalogue is whatever human-searchable form SRE finds
easiest to keep up — a wiki page, an AWS Service Catalog instance, a
folder of git links — and the install instructions for any plugin are
the same: "copy this line into the top of your `docker-compose.yaml`."

A plugin is anything with a compose half (runs locally, exposes the
configs/env vars/volumes the app needs) and a helm or terraform half
(provisions the production version with matching presets). A "service"
is anything that can be provisioned via helm and exposes some
dev-facing handle (env var, config file, mounted volume, named secret).
If it fits that shape, it fits the catalogue:

- **Databases** — prod: Aurora, managed Postgres, Cloud SQL. Local: postgres container with the same extensions, exporter, and wire shape.
- **Queues & messaging** — prod: SQS/SNS, Confluent Kafka, managed RabbitMQ. Local: RabbitMQ, NATS, or Redpanda in compose.
- **Object storage** — prod: S3 with the right lifecycle rules and IAM. Local: MinIO.
- **Secrets** — prod: Vault, AWS Secrets Manager, sealed-secrets. Local: dev CA and env-injected tokens.
- **Observability** — prod: managed Loki/Tempo/Mimir or Grafana Cloud. Local: full stack in compose (OTel collector + Tempo + Loki + VictoriaMetrics + Grafana — this repo's `observability/` module).
- **Provisioned resources** — prod: a real domain via Route53/Cloudflare, an API key minted from Stripe, a GPU node pool. Local: a stub domain, a sandbox API key, a CPU-only mock.

What falls out once the mesh is in place: organization-wide security
guarantees that don't depend on each team remembering them (encryption
between services, identity on every call, default-deny network policy,
an audit log of who-talked-to-what); one machine-readable file per
project — [`project.toml`](./project.toml) — that captures identity,
compliance flags, data residency, billing, and ownership in a form
compliance/legal/billing/ops can all read directly; and a baseline of
observability and structured logging every service gets for free.

LocalMesh is hands-off about what's *in* the catalogue and what the
rendered helm gets deployed to. SRE owns those choices — they're tied
to your production environment, your support bandwidth, your cloud,
your compliance posture. A homelab catalogue might be five plugins; a
midsize company's might be fifty. Both are valid LocalMesh deployments.
The framework's job is to keep the dev/SRE boundary clean: compose in,
helm out, strict translation between, and the rest is yours. Long
form: [`docs/WHAT_IS_MESH.md`](./docs/WHAT_IS_MESH.md).

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

**Grafana** at `localhost:3001` (admin / admin), three provisioned dashboards:

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
- **PostgreSQL Database** (`/d/database-postgres`) — community dashboard
  [9628](https://grafana.com/grafana/dashboards/9628) by Lucas Estienne,
  imported with patches: datasource UIDs rebound to our `vm`; uid pinned
  to `database-postgres`; Kubernetes-only `release=` / `namespace=` filters
  stripped (we don't run on k8s in dev). Adds one custom **top-N slow queries**
  table panel sourced from `pg_stat_statements` — no community dashboard
  surveyed had a slow-query panel matching our exporter's metric names.

**www** at `localhost:3000` — three demo buttons (`PrintPostgresStats`,
`ListGrafanaDatasources`, `TestRPC`) that exercise the trace path
end-to-end (Next.js → tonic → Rust → postgres). Each click produces a
trace visible in the APM dashboard's Tempo waterfall panel.

**Tempo** + **Loki** are reachable via the APM dashboard's panels;
direct queries via Grafana's Explore.
