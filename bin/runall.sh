#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${DIR}/.." && pwd)"
BINDIR="${DIR}"
CONFIGDIR="${ROOT}/services/gateway/growth/etc"
LOGDIR="${ROOT}/logs"

start_service() {
  local name="$1"
  local binary="$2"
  local config="$3"
  local service_log_dir="${LOGDIR}/${name}"
  local access_log="${service_log_dir}/access.log"
  local error_log="${service_log_dir}/error.log"

  mkdir -p "$service_log_dir"

  echo "Starting ${name} with ${config}..."
  # stdout -> access log, stderr -> error log to keep separation.
  (cd "$ROOT" && nohup "$binary" -f "$config" >>"$access_log" 2>>"$error_log" &)
}

mkdir -p "$CONFIGDIR" "$LOGDIR"

# Start microservices
start_service auth-rpc "${BINDIR}/auth" "${ROOT}/services/microservices/auth/rpc/etc/auth.yaml"
start_service client-rpc "${BINDIR}/client" "${ROOT}/services/microservices/client/rpc/etc/client.yaml"
start_service search-rpc "${BINDIR}/search" "${ROOT}/services/microservices/search/rpc/etc/search.yaml"
start_service conversations-rpc "${BINDIR}/conversations" "${ROOT}/services/microservices/conversations/rpc/etc/conversations.yaml"

# Start gateway
start_service gateway "${BINDIR}/gateway" "${CONFIGDIR}/growthapi.yaml"

echo "All services started."
