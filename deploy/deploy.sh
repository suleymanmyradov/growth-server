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

COMPOSE=(docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod --profile admin --profile tools)

# The migrate service reads DATABASE_URL from the env file. Create it once
# from the existing Postgres credentials without printing anything.
if ! grep -q '^DATABASE_URL=' deploy/.env.prod; then
  PG_USER=$(grep '^POSTGRES_USER=' deploy/.env.prod | head -1 | cut -d= -f2-)
  PG_PASS=$(grep '^POSTGRES_PASSWORD=' deploy/.env.prod | head -1 | cut -d= -f2-)
  PG_DB=$(grep '^POSTGRES_DB=' deploy/.env.prod | head -1 | cut -d= -f2-)
  printf 'DATABASE_URL=postgres://%s:%s@postgres:5432/%s?sslmode=disable\n' \
    "$PG_USER" "$PG_PASS" "$PG_DB" >> deploy/.env.prod
fi

BACKEND_SERVICES=(auth client search ai-coach filemanager notifications ai-coach-consumer search-sync gateway ai-gateway adminway)

echo "==> Pulling images"
"${COMPOSE[@]}" pull "${BACKEND_SERVICES[@]}" migrate

echo "==> Running migrations"
# -T: without it, `compose run` attaches stdin and CONSUMES the rest of this
# script (the script itself is piped in over SSH), so the deploy would stop
# silently after migrations. No trailing args either: `compose run <svc> cmd`
# would REPLACE the service command (which carries -path/-database/up).
"${COMPOSE[@]}" run --rm -T migrate

echo "==> Recreating changed services"
"${COMPOSE[@]}" up -d --no-deps "${BACKEND_SERVICES[@]}" caddy

echo "==> Health checks"
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
