# Consumer wiring — `base/`

## Overview
Meta-package. Pulls in the canonical opinionated stack so an app's `project.toml` can say `plugins = ["base"]` instead of enumerating each member. Pulls in: `auth/` (OIDC IdP), `security/` (mTLS), `observability/` (OTel + metrics + dashboards), `logging/` (log shipping).

## Environment
None at this level. Each member plugin documents its own consumer-side env vars in its own README. Open those four READMEs side by side when wiring an app.

## Volumes
None.

## depends_on
None at this level.

## Labels
None at this level.

## k8s rendering
None at this level. Each member handles its own katenary labels.

## Notes
`postgres16/` is deliberately NOT in base — apps that need Postgres opt in by adding it to `project.toml`'s `plugins = [...]` alongside `base`.

To pin a different version of a member plugin (e.g. swap `postgres16` for a future `postgres17`), copy the directory and list the new slug — version-pinning is by slug today, not by any schema field.

## Example app service block
None. (Meta-package — see each member plugin's README for its own example block.)
