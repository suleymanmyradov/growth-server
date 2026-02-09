#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${DIR}/.." && pwd)"

# Clean logs directory for fresh logs
LOG_DIR="${ROOT}/logs"
rm -rf "${LOG_DIR}"
mkdir -p "${LOG_DIR}"

echo "=== Restarting all services ==="
echo ""

"${DIR}/killall.sh"
sleep 2
"${DIR}/checkall.sh" || true
sleep 2
cd "${ROOT}"
make build
sleep 2
"${DIR}/runall.sh"
sleep 2
"${DIR}/checkall.sh"
