#!/usr/bin/env bash
# =============================================================================
# Bootstrap the evolella.com stack on a fresh Ubuntu 24.04 ARM EC2 instance.
# Run ON the instance as root:  bash bootstrap.sh
#
# Expects /home/ubuntu/env/growth.env (all secrets) uploaded beforehand.
# =============================================================================
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/suleymanmyradov/growth-server.git}"
REPO_DIR="/home/ubuntu/growth-server"
ENV_FILE="/home/ubuntu/env/growth.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE not found. Upload secrets first (see deploy/aws/README.md)." >&2
  exit 1
fi

echo "==> Installing Docker"
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg >/dev/null
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
apt-get update -qq
apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null

echo "==> Kernel tuning (Redpanda/Meilisearch) + swap"
sysctl -w vm.max_map_count=262144 >/dev/null
grep -q vm.max_map_count /etc/sysctl.conf || echo "vm.max_map_count=262144" >> /etc/sysctl.conf
if ! swapon --show | grep -q swapfile; then
  fallocate -l 4G /swapfile && chmod 600 /swapfile && mkswap /swapfile >/dev/null && swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

echo "==> Installing env file"
mkdir -p /home/ubuntu/env
test -f "$ENV_FILE" || { echo "ERROR: $ENV_FILE missing (rsync it first)"; exit 1; }
cp "$ENV_FILE" "$REPO_DIR/deploy/.env.prod"
chmod 600 "$REPO_DIR/deploy/.env.prod"

echo "==> Building images (sequential — 2 vCPU can't take parallel builds)"
for svc in auth client search ai-coach filemanager notifications ai-coach-consumer search-sync gateway ai-gateway adminway frontend admin-frontend; do
  echo "---- building $svc"
  docker compose -f "$REPO_DIR/deploy/docker-compose.prod.yml" \
    --env-file "$REPO_DIR/deploy/.env.prod" --project-directory "$REPO_DIR/deploy" build "$svc"
done

echo "==> Starting stack"
docker compose -f "$REPO_DIR/deploy/docker-compose.prod.yml" \
  --env-file "$REPO_DIR/deploy/.env.prod" --project-directory "$REPO_DIR/deploy" up -d

echo "==> Waiting for MinIO, then making the bucket publicly readable"
sleep 10
docker compose -f "$REPO_DIR/deploy/docker-compose.prod.yml" --project-directory "$REPO_DIR/deploy" \
  exec -T minio sh -c \
  'mc alias set local http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null 2>&1 \
   && mc mb --ignore-existing local/"$MINIO_ROOT_BUCKET" >/dev/null 2>&1 || true; \
   mc anonymous set download "local/$MINIO_ROOT_BUCKET" >/dev/null 2>&1 || true' || true
# Fallback: use explicit bucket name if env not visible inside container
BUCKET=growthmind
docker compose -f "$REPO_DIR/deploy/docker-compose.prod.yml" --project-directory "$REPO_DIR/deploy" \
  exec -T minio sh -c 'mc anonymous set download local/'"$BUCKET" || true

echo "==> Bootstrap complete."
docker compose -f "$REPO_DIR/deploy/docker-compose.prod.yml" --project-directory "$REPO_DIR/deploy" ps
