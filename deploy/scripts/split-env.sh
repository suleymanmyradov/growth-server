#!/usr/bin/env bash
# Split the monolithic deploy/.env.prod into per-service env files.
#
#   .env.prod      — stays, but is now used ONLY for compose-time
#                    interpolation (postgres bootstrap, domains, image tags,
#                    frontend build args). No longer attached to app containers.
#   .env.shared    — non-secret values needed by many services (issuer/audience,
#                    public keys, AI model names, feature flags, Sentry DSN).
#   .env.<svc>     — only the secrets that one service needs.
#
# Idempotent: existing values are never overwritten — re-running only fills
# gaps. deploy.sh runs this on every deploy; after the first split it is a
# no-op.
#
# POSTGRES_DATASOURCE is intentionally NOT copied — setup-db-roles.sh writes a
# per-service-role datasource into each file instead.
set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$DEPLOY_DIR/.env.prod"
[ -f "$SRC" ] || { echo "error: $SRC not found" >&2; exit 1; }

SHARED_VARS="APP_URL API_BASE_URL JWT_ISSUER JWT_AUDIENCE JWT_PUBLIC_KEY ADMIN_JWT_PUBLIC_KEY
EMAIL_FROM_ADDRESS
GOOGLE_CLIENT_ID GOOGLE_IOS_CLIENT_ID GOOGLE_ANDROID_CLIENT_ID
PADDLE_ENABLED PADDLE_ENVIRONMENT REVENUECAT_ENABLED
AI_BASE_URL AI_SPEECH_BASE_URL AI_STT_MODEL
AI_MODEL_CHEAP AI_MODEL_CHEAP_LONG AI_MODEL_CLASSIFIER AI_MODEL_CHAT AI_MODEL_FALLBACK
AI_FB1_CHEAP AI_FB1_CHEAP_LONG AI_FB1_CLASSIFIER AI_FB1_CHAT AI_FB1_FALLBACK
AI_FB2_CHEAP AI_FB2_CHEAP_LONG AI_FB2_CLASSIFIER AI_FB2_CHAT AI_FB2_FALLBACK
AI_FB3_CHEAP AI_FB3_CHEAP_LONG AI_FB3_CLASSIFIER AI_FB3_CHAT AI_FB3_FALLBACK
MINIO_PUBLIC_BASE_URL SENTRY_DSN SENTRY_ENVIRONMENT"

svc_vars() {
  case "$1" in
    auth)               echo "JWT_PRIVATE_KEY REDIS_PASSWORD RESEND_API_KEY GOOGLE_CLIENT_SECRET SERVICE_AUTH_SECRET" ;;
    client)             echo "REDIS_PASSWORD SERVICE_AUTH_SECRET PADDLE_API_KEY PADDLE_WEBHOOK_SECRET REVENUECAT_API_KEY REVENUECAT_WEBHOOK_SECRET" ;;
    search)             echo "MEILI_MASTER_KEY SERVICE_AUTH_SECRET" ;;
    ai-coach)           echo "REDIS_PASSWORD MEILI_MASTER_KEY AI_API_KEY OPENROUTER_API_KEY SERVICE_AUTH_SECRET" ;;
    ai-gateway)         echo "REDIS_PASSWORD AI_API_KEY OPENROUTER_API_KEY SERVICE_AUTH_SECRET" ;;
    ai-coach-consumer)  echo "REDIS_PASSWORD AI_API_KEY OPENROUTER_API_KEY" ;;
    filemanager)        echo "MINIO_ACCESS_KEY MINIO_SECRET_KEY SERVICE_AUTH_SECRET" ;;
    notifications)      echo "RESEND_API_KEY SERVICE_AUTH_SECRET" ;;
    search-sync)        echo "MEILI_MASTER_KEY" ;;
    analytics-consumer) echo "" ;;
    gateway)            echo "REDIS_PASSWORD SERVICE_AUTH_SECRET" ;;
    adminway)           echo "ADMIN_JWT_PRIVATE_KEY REDIS_PASSWORD SERVICE_AUTH_SECRET" ;;
    *) echo "" ;;
  esac
}

# copy_var <name> <dst-file> — append "name=value" from .env.prod unless dst
# already sets it.
copy_var() {
  local name="$1" dst="$2" line
  grep -q "^${name}=" "$dst" 2>/dev/null && return 0
  line=$(grep "^${name}=" "$SRC" | head -1 || true)
  [ -z "$line" ] && return 1
  echo "$line" >> "$dst"
}

# --- .env.shared -------------------------------------------------------------
touch "$DEPLOY_DIR/.env.shared"
missing_shared=""
for v in $SHARED_VARS; do
  copy_var "$v" "$DEPLOY_DIR/.env.shared" || missing_shared="$missing_shared $v"
done

# --- .env.<svc> --------------------------------------------------------------
missing_svc=""
for svc in auth client search ai-coach ai-gateway ai-coach-consumer filemanager notifications search-sync analytics-consumer gateway adminway; do
  f="$DEPLOY_DIR/.env.$svc"
  touch "$f"
  for v in $(svc_vars "$svc"); do
    # Private JWT keys may already live in their dedicated files from an older
    # split — copy_var won't clobber them either way.
    copy_var "$v" "$f" || missing_svc="$missing_svc $svc:$v"
  done
done

# ADMIN_POSTGRES_DATASOURCE (legacy name) is superseded by per-role
# POSTGRES_DATASOURCE written by setup-db-roles.sh — nothing to copy.

[ -z "$missing_shared" ] || echo "note: not found in .env.prod (fill manually if needed):$missing_shared" >&2
[ -z "$missing_svc" ] || echo "note: not found in .env.prod (fill manually if needed):$missing_svc" >&2
echo "==> env split complete (.env.shared + deploy/.env.<svc>)"
