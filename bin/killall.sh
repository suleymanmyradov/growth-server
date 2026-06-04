#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
BINDIR="${DIR}"

# List of service binaries and their config files for precise matching
# Format: "service_name:config_file"
SERVICES=(
  "auth:auth.yaml"
  "client:client.yaml"
  "search:search.yaml"
  "notifications:notifications.yaml"
  "ai-coach-rpc:aicoach.yaml"
  "ai-coach-consumer:ai-coach.yaml"
  "search-sync:search-sync.yaml"
  "gateway:growthapi.yaml"
)

echo "Stopping services..."

for service in "${SERVICES[@]}"; do
  name="${service%%:*}"
  config="${service##*:}"

  # Map service name to actual binary name.
  case "$name" in
    ai-coach-rpc) binary="ai-coach" ;;
    ai-coach-consumer) binary="ai-coach-consumer" ;;
    *) binary="$name" ;;
  esac

  # Match binary path with config file for precise identification
  pattern="${BINDIR}/${binary}.*${config}"

  echo "Stopping $name..."
  # pkill returns 0 if at least one process matched, 1 otherwise.
  # We suppress output and ignore exit code if not found.
  pkill -f "$pattern" || true
done

# Wait a moment for processes to actually exit
sleep 1

# Optional: Verify they are gone or force kill
for service in "${SERVICES[@]}"; do
  name="${service%%:*}"
  config="${service##*:}"
  case "$name" in
    ai-coach-rpc) binary="ai-coach" ;;
    ai-coach-consumer) binary="ai-coach-consumer" ;;
    *) binary="$name" ;;
  esac
  pattern="${BINDIR}/${binary}.*${config}"
  if pgrep -f "$pattern" >/dev/null; then
    echo "$name still running, force killing..."
    pkill -9 -f "$pattern" || true
  fi
done

# Clean up any stale pid file if it exists (legacy)
if [[ -f "${DIR}/app.pids" ]]; then
  rm -f "${DIR}/app.pids"
fi

rm -rf "${DIR}/logs"

echo "All services stopped."
