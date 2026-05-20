# Consumer wiring — `auth/`

## Overview

Dev OIDC IdP. One container: **dex** (built-in password DB, three pre-seeded users — alice / bob / charlie, all sharing password `dev`). The ingress envoy (mesh plugin) is the OIDC client; its `envoy.filters.http.oauth2` filter drives the dance and `envoy.filters.http.jwt_authn` validates the bearer on every subsequent request. oauth2-proxy is retired — the filter chain replaces it.

Dex's config lives in `.localmesh/dex.yaml`, copied from `dex.yaml.sample` by `make certs` on first run. The dev edits the local copy to add/remove `staticPasswords`. `make certs` never overwrites an existing `.localmesh/dex.yaml`.

Identity (`module_name`, `owned_by`) and the service definition (`container`, `port`, `requires_auth`) live in this plugin's `plugin.toml` per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) §1 + §3 — they are not redeclared here. Dex declares `mesh.exempt: "true"` on its compose `labels:` block (it sits outside the SPIFFE mesh because it fronts it; the ingress envoy reaches it plaintext on the docker network).

## Environment

| Name | Sample value | Source |
|------|--------------|--------|
| (none consumer-side) | — | — |

Consumer apps need no env vars from this plugin. Identity is delivered via HTTP headers injected by envoy's `jwt_authn` `claim_to_headers`:

- `x-user-id`    — JWT `sub` claim
- `x-user-email` — JWT `email` claim
- `x-user-name`  — JWT `preferred_username` claim
- `authorization: Bearer <jwt>` — the raw bearer, forwarded for downstream propagation

The headers land automatically when the consumer's `[[services]]` entry leaves `requires_auth` at its default (`true`). Services that opt out (the IdP itself, eventually grafana) set `requires_auth = false`.

## Volumes

`None` consumer-side. The plugin mounts `.localmesh/dex.yaml` into the dex container itself.

## depends_on

`None` consumer-side. The ingress-mesh service depends on dex via the mesh plugin's compose.

## Labels

`None` consumer-side. Plugin-side identity + mesh-exempt labels are declared in this plugin's own compose.

## k8s rendering

`None`. Dev-only plugin; replaced by the prod IdP integration (Okta / Keycloak / Auth0) in the prod overlay. `TODO: needs_prod_decisions IdP swap`.

## Notes

- `.localmesh/dex.yaml` is dev-owned (gitignored). Edit it to add `staticPasswords` entries — `make certs` won't clobber an existing copy. Delete and re-run `make certs` to reseed from `dex.yaml.sample`.
- Three users come pre-seeded: `alice@example.invalid`, `bob@example.invalid`, `charlie@example.invalid`. All share password `dev`. Regenerate the bcrypt hash via `docker run --rm httpd:2.4-alpine htpasswd -bnBC 10 "" dev | cut -d: -f2`.
- The OIDC client secret (`LOCALMESH_OIDC_CLIENT_SECRET`) is minted by the Go CLI on first `make certs` and persisted in `.env`. The ingress envoy reads the same value via two SDS-shaped files at `/run/ingress-mesh/oauth2-{client-secret,hmac}.yaml`, written by `make oauth2-secrets`. File-based SDS sidesteps the v1.31 oauth2-filter trap that static `GenericSecret` entries fall into (`Duplicate static GenericSecret secret name`).
- Sign-out: hit `/oauth2/signout` on any gated subdomain. envoy's oauth2 filter clears the cookie and 302's home.
- Dex is mesh-exempt: no SPIFFE certs minted, no inbound mTLS. The ingress envoy talks plaintext on the docker network. Browser-facing dex traffic still rides the ingress (TLS terminated at envoy).

## Example app service block

```yaml
services:
  www:
    # Nothing extra. Auth lands on the request transparently via
    # envoy's jwt_authn claim_to_headers. App code reads:
    #   req.headers['x-user-email']        — display name / audit
    #   req.headers['x-user-name']         — preferred_username
    #   req.headers.authorization          — propagate to downstream gRPC
```
