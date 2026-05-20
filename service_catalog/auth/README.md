# Consumer wiring — `auth/`

## Overview

Dev OIDC IdP. One container: **dex** (built-in password DB, three pre-seeded users — alice / bob / charlie, all sharing password `dev`).

Dex's config lives in `.localmesh/dex.yaml`, copied from `dex.yaml.sample` by `make certs` on first run. The dev edits the local copy to add/remove `staticPasswords`. `make certs` never overwrites an existing `.localmesh/dex.yaml`.

Identity (`module_name`, `owned_by`) and the service definition (`container`, `port`, `requires_auth`) live in this plugin's `plugin.toml` per [`../../docs/engineering/rules/plugin-conventions.md`](../../docs/engineering/rules/plugin-conventions.md) §1 + §3 — they are not redeclared here.

## Environment

| Name | Sample value | Source |
|------|--------------|--------|
| (none consumer-side) | — | — |

## Volumes

`None` consumer-side. The plugin mounts `.localmesh/dex.yaml` into the dex container itself.

## depends_on

`None` consumer-side.

## Labels

`None` consumer-side.

## k8s rendering

`None`. Dev-only plugin; replaced by the prod IdP integration (Okta / Keycloak / Auth0) in the prod overlay. `TODO: needs_prod_decisions IdP swap`.

## Notes

- `.localmesh/dex.yaml` is dev-owned (gitignored). Edit it to add `staticPasswords` entries — `make certs` won't clobber an existing copy. Delete and re-run `make certs` to reseed from `dex.yaml.sample`.
- Three users come pre-seeded: `alice@example.invalid`, `bob@example.invalid`, `charlie@example.invalid`. All share password `dev`. Regenerate the bcrypt hash via `docker run --rm httpd:2.4-alpine htpasswd -bnBC 10 "" dev | cut -d: -f2`.
