# Consumer wiring — `localmesh/`

## Overview
Meta-package. Pulls in the LocalMesh baseline (today: `observability`).

## Environment
None. See [`../observability/README.md`](../observability/README.md) for the OTel env vars `observability/` injects on consumer services.

## Volumes
None.

## depends_on
None.

## Labels
None.

## k8s rendering
None.

## Notes
Consumers don't wire to `localmesh/` directly — they list it in `project.toml`'s `plugins = [...]` and get the bundled set. For consumer-facing observability notes, see [`../observability/README.md`](../observability/README.md).

## Example app service block
None.
