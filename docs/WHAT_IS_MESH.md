# LocalMesh — production from your compose file

**LocalMesh is your local development environment, compiled into production.** What developers run on their laptops compiles into staging, prod, and anywhere else ops needs to run it — same databases, same configuration, same definition. Fix a bug in local dev and prod has the fix on the next deploy.

**LocalMesh** is a framework that turns the docker-compose your teams already write into the helm chart your prod cluster already runs. Teams keep their compose interface; the chart is build output. You — SRE — maintain a catalogue of plugins (postgres, rabbit, S3, observability, security, etc) that every team includes with one line in `docker-compose.yaml`. You don't review their charts. They don't write charts.

If you've used Rails, the shape will feel familiar — LocalMesh is omakase for infrastructure, in the Rails sense: an opinionated framework that takes the long list of things every service needs (databases, queues, object stores, observability, the mesh itself) and solves them as one curated package. Each plugin wraps production-grade machinery behind a one-line `include:`. Simplicity comes from the opinions baked in, and from a clean line of ownership — teams own what's inside their compose; SRE owns everything past the helm chart.

```mermaid
# TODO: some sort of visualization. Work on this
```

## What a service mesh is, briefly

Picture a condo tower. Each service is a unit; the team owns what's inside. SRE owns the building — wiring, plumbing, the shared amenities, etc. A service mesh is the building.

Concretely, it's the layer of network infrastructure that handles everything happening *between* your services — VPCs, encryption, identity, load balancing, observability. The boundary is loose on purpose: anything ops can stand up with terraform and place between services is fair game. Every cross-service call goes through it; you don't write code in each app to make that happen. It's installed once, at the cluster level, applied to every service automatically.

Under the hood, the keycard is mTLS — every service holds one the mesh issues at startup, the way your condo card opens certain floors and not others. To devs, it's just a card.

Because everything lives in the network layer — not in each app — the compliance controls auditors ask about become properties of the building, not promises each team has to keep:

- **Isolation.** A break-in in one unit doesn't spread. Default-deny inter-service traffic; the list of who-can-call-whom is one file, not scattered across services.
- **Identity.** Every connection is signed by a known service. Anonymous calls don't get a card; anonymous calls don't get in.
- **Auditability.** Every swipe is logged, queryable from one place. "Who talked to the customer database at 3am?" is one query, not five team standups.
- **Encryption everywhere.** The cards do TLS; every service has a card.

Compliance and safety land on the same controls. You point at the mesh config once, for the whole company.

Local dev is the hardest part of running a service mesh across an org. Prod gets the real load balancer, the real gateway, the real identity provider; local dev gets shims or mocks, and keeping the two in sync is a constant drain on SRE time — drift closes by hand, one ticket at a time, with you in the middle. LocalMesh gives you the tools to make that the devs' job: the service mesh is just another catalogue plugin, with a local-dev version configured the same way as its prod target. `docker compose up` runs the same mesh prod runs. When a dev fixes a bug in local, prod is fixed on the next deploy — they've maintained your prod for you, without ever opening a ticket.

With the mesh as a given, three things follow.

(a) **A catalogue, instead of platform tickets.** The catalogue is the small internal library of infrastructure you support — postgres, rabbit, S3, the service mesh, observability. A plugin doesn't have to be a service: anything you can stuff into a helm chart counts (reserve a domain, mint an API key, provision specialized hardware). Each plugin is at minimum two parts:

- A **compose fragment** that runs it locally — ports, configs, env vars, all set the way they will be in prod. For postgres: the container, `POSTGRES_PASSWORD`, the `pg_stat_statements` extension, `postgres_exporter` scraping it.
- A **helm chart / terraform template** that renders the production version with the same presets — for postgres, AWS Aurora, a self-managed `pg_cluster`, or whatever you've picked, with the same extensions and exporter wired the same way.

Plugins often ship a **Grafana dashboard** too, if there's anything worth watching — postgres has connection counts, query latency, replication lag.

Devs write one line — `include: repo:/database/postgres.yaml` — and paste an anchor onto their service. The connection env vars (`*_DB_URL`, `*_DB_USER`, even `*_DB_CLIENT_CERT_*` if ops has gone to that trouble) wire into the container automatically. The dev builds against that config for local dev; LocalMesh renders a helm chart that connects to the prod version of the same plugin, exactly the same way. Some catalogue entries are shared amenities — the company Stripe key, an expensive third-party API, a sensitive shared database — gated to teams that have asked for them; the `include:` line is the access request. You decide what's in the catalogue.

(b) **Observability is automatic; cross-service calls are normal code.** Every signal (traces, logs, metrics) lands on the team's dashboard without per-team wiring, in local dev and prod. A Python service calling another service is three lines:

```python
session.cert   = ("/run/mesh/client.crt", "/run/mesh/client.key")
session.verify = "/run/mesh/ca.crt"
session.get("https://payments/charge")
```

Teams don't run sidecars, don't pick a tracing library, don't think about cert rotation.

(c) **One file the rest of the company can read.** Buildings have rules — not every unit gets a key to the garage, some appliances need an electrician's sign-off, nobody runs a restaurant out of their bedroom. Each project ships a `project.toml` at the root with the identity, compliance flags, data-residency, billing code, and ownership — the rules the team has signed up for. Compliance, legal, billing, and ops all read the same file; flipping `handles_pii = true` pages the right people and gates the right amenities without you wiring up the page. You don't keep four spreadsheets in sync, and you don't translate engineering decisions into business language — the file does that on its own.

LocalMesh draws one line: compose in, helm out. Everything left of that line is opinionated — teams write compose the LocalMesh way, plugins render the LocalMesh way, the translation is strict and machine-enforced. Everything right of that line is yours. How the chart deploys, which cloud it lands on, how environments are promoted, even how the catalogue itself is hosted (git links on a wiki, AWS Service Catalog, your own thing) — too varied to prescribe; LocalMesh declines to try.

What LocalMesh assumes you've already built — the mesh, the cert authority, the observability backend, the helm-receiving cluster — comes from you, automated to the hilt. The framework makes plugins that *use* those services behind a clean interface; making the services run is still SRE's job. (LocalMesh itself installs the same way every plugin does: one line in compose, sensible defaults, ops supports the first onboarding.)

The compose→helm line is strict on purpose: the needs on either side are very different, and the boundary only stays clean if the translation does. Teams get a simple interface and never see helm; SRE can change anything past the chart without breaking devs. The catalogue grows as your bandwidth grows, and because every developer relies on it every day, it stays maintained — unlike the platform-team wiki nobody reads. LocalMesh turns local dev into a box fit for prod — the wins are large, but they only land if everyone respects the size and shape of the box.

Identity / service-mesh slice: [`AUTH.md`](./AUTH.md) · Take-home scope: [`TODO.md`](./TODO.md).
