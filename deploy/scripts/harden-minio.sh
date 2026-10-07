#!/usr/bin/env bash
# Enforce the MinIO bucket policy — safe to run repeatedly.
#
# The bucket stays PRIVATE except for the public-content prefix: article
# cover images are public content served on an unauthenticated endpoint, so
# the policy grants anonymous s3:GetObject on articles/* only. Everything
# else — avatars/ (served via presigned URLs minted at read time by the
# gateway) and exports/ (GDPR data, presigned short-lived) — must never be
# anonymously readable. Caddy additionally only proxies /files/*/articles/*
# without a signature; every other /files/* request must carry X-Amz-Signature
# or it never reaches MinIO (deploy/caddy/Caddyfile).
#
# "mc anonymous set none/private" only rewrites preset statements and
# silently keeps custom policies (verified on mc RELEASE.2025-08-13), so the
# policy is reset with an empty-statement set-json before the scoped grant
# is applied.
#
# Runs ON the VM. Called by deploy.sh on every deploy and by
# aws/bootstrap.sh on first provision. Exits non-zero on any failure so the
# calling deploy goes red instead of silently leaving the bucket open.
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

echo "==> Enforcing scoped anonymous policy on $BUCKET (articles/* download only)"
"${COMPOSE[@]}" exec -T minio sh -c '
  set -e
  mc alias set local http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
  mc mb --ignore-existing local/'"$BUCKET"' >/dev/null
  printf "%s" "{\"Version\":\"2012-10-17\",\"Statement\":[]}" > /tmp/private-policy.json
  mc anonymous set-json /tmp/private-policy.json local/'"$BUCKET"'
  mc anonymous set download local/'"$BUCKET"'/articles
'

echo "==> Verifying the policy round-trips"
# A failed/silent apply must not pass: the policy must contain the
# articles/* download grant and no bucket-wide grant.
policy=$("${COMPOSE[@]}" exec -T minio mc anonymous get-json local/"$BUCKET")
if ! printf '%s' "$policy" | tr -d '[:space:]' | grep -q "s3:::$BUCKET/articles/"; then
  echo "!! Anonymous policy on $BUCKET does not grant articles/* download — refusing to continue:" >&2
  echo "$policy" >&2
  exit 1
fi
if printf '%s' "$policy" | tr -d '[:space:]' | grep -q "s3:::$BUCKET/\*"; then
  echo "!! Anonymous policy on $BUCKET grants the whole bucket — refusing to continue:" >&2
  echo "$policy" >&2
  exit 1
fi

# Functional round-trip through the public origin: articles/ must answer
# 200 unsigned, exports/ must refuse. Skipped with a loud warning when
# MINIO_PUBLIC_BASE_URL is unset (the public-URL scheme is broken anyway).
PUBLIC_BASE=$(grep -h '^MINIO_PUBLIC_BASE_URL=' \
  "$REPO_DIR/deploy/.env.shared" "$REPO_DIR/deploy/.env.prod" 2>/dev/null \
  | head -1 | cut -d= -f2- | tr -d '"' || true)
if [ -n "$PUBLIC_BASE" ]; then
  echo "==> Probing public object access via $PUBLIC_BASE"
  probe="policy-probe-$(date +%s)"
  "${COMPOSE[@]}" exec -T minio sh -c '
    printf probe | mc pipe local/'"$BUCKET"'/articles/'"$probe"'.txt
    printf probe | mc pipe local/'"$BUCKET"'/exports/'"$probe"'.txt
  '
  cleanup() {
    "${COMPOSE[@]}" exec -T minio sh -c '
      mc rm --force local/'"$BUCKET"'/articles/'"$probe"'.txt
      mc rm --force local/'"$BUCKET"'/exports/'"$probe"'.txt
    ' >/dev/null 2>&1 || true
  }
  trap cleanup EXIT

  public_code=000
  for _ in $(seq 1 5); do
    public_code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 \
      "$PUBLIC_BASE/$BUCKET/articles/$probe.txt" || echo 000)
    [ "$public_code" = "200" ] && break
    sleep 2
  done
  if [ "$public_code" != "200" ]; then
    echo "!! Unsigned GET on articles/ answered HTTP $public_code (expected 200)." >&2
    echo "!! Article images would 404 — check the Caddy /files/*/articles/* route" >&2
    echo "!! and MINIO_PUBLIC_BASE_URL." >&2
    exit 1
  fi
  private_code=000
  for _ in $(seq 1 5); do
    private_code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 \
      "$PUBLIC_BASE/$BUCKET/exports/$probe.txt" || echo 000)
    { [ "$private_code" = "403" ] || [ "$private_code" = "404" ]; } && break
    sleep 2
  done
  if [ "$private_code" != "403" ] && [ "$private_code" != "404" ]; then
    echo "!! Unsigned GET on exports/ answered HTTP $private_code (expected 403/404)." >&2
    echo "!! Private objects may be anonymously readable. Aborting." >&2
    exit 1
  fi
  echo "==> articles/ is publicly readable, exports/ stays private"
else
  echo "!! WARNING: MINIO_PUBLIC_BASE_URL unset — skipped the public-access" >&2
  echo "!! round-trip; article images will 404 until it is configured." >&2
fi

echo "==> Bucket $BUCKET policy enforced"
