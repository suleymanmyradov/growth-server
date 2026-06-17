#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "${DIR}/.." && pwd)"
BINDIR="${DIR}"

services=(auth client search notifications ai-coach search-sync gateway adminway)

status_service() {
  local name="$1"
  local config="$2"

  # Check for exact binary path match with config file
  local pattern="${BINDIR}/${name}.*${config}"
  if pgrep -f "${pattern}" >/dev/null 2>&1; then
    local found
    found="$(pgrep -f "${pattern}" | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
    printf "[OK]   %-25s running (pid %s)\n" "$name" "$found"
    return 0
  fi

  printf "[FAIL] %-25s not running\n" "$name"
  return 1
}

main() {
  echo "Checking services from ${ROOT}"

  local overall=0
  
  # Check individual services with their configs
  status_service "auth" "auth.yaml" || overall=1
  status_service "client" "client.yaml" || overall=1
  status_service "search" "search.yaml" || overall=1
  status_service "notifications" "notifications.yaml" || overall=1
  status_service "ai-coach" "aicoach.yaml" || overall=1
  status_service "ai-coach-consumer" "ai-coach.yaml" || overall=1
  status_service "search-sync" "search-sync.yaml" || overall=1
  status_service "gateway" "growthapi.yaml" || overall=1
  status_service "adminway" "adminapi.yaml" || overall=1

  exit "$overall"
}

main "$@"
