#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
BINDIR="${DIR}"

# List of service binary names to kill
SERVICES=("auth" "client" "search" "conversations" "gateway")

echo "Stopping services..."

for name in "${SERVICES[@]}"; do
  # We use pkill -f to match the full command line, looking for the binary path.
  # This matches how runall.sh starts them (absolute path to binary).
  pattern="${BINDIR}/${name}"
  
  echo "Stopping $name..."
  # pkill returns 0 if at least one process matched, 1 otherwise.
  # We suppress output and ignore exit code if not found.
  pkill -f "$pattern" || true
done

# Wait a moment for processes to actually exit
sleep 1

# Optional: Verify they are gone or force kill
for name in "${SERVICES[@]}"; do
  pattern="${BINDIR}/${name}"
  if pgrep -f "$pattern" >/dev/null; then
    echo "$name still running, force killing..."
    pkill -9 -f "$pattern" || true
  fi
done

# Clean up any stale pid file if it exists (legacy)
if [[ -f "${DIR}/app.pids" ]]; then
  rm -f "${DIR}/app.pids"
fi

echo "All services stopped."
