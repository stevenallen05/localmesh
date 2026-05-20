# Consumer wiring — `mesh/`

## Overview

One envoy fleet across three roles, in a single plugin. The
`ingress-mesh` role terminates browser HTTPS on `:8443`,
runs the OIDC dance against dex via `envoy.filters.http.oauth2`,
validates the issued JWT with `envoy.filters.http.jwt_authn`, and
stamps claims onto `x-user-*` headers. Per-workload `<container>-mesh`
sidecars share their workload's netns via `network_mode:
"service:<container>"` and terminate east-west mTLS on the workload's
declared port. The `egress-mesh` role gates internet-bound traffic from
sidecars (uniform allowlist v0 in dev; declared allowlist via
`plugin.toml [[externals]]` aspirational).

Foundation-tier omakase rows from the envoy-omakase ruleset land at
boot: row 1 (OTel spans on every hop via `tracing.http`), row 2
(`:15090/stats/prometheus` per envoy), row 4 (`x-request-id`
propagation), row 5 (XFCC peer identity), row 6 (JSON access logs with
trace_id + peer SPIFFE URI + jwt_kid), row 7 (TLS 1.3 east-west only).
Deferred to `TODO: needs_prod_decisions`: row 3 (outlier detection),
rows 8–9 (retry / timeout schemas), row 10 (scope linter), row 11
(header stripping on egress), row 12 (claim-aware downstream), row 14
(cert-as-policy), row 15 (claim-filter at workload).

## Environment

| Name | Sample value | Source |
|------|--------------|--------|
| (none consumer-side) | — | — |

Consumer apps need no env vars from this plugin. The sidecar shares the
app's netns; the app dials peer container names plaintext on loopback
(e.g. `SERVER_ADDR=server:50051`). Docker DNS resolves the name;
iptables in the netns REDIRECTs the SYN to envoy; envoy reads
`SO_ORIGINAL_DST` and routes to the right `<target>-mesh` cluster over
mTLS.

`TODO: needs_prod_decisions MESH_*_{HOST,PORT,URL} auto-emission` — a
future CLI pass emits per-consumer connection-string env vars from each
workload's resolved `depends_on` so apps stop hand-authoring them.

## Volumes

`None` consumer-side. The mesh plugin's compose template emits the per-
role mounts itself. For reference, each envoy container receives:

| Mount | Source | Purpose |
|-------|--------|---------|
| `/run/<container>/{id.crt,id.key,trust.ca.crt}` | docker compose `secrets:` from `.localmesh/secrets/<container>/` | mTLS bundle (CA + leaf cert + key) |
| `/etc/envoy/envoy.yaml` | bind-mount `.localmesh/envoy/<container>-mesh.yaml` | rendered envoy bootstrap (gitignored output) |
| `/run/ingress-mesh/oauth2-{client-secret,hmac}.yaml` | docker compose `secrets:` (ingress only) | SDS-shaped GenericSecret docs the `oauth2` filter reads via `path_config_source` |
| `/init.sh` | bind-mount `.localmesh/envoy/<container>-mesh-init.sh` (init container only) | iptables REDIRECT script |

Certs travel as docker compose `secrets:` rather than bind mounts —
the secrets driver lands files at the declared target with a fresh
mode regardless of host-side ownership, sidestepping the uid mismatch
between envoy's drop-priv uid 101 and the host's uid 1000.

## depends_on

`None` consumer-side. Consumers don't depend on mesh explicitly —
their `<container>-mesh-init` and `<container>-mesh` services depend on
the consumer (`condition: service_started`), share its netns, and
intercept traffic transparently. The mesh plugin's template emits the
dependency edges per-workload from the catalog.

The `ingress-mesh` role depends on dex (`condition: service_healthy`)
so the OIDC dance has its IdP up at boot.

## Labels

`None` consumer-side. Plugin-side labels per envoy container (emitted
by the mesh template):

```yaml
labels:
  prometheus.io/scrape: "true"
  prometheus.io/port:   "15090"
  prometheus.io/path:   "/stats/prometheus"
  metrics.service_name: envoy
  metrics.module_name:  mesh
  metrics.owned_by:     sre@example.com
  mesh.role:            ingress | egress | sidecar
```

`mesh.role` lets dashboards filter the envoy fleet by topology axis.
`prometheus.io/*` drives observability's `docker_sd_configs` discovery.

Consumer workloads opt **out** of the mesh by declaring `mesh.exempt:
"true"` on their own service's compose `labels:` block. Today's
exempt set: dex (IdP fronts itself), postgres (STARTTLS not
transparent-proxyable), observability/*, logging/*.

## k8s rendering

`None`. The dev envoy fleet swaps for the prod mesh runtime (Istio /
Linkerd / Cilium eBPF) at katenary chart-render time via plugin
replacement. The compose-side same-pod construct
(`network_mode: "service:..."`) has no clean katenary expression today
— `TODO: needs_prod_decisions katenary same-pod label for sidecars`.

The iptables-init container + `cap_add: ["NET_ADMIN"]` pattern matches
Istio/Linkerd's compose-side shape; prod runtimes use eBPF (Cilium-
style) or per-pod init. `TODO: needs_prod_decisions iptables init
container vs eBPF (Cilium-style) for prod`.

## Notes

- **Cert delivery via docker secrets.** envoy 1.31's drop-priv uid 101
  can't read a host bind-mounted file owned by host uid 1000. The
  compose `secrets:` driver writes files inside the container at the
  declared target path with a fresh mode, sidestepping the mismatch.
  `TODO: needs_prod_decisions cert delivery via SPIRE Workload API or
  cert-manager Secret mount`.
- **oauth2 filter SDS.** Envoy v1.31's `static_resources.secrets` trips
  on `Duplicate static GenericSecret secret name` when the oauth2
  filter and other consumers reference the same name. File-based SDS
  via `path_config_source` sidesteps the trap. `make oauth2-secrets`
  writes the two SDS docs from `.env`'s `LOCALMESH_OIDC_CLIENT_SECRET`.
- **JWT claim flow.** The ingress envoy validates the JWT once via
  `jwt_authn` against dex JWKS. `claim_to_headers` injects `x-user-id`
  (sub), `x-user-email`, `x-user-name` (preferred_username) onto the
  upstream request. The OTel ingress span gets `enduser.{id, email,
  preferred_username}` via OTTL on otel-collector reading those same
  headers — the one site where PII lands on telemetry. Downstream
  sidecars don't re-validate the JWT; trust comes from the mTLS channel
  (peer SPIFFE URI on XFCC). Loki lines carry `jwt_kid` for forensic
  pivoting without indexing user identity. Aligns with the PII-at-
  ingress contract from `DESIGN_DECISIONS.md`.
- **Unauthenticated ingress virtual hosts.** Each service's
  `[[services]]` entry can set `requires_auth = false`. The ingress
  template skips `jwt_authn` for those virtual hosts. Today's unauth
  set: `dex.*` (the IdP can't gate itself), `envoy.*` (admin DX), and
  `grafana.*` (dashboards browsed openly in dev).
- **Egress catch-all.** Each workload-sidecar's outbound listener
  forwards unmatched destinations to `egress-mesh:15001` over mTLS.
  Today's egress allowlist is uniform (allow-all in dev). `TODO:
  needs_prod_decisions declared egress allowlist via plugin.toml
  [[externals]]` (row 13).
- **Per-callee outlier detection.** A single default applies uniformly
  until the CLI joins caller↔callee at template time so the callee's
  labels flow into the caller's outlier config. `TODO:
  needs_prod_decisions per-callee outlier detection needs CLI caller-
  callee join` (row 3).
- **Cert-as-policy.** Per-workload policy is shipped as a JSON file
  next to the mTLS cert (`policy/schema.example.json`). Envoy doesn't
  parse the file directly; an init step (today a placeholder) turns it
  into RBAC config the listener consumes. Prod migration moves the
  same payload into x509 custom OIDs without changing the RBAC
  contract downstream. `TODO: needs_prod_decisions per-caller RBAC via
  cert-as-policy (row 14)`.
- **Open admin endpoint.** `:15000` is bound openly on each envoy in
  dev for DX (browse the admin UI at
  `https://envoy.${PROJECT_NAME}.${LOCAL_DOMAIN}:8443/`). `TODO:
  needs_prod_decisions envoy admin locked down in prod`.
- **What you get for free.** The "always-on but doesn't merit a
  dashboard row" omakase set lands per envoy: HTTP/2 + h2c on
  loopback (for gRPC and trace propagation), TLS 1.3 only east-west
  (no protocol downgrade), connection limits per upstream cluster
  (default 1024), idle stream RST after 60s, server identification
  headers (`server`, `x-powered-by`) stripped.

## Example app service block

```yaml
services:
  www:
    image: myteam/www:1.4.0
    environment:
      # App dials the peer service name on its declared port; the
      # sidecar intercepts transparently via iptables + ORIGINAL_DST.
      SERVER_ADDR: server:50051
      # Auth lands on the request via x-user-* headers stamped by the
      # ingress envoy. App code reads:
      #   req.headers['x-user-email']
      #   req.headers['x-user-name']
      #   req.headers.authorization (raw JWT, forwarded to gRPC)
    labels:
      metrics.service_name: www
      metrics.module_name:  app
      metrics.owned_by:     ${TECH_LEAD_EMAIL}
      # mesh.exempt: "true"   # uncomment to opt out of the mesh

  # The www-mesh-init + www-mesh sidecar pair is emitted by the mesh
  # plugin's template, not by the consumer. Shown here for reference:
  #
  # www-mesh-init:
  #   image: alpine:3.20
  #   network_mode: "service:www"
  #   cap_add: ["NET_ADMIN"]
  #   command: ["sh", "-c", "apk add --no-cache iptables && /init.sh"]
  #   restart: "no"
  #
  # www-mesh:
  #   image: envoyproxy/envoy:v1.31-latest
  #   network_mode: "service:www"
  #   depends_on:
  #     www-mesh-init: { condition: service_completed_successfully }
  #     www:           { condition: service_started }
```
