# Tensorwave take-home

> A reference implementation of the **LocalMesh** deployment pattern, demonstrated through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is *one team's microservice repo* shaped the way LocalMesh expects: a top-level `docker-compose.yml` that `include:`s the org's `localmesh.yaml` plus a few LocalMesh plugins from a stub catalogue (observability, security, logging, database). The helm chart is generated from compose, not hand-maintained.

## TL;DR

**LocalMesh turns the `docker-compose.yaml` your teams already write for local dev into a helm chart your release pipeline can deploy.** It borrows Rails' omakase philosophy — opinionated curation of the boring infrastructure choices every project repeats — and applies it to the dev/prod gap. SRE assembles the catalogue; teams include one line per piece they need.

## What "omakase" means here

Rails put a name on a stance every team rediscovers: an opinionated framework that picks the boring choices on your behalf — the ORM, the routing layer, the templating, the dozen libraries every web app needs — so devs spend their time on the parts of the app that are actually theirs. The wins come from *not deciding*. Defaults that someone good has already picked, applied uniformly, beat bespoke decisions made by every team in parallel.

LocalMesh borrows the move for infrastructure. The boring choices most projects make — which postgres, which queue, what observability looks like, where secrets live, how identity propagates — get curated by SRE once, packaged as a catalogue of plugins, and pulled in by teams with one line in `docker-compose.yaml`. Devs accept the defaults, and SRE owns the prod posture. Both sides win the same way the Rails crowd does: less time spent on solved problems.

## What a service mesh is, briefly

Picture a condo tower. Each service is a unit; the team owns what's inside. SRE owns the building — wiring, plumbing, the shared amenities. A service mesh is the building.

Concretely, it's the layer of network infrastructure that handles everything happening *between* services: encryption, identity, routing, observability. The boundary is loose on purpose — anything ops can install for the whole building counts. Every cross-service call goes through it; teams don't write code in each app to make it happen. It's wired in once, applied to every service automatically. Under the hood, the keycard is mTLS — every service holds one the mesh issues at startup, the way your condo card opens certain floors and not others. To devs, it's just a card.

**And LocalMesh is that same trick, applied to docker-compose.** Where a service mesh imposes opinionated curation on the network layer, LocalMesh imposes it on the compose layer — so what teams run locally is the same shape as what prod installs. Long form: [`docs/WHAT_IS_MESH.md`](./docs/WHAT_IS_MESH.md).

## The dev/prod gap

In most companies, prod and local dev disagree about reality. Prod gets the real load balancer, the real identity provider, the real managed database, the real secrets backend; local dev gets shims, mocks, or "works on my machine." Drift accumulates one component at a time. SRE spends their week closing it ticket by ticket — a developer hits a thing prod does that compose doesn't, files a platform ticket, waits.

LocalMesh closes the gap by making local dev a literal compile target of prod. Same plugins, same defaults, same wire shape. Fix a bug in local, prod has the fix on the next deploy. Devs maintain prod for SRE without ever opening a ticket.

## The parts of LocalMesh

Before the parts: here's what it looks like in a team's repo.

```yaml
# A team's docker-compose.yaml
include:
  - repo:/localmesh.yaml          # one line — that's "LocalMesh installed"
  - repo:/database/postgres.yaml  # picked from the catalogue
  - repo:/cache/redis.yaml        # picked from the catalogue

services:
  api:
    image: myteam/api:1.4.0
    environment:
      DATABASE_URL: ${POSTGRES_URL}   # wired in by the postgres plugin
      REDIS_URL:    ${REDIS_URL}      # wired in by the redis plugin
```

```yaml
# service_catalog/localmesh.yaml — assembled by SRE, one per org
include:
  - repo:/mesh/baseline.yaml
  - repo:/observability/baseline.yaml
  - repo:/security/baseline.yaml
  # ...whatever this org requires
```

```yaml
# service_catalog/database/postgres.yaml — what SRE puts in the catalogue
services:
  postgres:
    image: postgres:16
    ports: ["5432"]
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: app
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    labels:
      katenary.v3/secrets: |-
        - POSTGRES_PASSWORD
      # The compose half above runs locally. The prod half is this helm chart.
      katenary.v3/dependencies: |-
        - name: postgresql
          repository: oci://registry-1.docker.io/bitnamicharts
          values:
            auth:
              database: app
              username: app

      # Alternatives — swap in to match your prod posture:
      #   katenary.v3/dependencies: |-
      #     - name: cnpg-cluster
      #       repository: https://helm.internal.example.com/charts        # self-hosted CloudNativePG
      #
      #   katenary.v3/dependencies: |-
      #     - name: aws-rds-shim
      #       repository: oci://helm.internal.example.com                 # AWS RDS via Crossplane
```

```yaml
# service_catalog/cache/redis.yaml
services:
  redis:
    image: redis:7-alpine
    ports: ["6379"]
    labels:
      katenary.v3/dependencies: |-
        - name: redis
          repository: oci://registry-1.docker.io/bitnamicharts

      # Alternatives:
      #   katenary.v3/dependencies: |-
      #     - name: redis-sentinel
      #       repository: https://helm.internal.example.com/charts         # self-hosted HA
      #
      #   katenary.v3/dependencies: |-
      #     - name: elasticache-shim
      #       repository: oci://helm.internal.example.com                  # AWS ElastiCache
```

The dev/SRE split is visible right in the file. The team's `api` service references env vars (`POSTGRES_URL`, `REDIS_URL`); SRE picks the prod backend behind those vars by editing one annotation. Same compose interface, different prod target.

The parts:

- **`localmesh.yaml`** — the plugin SRE assembles to bundle the bits every project at this org gets. Installing LocalMesh = including this file. Same shape as any other plugin; plugins all the way down. Mandatory in the sense that every project includes it; *what's in it* is whatever SRE decided this org needs.
- **A LocalMesh plugin** — one file that runs a thing locally (compose half) and points at the prod equivalent (helm half, via `katenary.v3/dependencies`). Devs use it with one `include:` line.
- **The catalogue** — wherever SRE keeps the plugins. A git repo, an internal wiki, an AWS Service Catalog instance. LocalMesh doesn't prescribe.
- **[`project.toml`](./project.toml)** — one file per project capturing identity, ownership, compliance flags, billing code. Compliance, legal, and billing read the same file engineers do; flipping `handles_pii = true` pages the right people without anyone wiring up the page.
- **The compose→helm boundary** — strict on purpose. Devs write compose. SRE owns helm. The translation is machine-enforced (via [katenary](https://docs.katenary.io/)); SRE can change anything past the chart without breaking devs.

## FAQ

**So what does a team actually do day-to-day?**

Write a `docker-compose.yaml`. Include `localmesh.yaml` and whichever plugins from the catalogue the team needs. Reference the env vars those plugins wire in (`DATABASE_URL`, `REDIS_URL`, etc.). That's it — never open a platform ticket, never write helm, never reinvent observability. The compose runs locally; the chart generated from it runs in prod, same shape.

**How is this different from Backstage / Crossplane / umbrella helm charts?**

Backstage is a portal over the services you already have; LocalMesh is the *definition* of them. Crossplane provisions cloud resources from k8s, exposed as custom resources; LocalMesh defines what your compose includes, and the helm chart it renders is what eventually uses Crossplane (or doesn't — that's SRE's choice). Umbrella helm charts let you compose helm packages; LocalMesh lets you compose *compose files* and gets you the helm chart as build output.

**What doesn't LocalMesh solve?**

It doesn't pick your cloud. It doesn't host the catalogue — SRE picks the form. It doesn't replace the SRE-maintained services the plugins use (the mesh itself, the cert authority, the observability backend) — those still need to exist. And it doesn't deploy the rendered chart — you bring the release pipeline. The obvious shape is k8s + CD, but anything ending in a partially-automated, helm-readable release works: mobile apps, binary drivers, anything compose-shaped in dev with a path to prod that can be triggered from a chart. Fully manual releases don't fit.

**Do I need this if I have three services?**

Probably not. The value comes from removing the platform-ticket bottleneck, and at three services the bottleneck doesn't exist yet. The break-even is when SRE is approving the same five things every quarter — at which point packaging them as plugins pays for itself.

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
  resources (cadvisor + node-exporter) plus a roster panel listing
  every catalogued plugin's identity.
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
