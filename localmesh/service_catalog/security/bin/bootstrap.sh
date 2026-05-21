#!/usr/bin/env bash
set -euo pipefail

# Runs once per compose-up: waits for kuma-cp to answer, points kumactl at
# it, then applies every YAML it finds in /etc/localmesh/bootstrap/. The
# directory is bind-mounted in by the rendered compose and may include
# both static plugin files (mesh-defaults, ingress.dataplane) and
# rendered-at-build-time files (ingress routes, depends_on permissions).

CP_HTTP=http://kuma-cp:5681
KUMACTL=/usr/local/bin/kumactl
BOOTSTRAP_DIR=/etc/localmesh/bootstrap

log() { printf '[localmesh-bootstrap] %s\n' "$*" >&2; }

log "waiting for ${CP_HTTP}/ (200 expected)"
until curl --silent --fail "${CP_HTTP}/" >/dev/null; do sleep 1; done

log "configuring kumactl"
"${KUMACTL}" config control-planes add \
  --address "${CP_HTTP}" --name default --overwrite >/dev/null

shopt -s nullglob
for f in "${BOOTSTRAP_DIR}"/*.yaml; do
  log "applying $(basename "$f")"
  "${KUMACTL}" apply -f "$f"
done

log "done"
