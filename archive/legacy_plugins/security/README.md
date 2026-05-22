# Consumer wiring — `security/`

## Overview
mTLS + service identity for east-west traffic. Runs kuma-cp + per-workload kuma-dp sidecars (deferred this round) + an ingress role. App-tier services get a SPIFFE identity and end-to-end encryption with no per-app config.

## Environment
None. (Workload-sidecar minting is deferred; once it lands, the consumer wiring will surface here.)

## Volumes
None.

## depends_on
None at the consumer level. Sidecar minting (deferred) will land its own `depends_on` shape.

## Labels
None at the consumer level today. `metrics.module_name: security` will flip on the security/ containers once `identityLabels` is wired into the security/ compose template (forward-looking).

## k8s rendering
None this round.

## Notes
Out-of-scope this round: workload sidecar minting from the CLI, MeshGateway/MeshGatewayRoute resources, postgres native mTLS conflict (accepted).

## Example app service block
None. (Security is infrastructure — apps don't compose-include it directly. Once workload sidecars land, this section will show the sidecar attach block.)
