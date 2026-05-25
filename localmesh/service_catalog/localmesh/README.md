# Consumer wiring — `localmesh/`

## Overview
Meta-package. Declares the LocalMesh baseline plugin set. Today it bundles `observability/`. `mesh/` is its own selectable plugin (named directly in `project.toml`'s `plugins = [...]`); it may fold into this baseline once its shape stabilises. `auth/` is archived in `archive/legacy_plugins/`.

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
Consumers don't wire to `localmesh/` directly. They list it in `project.toml`'s `plugins = [...]` and get the bundled set. For consumer-facing observability notes, see [`../observability/README.md`](../observability/README.md).

## Example app service block
None.
