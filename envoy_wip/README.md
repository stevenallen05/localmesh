# Consumer wiring — `mesh/` (envoy-omakase WIP)

> **Status: scaffold.** Templates are minimally-plausible until the
> bundling CLI lands the catalog-pass that feeds context. Today the
> templates document their CLI contract via header comments; nothing
> renders end-to-end yet. The existing ghostunnel + caddy + oauth2-proxy
> stack stays the runtime answer until this swap is ready.

## Overview

One Envoy fleet across three roles replaces caddy (north-south ingress
+ OIDC), ghostunnel (east-west mTLS sidecars), and oauth2-proxy (login
flow). Per-workload sidecars share their workload's netns and terminate
inbound mTLS on the declared east-west port. The ingress role
terminates HTTPS on `:8443`, runs the OIDC dance against Dex, validates
the issued JWT, and stamps user claims onto downstream headers. The
egress role gates internet traffic from workload sidecars by reading a
per-caller policy file delivered alongside the workload's mTLS cert.

Catalog of features baked into every role's bootstrap lives in
`templates/_helpers.tmpl`. Per-row coverage maps to the impact-ranked
omakase list. Rows in scope: 1, 2, 4, 5, 6, 7, 13, 14. Deferred:
3 (`needs_prod`), 8, 9, 10, 11, 12, 15.

## Roles

| Role | Container | Replaces | Listens on |
|------|-----------|----------|------------|
| Ingress | `ingress-mesh` | `caddy` + `oauth2-proxy` | `0.0.0.0:8443` (HTTPS) |
| Egress | `egress-mesh` | none today (new) | `0.0.0.0:15001` (mTLS) |
| Sidecar | `<workload>-mesh` | `<workload>-inbound` + `<workload>-outbound` ghostunnel pair | per-workload east-west port (mTLS) + `127.0.0.1:15001` (outbound catch-all) |

The sidecar role is per-workload. The CLI mints one render per
workload from `templates/workload-sidecar.envoy.yaml.gotmpl`.

## Environment

| Name | Sample value | Source |
|------|--------------|--------|
| `OTEL_SERVICE_NAME` | `ingress-mesh` / `egress-mesh` / `<workload>-mesh` | plugin |
| `OTEL_RESOURCE_ATTRIBUTES` | `service.namespace=${PROJECT_NAME},deployment.environment.name=dev,module_name=mesh,owned_by=sre@example.com` | plugin |

Consumer apps need no envoy env vars. The sidecar shares the app's
netns; the app dials plaintext on loopback as before.

## Volumes

The CLI mounts three things into each envoy container:

| Mount | Source | Purpose |
|-------|--------|---------|
| `/run/<role>` | `./.secrets/certs/<role>` | step-ca cert bundle (`id.crt`, `id.key`, `trust.ca.crt`) |
| `/etc/envoy/envoy.yaml` | `./envoy_wip/rendered/<role>.yaml` | rendered envoy bootstrap (gitignored output) |
| `/var/mesh/policy/id.policy.json` | `./.secrets/policy/<workload>.policy.json` | per-caller policy file (sidecars + egress only) |

## depends_on

Workload sidecars depend on their workload:

```yaml
depends_on:
  <workload>:
    condition: service_started
```

The CLI emits this per-workload. The ingress role depends on `dex`
when the auth/ plugin is included.

## Labels

`metrics.*` per [`plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md)
plus the new `mesh.envoy.*` namespace:

| Label | Values | Purpose |
|-------|--------|---------|
| `mesh.role` | `ingress` / `egress` / `sidecar` | identifies the envoy role in dashboards |
| `mesh.envoy.admin_port` | `15000` | open in dev (prod's call); locked-down version is SRE-owned |
| `mesh.envoy.stats_port` | `15090` | observability/ prometheus job discovers via this label |

Per-callee outlier detection labels were on the original omakase list
(row 3). They moved to `needs_prod` because tuning needs the CLI to
join caller↔callee at template time. The default applies uniformly
in the meantime.

## k8s rendering

`katenary` rendering is not pinned. Per-workload sidecars need
same-pod placement (the netns share has no clean katenary expression).
See `docs/TODO.md`: `katenary same-pod label for sidecars`.

## Notes

- **JWT claim flow.** The ingress envoy validates JWT once. `x-user-*`
  headers land at downstream sidecars via header_mutation; downstream
  sidecars strip externally-sourced `x-user-*` on outbound so the
  ingress is the only entry point. `user.id` lands on the OTel trace
  as an attribute, not into log fields. Loki lines carry `jwt_kid`
  for forensic pivoting without indexing user identity. Aligns with
  the PII-at-ingress contract.
- **Cert-as-policy.** Per-workload policy is delivered as a JSON file
  next to the mTLS cert (`id.policy.json`). Envoy doesn't parse the
  file; an init step (today a placeholder) turns it into the RBAC
  config the listener consumes. Prod migration moves the same payload
  into x509 custom OIDs without changing the RBAC contract downstream.
  See `docs/TODO.md` Aspirational: App-tier cert as the access-policy
  carrier.
- **Open admin endpoint.** `:15000` is bound on `0.0.0.0` in dev. Prod
  deployment is SRE-owned and locks this down. Documented as
  always-on omakase, not a per-workload knob.
- **Workload-sidecar transparent egress is not pinned.** Iptables
  redirection isn't a clean compose construct. Interim is named
  upstreams that bypass the catch-all. See `notes.md` for the
  enumerated gaps.
