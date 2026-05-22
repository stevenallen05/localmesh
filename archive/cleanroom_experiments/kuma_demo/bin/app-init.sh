#!/usr/bin/env bash
set -euo pipefail

# Wraps the kuma-counter-demo container so it brings up the demo app, the
# kuma-dp sidecar, and the transparent-proxy iptables rules in the order
# the upstream quickstart does (app first, then sidecar, then tproxy).
#
# Required env (set in kuma.compose.yaml):
#   WORKLOAD_NAME, APP_PORT, CP_HOST, KUMA_VERSION, TOKEN_DIR

DATAPLANE_FILE=/etc/kuma/dataplane.yaml
TPROXY_FILE=/etc/kuma/transparent-proxy.yaml
APP_BIN=/kuma-counter-demo

log() { printf '[app-init %s] %s\n' "$WORKLOAD_NAME" "$*" >&2; }

log "installing iptables (kuma binaries are mounted from tools/)"
apt-get update >/dev/null
apt-get install --yes --no-install-recommends iptables ca-certificates >/dev/null

id -u kuma-data-plane-proxy >/dev/null 2>&1 \
  || useradd --uid 5678 --user-group kuma-data-plane-proxy

APP_ADDR=$(hostname -i | awk '{print $1}')
log "advertising ${WORKLOAD_NAME} at ${APP_ADDR}:${APP_PORT}"

APP_PID=
DP_PID=
shutdown() {
  log "shutdown signal received"
  [ -n "${APP_PID}" ] && kill "${APP_PID}" 2>/dev/null || true
  [ -n "${DP_PID}"  ] && kill "${DP_PID}"  2>/dev/null || true
}
trap shutdown TERM INT

log "starting demo app: ${APP_BIN}"
"${APP_BIN}" &
APP_PID=$!

log "starting kuma-dp"
runuser --user kuma-data-plane-proxy -- \
  /usr/local/bin/kuma-dp run \
    --cp-address "https://${CP_HOST}" \
    --dataplane-file "${DATAPLANE_FILE}" \
    --dataplane-var "name=${WORKLOAD_NAME}" \
    --dataplane-var "address=${APP_ADDR}" \
    --dataplane-var "port=${APP_PORT}" &
DP_PID=$!

log "waiting for kuma-dp inbound listener on 15006"
for _ in $(seq 1 30); do
  if (exec 3<>/dev/tcp/127.0.0.1/15006) 2>/dev/null; then
    exec 3<&-; exec 3>&-
    break
  fi
  sleep 1
done

log "installing transparent-proxy"
kumactl install transparent-proxy --config-file "${TPROXY_FILE}"

log "ready"
wait -n
