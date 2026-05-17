# Mesh — production from your compose file

**Mesh is your local development environment, compiled into production.** Whatever your developers run on their laptops is the same thing that runs in prod — same plugins, same configuration, just at production scale. Fix a bug in local dev and prod has the fix on the next deploy. Improve local dev and prod gets the improvement for free. Local dev compiles into any other environment — staging, prod, anywhere else ops needs to run it — from the same definition.

**Mesh** is a framework that turns the docker-compose your teams already write into the helm chart your prod cluster already runs. Teams keep their compose interface; the chart is build output. You — SRE — maintain a small catalogue of plugins (postgres, rabbit, S3, the service mesh, observability) that every team includes with one line. Each plugin is a complete, working local-dev version that compiles to a hardened prod version, configured the same way. You don't review their charts. They don't write charts.

## What a service mesh is, briefly

A service mesh is the layer of network infrastructure that handles everything happening *between* your services — encryption, identity, load balancing, API gateways, traffic policy, observability. The boundary is loose on purpose: in practice, anything ops can stand up with terraform and place between services is fair game. Every cross-service call goes through it; you don't write code in each app to make that happen. It's installed once, at the cluster level, and it applies to every service automatically.

The mesh's headline feature is **mTLS** — *mutual* TLS, where both sides of every connection prove who they are with a short-lived certificate the mesh issues at startup. No service can talk to another without identifying itself first; no traffic crosses the wire in cleartext; no team has to learn PKI to make any of that true. The certs are already on disk; rotation is the mesh's job.

Because all of this lives in the network layer — not in each app — the compliance controls auditors actually ask about become properties of the infrastructure, not promises each team has to keep. Encryption-in-transit, service authentication, default-deny inter-service traffic: true for every service in the catalogue, all the time, whether the team that wrote the service understood the requirement or not. When an auditor asks whether internal traffic is encrypted, you point at the mesh config — once, for the whole company.

The hard part of running a service mesh across an org isn't the prod side — it's keeping it the same on every developer's laptop. Prod gets the real load balancer, the real gateway, the real identity provider; local dev gets shims or mocks, and the drift gets closed by hand, one ticket at a time, by you. That's the gap Mesh fills: the service mesh is just another catalogue plugin, with a local-dev version configured the same way as its prod target. `docker compose up` runs the same mesh prod runs. The drift you used to chase isn't there to chase.

With the mesh as a given, three things land in your lap.

(a) **A catalogue, instead of platform tickets.** A service catalogue is the small internal library of supported infrastructure you maintain — postgres, rabbit, S3, the service mesh, observability — each entry a complete local-dev version that compiles to a hardened prod target (RDS, self-hosted, whatever you've chosen). Need postgres? Include the database plugin. Each plugin ships the version *you* support — extensions, security settings, migrations, all pre-wired — configured the same way local and prod. Teams don't ask how to set it up. You decide what's in the catalogue, and you keep it as small as your bandwidth allows.

(b) **Observability is automatic; cross-service calls are normal code.** Every signal (traces, logs, metrics) lands on the team's dashboard without per-team wiring, in local dev and prod. A Python service calling another service is three lines:

```python
session.cert   = ("/run/mesh/client.crt", "/run/mesh/client.key")
session.verify = "/run/mesh/ca.crt"
session.get("https://payments/charge")
```

Teams don't run sidecars, don't pick a tracing library, don't think about cert rotation. When a team improves their local dashboards, prod gets the same improvement for free.

(c) **One file the rest of the company can read.** Every project ships a `project.toml` at the root listing the identity, compliance flags, data-residency, billing code, and ownership. Compliance, legal, billing, and ops all read the same file; flipping `handles_pii = true` pages the right people without you wiring up the page. You don't keep four spreadsheets in sync, and you don't translate engineering decisions into business language — the file does that on its own.

Mesh draws one line and forces simplicity across it. Teams write compose, include the plugins they need, and own their service from local dev to prod. You maintain the catalogue, decide what's in it, and run as many plugins as your bandwidth allows. The boundary is a contract, not a negotiation; and because every developer relies on it every day, it's the only contract that actually stays maintained.

You ship Mesh the same way every plugin ships: one line in compose, sensible defaults, ops supports the first onboarding. The catalogue grows as your bandwidth grows. Helm stops being a thing teams ever see — because the chart is build output, and you decided its shape when you wrote the plugin.

Identity / service-mesh slice: [`AUTH.md`](./AUTH.md) · Take-home scope: [`TODO.md`](./TODO.md).
