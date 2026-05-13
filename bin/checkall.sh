#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${DIR}/.." && pwd)"
BINDIR="${DIR}"

services=(auth client search conversations gateway)

status_service() {
  local name="$1"

  # Check for exact binary path match (avoid partial matches like client matching conversations)
  local pattern="${BINDIR}/${name}"
  if pgrep -f "^${pattern} " >/dev/null 2>&1; then
    local found
    found="$(pgrep -f "^${pattern} " | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
    printf "[OK]   %-12s running (pid %s)\n" "$name" "$found"
    return 0
  fi

  printf "[FAIL] %-12s not running\n" "$name"
  return 1
}

main() {
  echo "Checking services from ${ROOT}"

  local overall=0
  for i in "${!services[@]}"; do
    local name="${services[$i]}"
    if ! status_service "$name"; then
      overall=1
    fi
  done

  exit "$overall"
}

main "$@"
