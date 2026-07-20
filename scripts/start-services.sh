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
