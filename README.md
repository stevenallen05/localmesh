# Take-home

> **LocalMesh** is a deployment pattern I designed for this take-home and am bootstrapping through a small Rust + Next.js metrics service.

The take-home asked for a metrics monitoring system. The artifact here is two things at once: the metrics service itself, and the early shape of LocalMesh — the pattern I built the service inside of. The top-level `docker-compose.yml` `include:`s a `localmesh build`–rendered bundle of plugins from a stub catalogue. The helm chart is generated from compose, not hand-maintained. Everything past this point describes LocalMesh as the pattern intends to work; the repo is the first implementation against it.

> **Status.** Phase 1 of the plugin config inversion landed; redis is the worked example. Re-promoting the rest of the catalogue under the new manifest shape is the next chunk of work. The narrative below describes the pattern, not the on-disk catalogue at this commit.

## TL;DR

**LocalMesh turns the `docker-compose.yaml` your teams already write for local dev, into a helm chart your k8s can use.** It borrows the [Rails "omakase" philosophy](https://rubyonrails.org/doctrine#omakase); opinionated curation of the boring infrastructure choices every project repeats, and applies it to the dev/prod gap. SRE assembles the catalogue; teams include one line per piece they need.

## What every development environment gets

| URL | Container | Source |
|-----|-----------|--------|
| https://www.metrics-collector.lvh.me:8443 | www | project |
| https://grafana.metrics-collector.lvh.me:8443 | grafana | observability |


## What "omakase" means here

Ruby on Rails is an opinionated web framework. It picks the ORM, the routing layer, the templating engine, and the dozen libraries every web app needs. Teams don't repeat those choices on every project. The trade-off is flexibility for not having to decide. Pre-picked defaults applied uniformly beat bespoke decisions each team makes in parallel.

LocalMesh applies the same idea to infrastructure. Most projects make the same boring choices: which postgres, which queue, what observability looks like, where secrets live, how identity propagates. SRE curates them once and packages them as a catalogue of plugins. Teams pull them in with one line in `docker-compose.yaml`. Devs accept the defaults. SRE owns the prod posture. Both sides save time on solved problems.

## What a service mesh is, briefly

A condo tower is a useful analogy. Each service is a unit; the team owns what's inside. SRE owns the building — wiring, plumbing, shared amenities. A service mesh is the building.

Concretely, a service mesh is the layer of network infrastructure between services. It handles encryption, identity, routing, and observability. The boundary is loose on purpose. Anything ops can install for the whole building counts. Every cross-service call goes through it. Teams don't write code in each app to make it happen — it's wired in once and applied to every service automatically. Mechanically: mTLS. Every service gets a certificate at startup, signed by the mesh's CA. Apps don't manage it; they just have it.

**LocalMesh applies the same pattern to docker-compose.** A service mesh imposes opinionated curation on the network layer. LocalMesh imposes it on the compose layer. What teams run locally is the same shape as what prod installs. Long form: [`docs/WHAT_IS_MESH.md`](./docs/WHAT_IS_MESH.md).

## The dev/prod gap

In most companies, prod and local dev are different setups with different behavior. Prod has the real load balancer, identity provider, managed database, and secrets backend. Local dev has shims, mocks, or "works on my machine." Drift accumulates one component at a time. SRE spends their week closing it ticket by ticket. A developer hits something prod does that compose doesn't, files a platform ticket, waits.

LocalMesh closes the gap by treating local dev as a compile target of prod. Both run the same plugins with the same defaults. A bug fix in local lands in prod on the next deploy. Devs maintain prod for SRE without filing tickets.

## The pattern: Resource Definitions

LocalMesh implements the **Resource Definition pattern** (also called the Platform Orchestrator pattern). The same shape shows up under different names: Score and Humanitec call them Resource Definitions, Kratix calls them Promises, Radius calls them Recipes, Crossplane calls them Compositions. The pattern separates *what* an app needs from *how* the platform provides it:

- **Workload specification** — declared by the app team. Says "I need a postgres" without saying which postgres or how to provision it.
- **Resource Definition** — authored by SRE. The recipe: hardening, sizing, backups, IAM, log retention, all the boring details, encoded once.
- **Platform Orchestrator** — matches a workload's declared needs against the available Resource Definitions and runs the provisioning.

LocalMesh's terms map onto this directly:


| Pattern concept        | LocalMesh                                                                                                                   |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| Workload specification | A team's`docker-compose.yaml`                                                                                               |
| Resource Definition    | A LocalMesh plugin                                                                                                          |
| Service catalog        | The catalogue                                                                                                               |
| Platform Orchestrator  | Katenary plus the downstream provisioner (Crossplane, a Terraform Operator, ACK, whatever the rendered helm chart triggers) |

LocalMesh's twist on the pattern: each Resource Definition has **two render targets**, not one. The compose half of a plugin is the local-dev render; the helm dependency is the prod render. Same definition, same defaults, two outputs. That dual render is what closes the dev/prod gap — devs and SRE work against the same artifact.

## The parts of LocalMesh

Before the parts: here's what it looks like in a team's repo.

```yaml
# A team's docker-compose.yaml — the workload specification
include:
  - repo:/localmesh.yaml          # the org's baseline — "LocalMesh installed"
  - repo:/database/postgres.yaml  # one piece from the catalogue

services:
  api:
    environment:
      DATABASE_URL: ${POSTGRES_URL}   # wired in by the postgres plugin
```

```yaml
# service_catalog/database/postgres.yaml — what SRE puts in the catalogue
services:
  postgres:
    image: postgres:16
    labels:
      # Compose service above = the local-dev render.
      # katenary.v3/dependencies = the prod render, owned by SRE.
      katenary.v3/dependencies: |-
        - name: postgresql
          repository: oci://registry-1.docker.io/bitnamicharts
      # Swap the chart for CNPG, RDS-via-Crossplane, etc. without
      # touching the team's compose file.
```

The team references `${POSTGRES_URL}`; SRE picks the prod backend behind it by editing one annotation. Same compose interface, different prod target. The org's baseline (`service_catalog/localmesh.yaml`) is the same shape — one plugin that `include:`s the mesh, observability, and compliance plugins every project at this org gets.

The parts:

- **`localmesh.yaml`** — the org's baseline Resource Definition. Bundles every plugin every project at this org gets. Installing LocalMesh = including this file. Same shape as any other plugin; it just happens to include other plugins itself. Mandatory in the sense that every project includes it; the contents are whatever SRE decided this org needs.
- **A LocalMesh plugin** — a Resource Definition in LocalMesh's format. One file that defines two render targets for the same logical dependency: the local-dev shape (the compose half) and the prod shape (the helm dependency, declared via `katenary.v3/dependencies`). Devs use it with one `include:` line.
- **The catalogue** — LocalMesh's service catalog. Wherever SRE keeps the plugins. A git repo, an internal wiki, an AWS Service Catalog instance, an OCI registry. LocalMesh doesn't prescribe.
- **[`project.toml`](./project.toml)** — workload metadata. One file per project capturing identity, ownership, compliance flags, billing code. Compliance, legal, and billing read the same file engineers do; flipping `handles_pii = true` pages the right people without anyone wiring up the page.
- **The compose→helm boundary** — the Platform Orchestrator seam, machine-enforced. Devs author the workload spec (compose). SRE owns the prod render (helm). Katenary does the translation. SRE can change anything past the chart without breaking devs.

## FAQ

**So what does a team actually do day-to-day?**

Write a `docker-compose.yaml`. Include `localmesh.yaml` and whichever plugins from the catalogue the team needs. Reference the env vars those plugins wire in (`DATABASE_URL`, `REDIS_URL`, etc.). That's it. No platform tickets, no hand-written helm. The compose runs locally; the generated chart deploys to prod with the same configuration.

**How is this different from Backstage / Crossplane / umbrella helm charts / Score?**

Backstage is a portal over the services you already have; LocalMesh is what defines them. Crossplane is one possible target for the Platform Orchestrator step — a plugin's helm dependency can render a Crossplane Claim, but it can equally render an ACK CR, a Terraform Operator CR, or an in-cluster chart; SRE decides per plugin. Umbrella helm charts let you compose helm packages; LocalMesh composes *compose files* and emits the helm chart as build output, so the dev never writes helm. Score is the closest peer — same Resource Definition pattern, different workload-spec format. LocalMesh uses compose because compose is already the local-dev artifact; Score uses its own YAML and leaves local dev to a separate tool.

**What doesn't LocalMesh solve?**

It doesn't pick your cloud. It doesn't host the catalogue — SRE picks the form. It doesn't replace the SRE-maintained services the plugins use: the mesh itself, the cert authority, the observability backend. Those still need to exist. And it doesn't deploy the rendered chart — you bring the release pipeline. The obvious shape is k8s + CD. Anything ending in a partially-automated, helm-readable release also works: mobile apps, binary drivers, anything compose-shaped in dev with a release path that can be triggered from a chart. Fully manual releases don't fit.

**Do I need this if I have three services?**

Probably not. The value comes from removing the platform-ticket bottleneck. At three services, that bottleneck doesn't exist yet. The break-even is when SRE is approving the same five things every quarter. At that point, packaging them as plugins pays for itself.

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

What runs end-to-end at this commit is the **Phase 1 redis worked example**. The fuller stack (Grafana / Tempo / Loki / www / postgres + dashboards) is offline pending its plugins' re-promotion under the inverted-config schema — see the Status block above.

Prereq: Go 1.24 (for the `localmesh` CLI) + Docker.

```bash
go run ./localmesh_src/cmd/localmesh build
docker compose up -d redis
```

`localmesh build` rewrites `.env`'s managed section + regenerates `localmesh/bundled.compose.yaml` + writes `localmesh/developer_reference.md`. The top-level `docker-compose.yml` `include:`s the bundle and reads the resolved env vars (e.g., `${CACHE_URL}`).

`make setup` is currently a no-op-with-error: its dexseed step reads `localmesh/service_catalog/auth/dex.yaml.sample`, which moved to `archive/legacy_plugins/`. Bringing it back is tracked in `docs/CLEANUP.md` §7. Use `localmesh build` directly until then.

## What to look at

After `docker compose up -d redis`, dial it from a one-off client on the project network:

```bash
docker run --rm --network ${PROJECT_NAME:-metrics-collector}_default \
  redis:7-alpine redis-cli -u "$(grep '^CACHE_URL=' .env | cut -d= -f2)" ping
# PONG
```

The consumer dialed an env var whose value the plugin author owns and whose destination name the project author chose. The fuller stack (Grafana, APM dashboard, www demo buttons, Tempo, Loki) returns as each plugin re-promotes under the new manifest shape.
