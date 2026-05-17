# LocalMesh — production from your compose file

**LocalMesh is your local development environment, compiled into production.** What developers run on their laptops compiles into staging, prod, and anywhere else ops needs to run it — same databases, same configuration, same definition. Fix a bug in local dev and prod has the fix on the next deploy.

**LocalMesh** is a framework that turns the docker-compose your teams already write into the helm chart your prod cluster already runs. Teams keep their compose interface; the chart is build output. You — SRE — maintain a catalogue of plugins (postgres, rabbit, S3, observability, security, etc) that every team includes with one line in `docker-compose.yaml`. You don't review their charts. They don't write charts.

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

(a) **A catalogue, instead of platform tickets.** The catalogue is the small internal library of infrastructure you support — postgres, rabbit, S3, the service mesh, observability. Each module is three pieces:

- A **compose fragment** that runs it locally — ports, configs, env vars, the metrics exporter, the extensions you support, all pre-wired. For postgres, that's the container, `POSTGRES_PASSWORD`, the `pg_stat_statements` extension, `postgres_exporter` scraping it.
- A **Grafana dashboard**, if the module has anything worth watching — postgres has connection counts, query latency, replication lag.
- A **Helm chart** that compiles to a hardened prod target with the same presets — for postgres, AWS Aurora, a self-managed `pg_cluster`, or whatever you've picked, with the same extensions and exporter wired the same way.

Teams write `include: postgres` and get the database, the dashboard, and the prod target — all configured the same way. Some catalogue entries are shared amenities — the company Stripe key, an expensive third-party API, a sensitive shared database — gated to teams that have asked for them; the `include:` line is the access request. You decide what's in the catalogue.

(b) **Observability is automatic; cross-service calls are normal code.** Every signal (traces, logs, metrics) lands on the team's dashboard without per-team wiring, in local dev and prod. A Python service calling another service is three lines:

```python
session.cert   = ("/run/mesh/client.crt", "/run/mesh/client.key")
session.verify = "/run/mesh/ca.crt"
session.get("https://payments/charge")
```

Teams don't run sidecars, don't pick a tracing library, don't think about cert rotation.

(c) **One file the rest of the company can read.** Buildings have rules — not every unit gets a key to the garage, some appliances need an electrician's sign-off, nobody runs a restaurant out of their bedroom. Each project ships a `project.toml` at the root with the identity, compliance flags, data-residency, billing code, and ownership — the rules the team has signed up for. Compliance, legal, billing, and ops all read the same file; flipping `handles_pii = true` pages the right people and gates the right amenities without you wiring up the page. You don't keep four spreadsheets in sync, and you don't translate engineering decisions into business language — the file does that on its own.

LocalMesh draws one line: compose in, helm out. Teams write compose, include the plugins they need, and own their service from local dev to prod. You maintain the catalogue, decide what's in it, and run as many plugins as your bandwidth allows. Because every developer relies on the catalogue every day, it stays maintained — unlike the platform-team wiki nobody reads.

You ship LocalMesh the same way every plugin ships: one line in compose, sensible defaults, ops supports the first onboarding. The catalogue grows as your bandwidth grows. Helm stops being a thing teams ever see — because the chart is build output, and you decided its shape when you wrote the plugin.

Identity / service-mesh slice: [`AUTH.md`](./AUTH.md) · Take-home scope: [`TODO.md`](./TODO.md).
