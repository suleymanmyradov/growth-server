#!/usr/bin/env bash
# Start all long-running Go services in a single tmux session, each in its
# own window, each hot-reloaded via air (air-verse/air) using the per-service
# configs in .air/<svc>.toml.
#
# Used by: `make dev-all` / `make tmux-start`
set -euo pipefail

SESSION="growth"
BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$BACKEND_DIR"

# --- ensure air is available -------------------------------------------------
if ! command -v air >/dev/null 2>&1; then
  echo "air not found, installing github.com/air-verse/air@latest ..."
  go install github.com/air-verse/air@latest
  # $(go env GOPATH)/bin may not be on PATH in some shells
  export PATH="$(go env GOPATH)/bin:$PATH"
fi

# --- ensure tmux is available ------------------------------------------------
if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux not found. Install it first: brew install tmux" >&2
  exit 1
fi

# --- wait for infra deps to accept connections -------------------------------
# App services (notably filemanager) fatal-exit on a missing dependency, and
# `air` only restarts on file changes — not on child exit — so a service that
# dies at startup stays dead silently. Wait for the infra ports first so every
# service finds its deps ready. Run `make docker-up` before this script.
#
# host:port pairs for the dev infra (see deploy/docker-compose.yml).
DEPS=(
  "127.0.0.1:5434"   # postgres
  "127.0.0.1:6379"   # redis
  "127.0.0.1:9092"   # redpanda (kafka)
  "127.0.0.1:7700"   # meilisearch
  "127.0.0.1:9000"   # minio
)

wait_for_port() {
  local host="$1" port="$2" name="$3" waited=0
  while ! nc -z -w1 "$host" "$port" >/dev/null 2>&1; do
    waited=$((waited + 1))
    if [ "$waited" -ge 60 ]; then
      echo "  [FAIL] $name ($host:$port) not up after 60s — run 'make docker-up' first." >&2
      return 1
    fi
    [ "$waited" -eq 1 ] && echo "  waiting for $name ($host:$port)..."
    sleep 1
  done
  echo "  [ok] $name ($host:$port) ready"
}

echo "Checking infra deps (run 'make docker-up' first if any fail):"
deps_ready=0
for dep in "${DEPS[@]}"; do
  host="${dep%%:*}"
  port="${dep##*:}"
  # Derive a friendly name from the port.
  case "$port" in
    5434) name="postgres" ;;
    6379) name="redis" ;;
    9092) name="redpanda" ;;
    7700) name="meilisearch" ;;
    9000) name="minio" ;;
    *)    name="$dep" ;;
  esac
  if ! wait_for_port "$host" "$port" "$name"; then
    deps_ready=1
  fi
done
if [ "$deps_ready" -ne 0 ]; then
  echo "One or more infra deps are not reachable. Aborting." >&2
  exit 1
fi
echo ""

# --- recreate session --------------------------------------------------------
if tmux has-session -t "$SESSION" 2>/dev/null; then
  echo "Session '$SESSION' already exists — killing and recreating."
  tmux kill-session -t "$SESSION"
fi

# Long-running services only. billing-reconciler is a one-shot CLI and is
# intentionally omitted (use `make dev-billing-reconciler` for ad-hoc runs).
# Order: core dependencies first so logs read top-to-bottom in tmux.
SERVICES=(
  auth
  client
  search
  notifications
  ai-coach
  ai-coach-consumer
  filemanager
  search-sync
  gateway
  ai-gateway
  adminway
)

FIRST="${SERVICES[0]}"
tmux new-session -d -s "$SESSION" -n "$FIRST" -c "$BACKEND_DIR"
tmux send-keys -t "$SESSION:$FIRST" "air -c .air/$FIRST.toml" C-m

for svc in "${SERVICES[@]:1}"; do
  tmux new-window -t "$SESSION" -n "$svc" -c "$BACKEND_DIR"
  tmux send-keys -t "$SESSION:$svc" "air -c .air/$svc.toml" C-m
done

# Land on the gateway window by default (the most-looked-at service).
tmux select-window -t "$SESSION:gateway"

echo ""
echo "All services launching in tmux session '$SESSION' (one window per service)."
echo ""
echo "  Attach:          make tmux-attach    (or: tmux attach -t $SESSION)"
echo "  Stop all:        make tmux-stop      (or: tmux kill-session -t $SESSION)"
echo "  Restart one:     in its window: Ctrl-C, then Up + Enter"
echo "  Switch window:   Ctrl-b then the window number, or Ctrl-b n / Ctrl-b p"
echo "  List windows:    Ctrl-b w"
echo ""
echo "Services: ${SERVICES[*]}"
