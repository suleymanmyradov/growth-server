#!/usr/bin/env bash
# Enforce the private bucket policy on MinIO — safe to run repeatedly.
#
# The bucket must stay PRIVATE: it holds avatars and GDPR data exports, and
# Caddy proxies /files/* straight to MinIO — anonymous access there would let
# anyone enumerate/download other users' exports. All access goes through
# short-lived presigned URLs (SigV4 in the query string) or the authenticated
# filemanager RPC. An empty-statement bucket policy revokes every anonymous
# grant — "mc anonymous set none/private" rewrites preset statements only and
# silently keeps custom policies (verified on mc RELEASE.2025-08-13).
#
# Runs ON the VM. Called by deploy.sh on every deploy and by
# aws/bootstrap.sh on first provision. Exits non-zero on any failure so the
# calling deploy goes red instead of silently leaving the bucket public.
set -euo pipefail

REPO_DIR="${REPO_DIR:-/home/ubuntu/growth-server}"
BUCKET="${MINIO_BUCKET:-growthmind}"
COMPOSE=(docker compose -f "$REPO_DIR/deploy/docker-compose.prod.yml"
  --env-file "$REPO_DIR/deploy/.env.prod" --project-directory "$REPO_DIR/deploy")

echo "==> Waiting for MinIO readiness"
for _ in $(seq 1 30); do
  "${COMPOSE[@]}" exec -T minio mc ready local >/dev/null 2>&1 && break
  sleep 2
done
# Hard-fail if MinIO never became ready (previous loop only attempted).
"${COMPOSE[@]}" exec -T minio mc ready local >/dev/null

echo "==> Enforcing private bucket policy on $BUCKET"
"${COMPOSE[@]}" exec -T minio sh -c '
  set -e
  mc alias set local http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
  mc mb --ignore-existing local/'"$BUCKET"' >/dev/null
  printf "%s" "{\"Version\":\"2012-10-17\",\"Statement\":[]}" > /tmp/private-policy.json
  mc anonymous set-json /tmp/private-policy.json local/'"$BUCKET"'
'

echo "==> Verifying anonymous access is revoked"
# Round-trip the policy back out — a failed/silent apply must not pass.
# Match on an empty Statement array (not exact JSON) so field ordering or
# mc-side formatting can't false-fail the check.
policy=$("${COMPOSE[@]}" exec -T minio mc anonymous get-json local/"$BUCKET")
if ! printf '%s' "$policy" | tr -d '[:space:]' | grep -q '"Statement":\[\]'; then
  echo "!! Anonymous policy on $BUCKET has non-empty statements — refusing to continue:" >&2
  echo "$policy" >&2
  exit 1
fi
echo "==> Bucket $BUCKET is private"
