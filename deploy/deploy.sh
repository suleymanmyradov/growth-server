#!/usr/bin/env bash
# Production deploy entrypoint — runs ON the VM, invoked by GitHub Actions
# (see .github/workflows/deploy.yml) or manually over SSH.
#
# Steps: sync repo to origin/main → pull GHCR images → run migrations →
# recreate changed containers → verify health endpoints. Exits non-zero on
# any failure so the CI deploy job goes red.
#
# Rollback to a previous build (images are tagged sha-<commit>):
#   cd /home/ubuntu/growth-server
#   BACKEND_TAG=sha-<sha> docker compose -f deploy/docker-compose.prod.yml \
#     --env-file deploy/.env.prod --profile admin up -d --no-deps \
#     auth client search ai-coach filemanager notifications ai-coach-consumer \
#     search-sync gateway ai-gateway adminway caddy

set -euo pipefail

REPO_DIR=/home/ubuntu/growth-server
cd "$REPO_DIR"

# Sync tracked files (compose, Caddyfile, configs) to the pushed commit.
# deploy/.env.prod and logs/ are gitignored, so they survive the reset.
git fetch origin main
git reset --hard origin/main

COMPOSE=(docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod --profile admin --profile tools --profile monitoring)

# JWT private keys live in per-service env files so they never land in
# verifier containers' env (.env.prod is attached to every backend service).
# Create placeholders so compose doesn't fail on a missing env_file; the
# services fail closed at startup if no key is actually configured.
touch deploy/.env.auth deploy/.env.adminway
if ! grep -qs '^JWT_PRIVATE_KEY=' deploy/.env.auth deploy/.env.prod; then
  echo "!! WARNING: JWT_PRIVATE_KEY unset (deploy/.env.auth) — auth will only" >&2
  echo "!! sign legacy HS256 tokens while JWT_SECRET remains configured." >&2
fi
if ! grep -qs '^ADMIN_JWT_PRIVATE_KEY=' deploy/.env.adminway deploy/.env.prod; then
  echo "!! WARNING: ADMIN_JWT_PRIVATE_KEY unset (deploy/.env.adminway) — adminway" >&2
  echo "!! will only sign legacy HS256 tokens while JWT_SECRET remains configured." >&2
fi

# The migrate service reads DATABASE_URL from the env file. Create it once
# from the existing Postgres credentials without printing anything.
if ! grep -q '^DATABASE_URL=' deploy/.env.prod; then
  PG_USER=$(grep '^POSTGRES_USER=' deploy/.env.prod | head -1 | cut -d= -f2-)
  PG_PASS=$(grep '^POSTGRES_PASSWORD=' deploy/.env.prod | head -1 | cut -d= -f2-)
  PG_DB=$(grep '^POSTGRES_DB=' deploy/.env.prod | head -1 | cut -d= -f2-)
  printf 'DATABASE_URL=postgres://%s:%s@postgres:5432/%s?sslmode=disable\n' \
    "$PG_USER" "$PG_PASS" "$PG_DB" >> deploy/.env.prod
fi

BACKEND_SERVICES=(auth client search ai-coach filemanager notifications ai-coach-consumer search-sync analytics-consumer gateway ai-gateway adminway)
MONITORING_SERVICES=(prometheus grafana loki promtail tempo cadvisor)

echo "==> Pulling images"
"${COMPOSE[@]}" pull "${BACKEND_SERVICES[@]}" migrate "${MONITORING_SERVICES[@]}"

echo "==> Running migrations"
# -T + </dev/null: `compose run` attaches stdin by default and would CONSUME
# the rest of this script (the script itself is piped in over SSH), so the
# deploy silently stopped after migrations. No trailing args either:
# `compose run <svc> cmd` would REPLACE the service command (which carries
# -path/-database/up).
"${COMPOSE[@]}" run --rm -T migrate < /dev/null

echo "==> Recreating changed services"
# --force-recreate: deploy/config/*.yaml are bind-mounted, not baked into the
# image, so compose sees no container definition change on config-only deploys
# and would silently keep running the old config.
"${COMPOSE[@]}" up -d --no-deps --force-recreate "${BACKEND_SERVICES[@]}" "${MONITORING_SERVICES[@]}" caddy

echo "==> Health checks"
# Per-service docker healthchecks first — they catch a service that is
# running but wedged (the external /health check only proves Caddy is up).
# Containers are named deploy-<svc>-1 (compose project = deploy).
unhealthy=""
for _ in $(seq 1 40); do
  unhealthy=""
  for svc in "${BACKEND_SERVICES[@]}"; do
    status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' \
      "deploy-${svc}-1" 2>/dev/null || echo missing)
    [ "$status" = healthy ] || unhealthy="$unhealthy $svc:${status}"
  done
  [ -z "$unhealthy" ] && break
  sleep 3
done

if [ -n "$unhealthy" ]; then
  echo "!! Services failed health checks:$unhealthy" >&2
  docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod ps >&2
  exit 1
fi
echo "==> All services report healthy"

check() { curl -fsS --max-time 5 -o /dev/null "$1"; }
for _ in $(seq 1 30); do
  if check https://api.evolella.com/health \
    && check https://app.evolella.com \
    && check https://admin.evolella.com/login; then
    echo "==> Deploy healthy"
    docker image prune -f >/dev/null
    exit 0
  fi
  sleep 2
done

echo "!! Health checks failed after deploy" >&2
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod ps >&2
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod logs --tail 30 gateway adminway ai-gateway >&2
exit 1
