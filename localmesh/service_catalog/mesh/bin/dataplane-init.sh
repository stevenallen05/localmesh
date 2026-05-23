#!/usr/bin/env bash
set -euo pipefail

# Runs inside the localmesh dataplane container, which joins the target
# workload's network namespace via compose `network_mode: "service:<svc>"`.
# Starts kuma-dp, waits for envoy listeners, then installs the
# transparent-proxy iptables rules into the shared netns. The workload's
# own container is unmodified.
#
# Required env (set by the rendered compose):
#   WORKLOAD_NAME   name as it appears on kuma.io/service
#   APP_PORT        port the workload listens on
#   APP_PROTOCOL    kuma.io/protocol tag (http|grpc|tcp)
#   CP_HOST         control-plane host:port for the DP gRPC (e.g. kuma-cp:5678)

DATAPLANE_FILE=/etc/kuma/dataplane.yaml
TPROXY_FILE=/etc/kuma/transparent-proxy.yaml

log() { printf '[localmesh-dataplane %s] %s\n' "$WORKLOAD_NAME" "$*" >&2; }

APP_ADDR=$(hostname -i | awk '{print $1}')
log "advertising ${WORKLOAD_NAME} at ${APP_ADDR}:${APP_PORT}"

DP_PID=
shutdown() {
  log "shutdown signal received"
  [ -n "${DP_PID}" ] && kill "${DP_PID}" 2>/dev/null || true
}
trap shutdown TERM INT

log "starting kuma-dp"
runuser --user kuma-data-plane-proxy -- \
  /usr/local/bin/kuma-dp run \
    --cp-address "https://${CP_HOST}" \
    --dataplane-file "${DATAPLANE_FILE}" \
    --dataplane-var "name=${WORKLOAD_NAME}" \
    --dataplane-var "address=${APP_ADDR}" \
    --dataplane-var "port=${APP_PORT}" \
    --dataplane-var "protocol=${APP_PROTOCOL}" &
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
