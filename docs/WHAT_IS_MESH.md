# Mesh — production from your compose file

**Mesh is your local development environment, compiled into production.** Whatever your developers run on their laptops is the same thing that runs in prod — same plugins, same configuration, just at production scale. Fix a bug in local dev and prod has the fix on the next deploy. Improve local dev and prod gets the improvement for free. The two environments never drift apart, because there's only one of them.

**Mesh** is a framework that turns the docker-compose your teams already write into the helm chart your prod cluster already runs. Teams keep their compose interface; the chart is build output. You — SRE — maintain a small catalogue of plugins (postgres, rabbit, S3, the service mesh, observability) that every team includes with one line. Each plugin is a complete, working local-dev version that compiles to a hardened prod version, configured the same way. You don't review their charts. They don't write charts. Three things land in your lap:

(a) **Plugins, instead of platform tickets.** Need postgres? Include the database plugin. Need a broker? Include rabbit. Static-asset storage? S3. Each plugin ships the version you support — extensions, security settings, migrations, all pre-wired — in a local-dev image that compiles to a hardened prod target (RDS, self-hosted, whatever you've chosen) configured the same way. Teams don't ask how to set it up. You decide what's in the catalogue, and you keep it as small as your bandwidth allows.

(b) **mTLS, observability, identity — built in, not bolted on.** Every service in the catalogue is wrapped in mTLS by the service mesh, and every signal (traces, logs, metrics) lands on the team's dashboard without per-team wiring. Calling another service from Python looks like this:

```python
session.cert   = ("/run/mesh/client.crt", "/run/mesh/client.key")
session.verify = "/run/mesh/ca.crt"
session.get("https://payments/charge")
```

Three lines. The certs are already on disk; rotation is the mesh's job. Teams don't learn PKI, don't run sidecars, don't ask you what the CA bundle path is this week. Local dev is observed the same way prod is — when a team improves their local dashboards, prod gets the same improvement for free.

(c) **One file the rest of the company can read.** Every project ships a `project.toml` at the root listing the identity, compliance flags, data-residency, billing code, and ownership. Compliance, legal, billing, and ops all read the same file; flipping `handles_pii = true` pages the right people without you wiring up the page. You don't keep four spreadsheets in sync, and you don't translate engineering decisions into business language — the file does that on its own.

Mesh draws one line and forces simplicity across it. Teams write compose, include the plugins they need, and own their service from local dev to prod. You maintain the catalogue, decide what's in it, and run as many plugins as your bandwidth allows. The boundary is a contract, not a negotiation; and because every developer relies on it every day, it's the only contract that actually stays maintained.

You ship Mesh the same way every plugin ships: one line in compose, sensible defaults, ops supports the first onboarding. The catalogue grows as your bandwidth grows. Helm stops being a thing teams ever see — because the chart is build output, and you decided its shape when you wrote the plugin.

Identity / service-mesh slice: [`AUTH.md`](./AUTH.md) · Take-home scope: [`TODO.md`](./TODO.md).
