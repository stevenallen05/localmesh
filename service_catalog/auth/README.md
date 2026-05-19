# Consumer wiring — `auth/`

## Overview

Required ingress auth gate. Two containers: **oauth2-proxy** (auth-decision sidecar that Caddy hits via `forward_auth`) and **dex** (OIDC IdP using its built-in local connector with one `staticPasswords:` entry per dev user from `.secrets/users.yaml`, all sharing the password `dev`). Together they replace the prior `auth_shim/` plugin and remove all auth responsibility from www.

Identity (`module_name`, `owned_by`) and the service definitions (`container`, `port`, `requires_auth`) live in this plugin's `plugin.toml` per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) §1 + §3 — they are not redeclared here. Both services declare `mesh.exempt: "true"` on their compose `labels:` blocks (the source of truth, parsed by `secrets-gen.py` — they sit outside the SPIFFE mesh because they front it).

## Environment

| Name             | Sample value                                                            | Source |
|------------------|-------------------------------------------------------------------------|--------|
| (none consumer-side) | — | — |

Consumer apps need no env vars from this plugin. Identity is delivered via HTTP headers (`X-Forwarded-User`, `X-Forwarded-Email`, `X-Forwarded-Preferred-Username`, `Authorization: <jwt>`) injected by Caddy's `forward_auth` middleware, configured automatically when `requires_auth = true` (the default) on the consumer's `[[services]]` entry.

## Volumes

`None` consumer-side. The plugin manages its own config volumes.

## depends_on

`None` consumer-side. Caddy depends on oauth2-proxy and dex via its compose `depends_on` block. App-tier services do not.

## Labels

`None` consumer-side. Plugin-side identity + mesh-exempt labels are declared in this plugin's own compose.

## k8s rendering

`None`. Dev-only plugin; replaced by the prod IdP integration (`oauth2-proxy` + real IdP like Okta / Keycloak / Auth0) in the prod overlay. `TODO: needs_prod_decisions IdP swap`.

## Notes

- `.secrets/users.yaml` is the source of truth for the dev-user list. Edit + `make certs && docker compose up -d dex` to pick up changes (the file gets re-rendered into `dex.yaml.generated`).
- Both containers are mesh-exempt: no SPIFFE certs minted, no inbound mTLS. Caddy reaches them via plaintext on the docker network.
- Sign-out: hit `/oauth2/sign_out?rd=/` on any gated subdomain. oauth2-proxy clears the cookie and 302's to `rd`.
- The OIDC client secret (`LOCALMESH_OIDC_CLIENT_SECRET`) and cookie session secret (`OAUTH2_PROXY_COOKIE_SECRET`) are minted by `secrets-gen.py` on first `make certs`, persisted to `.env`'s managed section. They survive subsequent runs.

## Example app service block

```yaml
services:
  www:
    # Nothing extra. Auth is delivered to www transparently by Caddy's
    # forward_auth middleware. App code reads:
    #   req.headers['x-forwarded-email']    — display name
    #   req.headers.authorization           — proxy opaquely to gRPC server
```
