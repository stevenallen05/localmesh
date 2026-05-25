# Consumer wiring — `mesh/`

## Overview
Runs the LocalMesh service-mesh control plane (kuma-cp) and an ingress gateway. Every other plugin's `[[services]]` is meshed automatically — the plugin emits a `<name>-mesh` dataplane container per workload at build time. The dataplane handles mesh-mTLS, transparent proxying, and trace export to `alloy:4317`.

## Environment
| Name | Sample value | Source |
|---|---|---|
| (none) | | |

The plugin publishes no consumer-facing env vars. Mesh DNS names (`<svc>.svc.mesh.local`) are resolved by the dataplane's coredns at `:15053`.

## Volumes
None.

## depends_on
None at the consumer service level. The CLI emits the dataplane container's `depends_on` automatically; the consumer's own app container needs no edit.

## Labels
None specific to this plugin. The identity-tuple labels (`metrics.*`) the app already declares are sufficient.

## k8s rendering
Out of scope for the demo — production swaps the compose-side kuma-dp for the mesh runtime native to the target cluster (Istio / Linkerd / Cilium).

## Notes
- Every meshed service must have a valid `scheme` in its `[[services]]` entry. Supported: `http`, `https`, `grpc`, `tcp`, `postgresql`. The CLI maps each to `kuma.io/protocol`; unknown values fail the build.
- Apps must not terminate TLS themselves — the dataplane terminates inbound mesh-mTLS on the workload's declared port. Browser-visible TLS terminates at the ingress MeshGateway on `:8443`.
- Egress is policy-bound: `Mesh.networking.outbound.passthrough: false` plus a `MeshPassthrough` allowing only `*.${EXTERNAL_DOMAIN}`. Anything else is denied.
- All policy is applied by `localmesh-dataplane-bootstrap` (a one-shot container). Reruns are idempotent (`kumactl apply` is upsert-shaped).

## Example app service block
None — there's nothing to copy into a consumer service. The mesh wiring is emitted automatically by `localmesh build`.
