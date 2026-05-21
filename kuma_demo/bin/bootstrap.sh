#!/usr/bin/env bash
set -euo pipefail

# One-shot. Waits for kuma-cp, applies baseline policies, mints
# per-workload Dataplane tokens. Runs in a debian:bookworm-slim container
# with tools/kumactl mounted in; installs curl on the fly.

CP_HTTP=http://kuma-cp:5681
TOKEN_DIR=/shared
KUMACTL=/usr/local/bin/kumactl

log() { printf '[bootstrap] %s\n' "$*" >&2; }

log "installing curl"
apt-get update >/dev/null
apt-get install --yes --no-install-recommends curl ca-certificates >/dev/null

log "waiting for ${CP_HTTP}/ (200 expected)"
until curl --silent --fail "${CP_HTTP}/" >/dev/null; do sleep 1; done

log "configuring kumactl"
"${KUMACTL}" config control-planes add \
  --address "${CP_HTTP}" --name default --overwrite

log "applying Mesh (default, Exclusive)"
"${KUMACTL}" apply -f - <<'EOF'
type: Mesh
name: default
meshServices:
  mode: Exclusive
EOF

log "applying MeshIdentity (Bundled, autogenerate)"
"${KUMACTL}" apply -f - <<'EOF'
type: MeshIdentity
mesh: default
name: default-identity
spec:
  selector:
    dataplane:
      matchLabels: {}
  spiffeID:
    trustDomain: "{{ .Mesh }}.mesh.local"
    path: "/workload/{{ .Workload }}"
  provider:
    type: Bundled
    bundled:
      meshTrustCreation: Enabled
      insecureAllowSelfSigned: true
      autogenerate:
        enabled: true
      certificateParameters:
        expiry: 24h
EOF

log "applying MeshTrafficPermission (demo-app -> kv)"
"${KUMACTL}" apply -f - <<'EOF'
type: MeshTrafficPermission
name: allow-kv-from-demo-app
mesh: default
spec:
  targetRef:
    kind: Dataplane
    labels:
      app: kv
  rules:
    - default:
        allow:
          - spiffeID:
              type: Exact
              value: "spiffe://default.mesh.local/workload/demo-app"
EOF

log "DP token auth disabled on kuma-cp (KUMA_DP_SERVER_AUTH_TYPE=none); skipping token minting"
log "done"
