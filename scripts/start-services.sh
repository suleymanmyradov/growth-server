#!/usr/bin/env bash

SESSION="growth"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Kill existing session if it exists
tmux kill-session -t "$SESSION" 2>/dev/null

tmux new-session -d -s "$SESSION" -n "auth" -c "$DIR"
tmux send-keys -t "$SESSION:auth" "go run ./services/microservices/auth/rpc/auth.go -f ./services/microservices/auth/rpc/etc/auth.yaml" Enter

tmux new-window -t "$SESSION" -n "client" -c "$DIR"
tmux send-keys -t "$SESSION:client" "go run ./services/microservices/client/rpc/client.go -f ./services/microservices/client/rpc/etc/client.yaml" Enter

tmux new-window -t "$SESSION" -n "search" -c "$DIR"
tmux send-keys -t "$SESSION:search" "go run ./services/microservices/search/rpc/search.go -f ./services/microservices/search/rpc/etc/search.yaml" Enter

tmux new-window -t "$SESSION" -n "ai-coach" -c "$DIR"
tmux send-keys -t "$SESSION:ai-coach" "go run ./services/microservices/ai-coach/rpc/aicoach.go -f ./services/microservices/ai-coach/rpc/etc/aicoach.yaml" Enter

tmux new-window -t "$SESSION" -n "filemanager" -c "$DIR"
tmux send-keys -t "$SESSION:filemanager" "go run ./services/microservices/filemanager/rpc/filemanager.go -f ./services/microservices/filemanager/rpc/etc/filemanager.yaml" Enter

tmux new-window -t "$SESSION" -n "gateway" -c "$DIR"
tmux send-keys -t "$SESSION:gateway" "go run ./services/gateway/growth/growthapi.go -f ./services/gateway/growth/etc/growthapi.yaml" Enter

tmux new-window -t "$SESSION" -n "ai-coach-consumer" -c "$DIR"
tmux send-keys -t "$SESSION:ai-coach-consumer" "go run ./services/microservices/ai-coach-consumer/ai-coach.go -f ./services/microservices/ai-coach-consumer/etc/ai-coach.yaml" Enter

tmux new-window -t "$SESSION" -n "notifications" -c "$DIR"
tmux send-keys -t "$SESSION:notifications" "go run ./services/microservices/notifications/rpc/notifications.go -f ./services/microservices/notifications/rpc/etc/notifications.yaml" Enter

tmux new-window -t "$SESSION" -n "search-sync" -c "$DIR"
tmux send-keys -t "$SESSION:search-sync" "go run ./services/microservices/search-sync/search-sync.go -f ./services/microservices/search-sync/etc/search-sync.yaml" Enter

tmux new-window -t "$SESSION" -n "server" -c "$DIR"

tmux select-window -t "$SESSION:server"
tmux attach-session -t "$SESSION"
