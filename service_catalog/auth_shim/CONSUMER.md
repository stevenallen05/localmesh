# Consumer wiring — `auth_shim/`

## Overview

Dev-only user picker source. Exposes `GET /users` on `http://auth-shim:8080/users`
returning the configured dev-user list from `.secrets/users.yaml`. www's
`/auth/login` page fetches this server-side and renders a radio-button picker;
selection sets `lm.user.{id,email,name}` cookies that subsequent API-route
requests pick up via `getUser(req)` in `www/lib/identity.ts`.

Mesh-exempt (no certs minted). Reached over plaintext on the docker network.
Replaced in prod by oauth2-proxy + a real OIDC IdP (Dex, Keycloak, etc.).
`TODO: needs_prod_decisions oauth2-proxy + IdP at ingress`.

## Environment

| Name                   | Sample value          | Source  |
|------------------------|-----------------------|---------|
| `AUTH_SHIM_MODULE_NAME`| `auth_shim`           | plugin  |
| `AUTH_SHIM_OWNED_BY`   | `sre@example.com`     | plugin  |
| `AUTH_SHIM_PORT`       | `8080`                | plugin  |

Values flow into `.env` via `make certs` from this plugin's `plugin.toml`.

## Volumes

`None`. Cert dir not mounted (mesh-exempt).

## depends_on

`None` for consumers. www calls `auth-shim` opportunistically in
`/auth/login`'s `getServerSideProps` and falls back to env-based identity
when the call fails — so the plugin can be included or not without
breaking www.

## Labels

`None` consumer-side. The plugin itself carries:

```yaml
labels:
  mesh.exempt: "true"
  metrics.service_name: auth-shim
  metrics.module_name: ${AUTH_SHIM_MODULE_NAME}
  metrics.owned_by: ${AUTH_SHIM_OWNED_BY}
```

## k8s rendering

`None`. Dev-only plugin; not part of the prod rendering target. Removed
when the prod IdP module replaces it.

## Notes

- `.secrets/users.yaml` is the source of truth for the dev-user list.
  Edit + `docker compose restart auth-shim` to pick up changes.
- The plugin uses a hand-rolled tiny YAML parser (`yaml-tiny.mjs`) to
  avoid an `npm install` step in the container. Supports only the
  inline-object list shape used by `users.yaml`. Swap in `js-yaml` if
  the schema grows.
- The cookies `lm.user.{id,email,name}` are plaintext and tamperable —
  the dev "auth provider" doesn't sign or validate them. Per the spec
  §9.3, any anti-spoofing enforcement is deferred to a real IdP.

## Example app service block

```yaml
services:
  www:
    environment:
      # Fallback identity when auth_shim isn't included or the user
      # hasn't picked yet. Same values lib/identity.ts reads at the
      # bottom of its resolution order.
      DEV_USER_ID:    "dev-user"
      DEV_USER_EMAIL: "dev-user@example.invalid"
      DEV_USER_NAME:  "Dev User"
```
