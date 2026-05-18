# Consumer wiring — `auth_shim/`

## Overview

Dev-only user picker source. Exposes `GET /users` on `http://auth-shim:8080/users` returning the configured dev-user list from `.secrets/users.yaml`. www's `/auth/login` page fetches this server-side and renders a radio-button picker; selection sets `lm.user.{id,email,name}` cookies that subsequent API-route requests pick up via `getUser(req)` in `www/lib/identity.ts`.

Mesh-exempt (no certs minted). Reached over plaintext on the docker network. Replaced in prod by oauth2-proxy + a real OIDC IdP (Dex, Keycloak, etc.). `TODO: needs_prod_decisions oauth2-proxy + IdP at ingress`.

Identity (`module_name`, `owned_by`, `mesh_exempt`) and the `auth-shim` service definition (`container`, `port`) live in this plugin's `plugin.toml` per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) §1 + §3 — they are not redeclared here.

## Environment

| Name             | Sample value               | Source |
|------------------|----------------------------|--------|
| `DEV_USER_ID`    | `dev-user`                 | app    |
| `DEV_USER_EMAIL` | `dev-user@example.invalid` | app    |
| `DEV_USER_NAME`  | `Dev User`                 | app    |

Fallback identity for when `auth_shim` isn't included or the user hasn't picked yet. `www/lib/identity.ts` reads these at the bottom of its resolution order (cookie > X-Forwarded headers > env).

## Volumes

`None`. Cert dir not mounted (mesh-exempt).

## depends_on

`None` for consumers. www calls `auth-shim` opportunistically in `/auth/login`'s `getServerSideProps` and falls back to env-based identity when the call fails — the plugin can be included or not without breaking www.

## Labels

`None` consumer-side. Plugin-side identity labels per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) §1 are declared in this plugin's own compose; consumer services do not add anything for `auth_shim`.

## k8s rendering

`None`. Dev-only plugin; not part of the prod rendering target. Removed when the prod IdP module replaces it.

## Notes

- `.secrets/users.yaml` is the source of truth for the dev-user list. Edit + `docker compose restart auth-shim` to pick up changes.
- The plugin uses a hand-rolled tiny YAML parser (`yaml-tiny.mjs`) to avoid an `npm install` step in the container. Supports only the inline-object list shape used by `users.yaml`. Swap in `js-yaml` if the schema grows.
- The cookies `lm.user.{id,email,name}` are plaintext and tamperable — the dev "auth provider" doesn't sign or validate them. Per the spec §9.3, any anti-spoofing enforcement is deferred to a real IdP.

## Example app service block

```yaml
services:
  www:
    environment:
      DEV_USER_ID:    "dev-user"
      DEV_USER_EMAIL: "dev-user@example.invalid"
      DEV_USER_NAME:  "Dev User"
```
