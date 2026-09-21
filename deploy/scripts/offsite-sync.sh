#!/usr/bin/env bash
# =============================================================================
# Optional off-site backup sync for the growth stack.
#
# Mirrors the local backups in /home/ubuntu/backups (pg_dump *.dump files and
# MinIO *.tar.gz archives — scratch files are excluded) to any S3-compatible
# object store: AWS S3, UpCloud Object Storage, Backblaze B2, Cloudflare R2,
# MinIO, etc. Sync runs inside a throwaway amazon/aws-cli container so nothing
# extra is installed on the host. `aws s3 sync` is idempotent (re-uploads only
# new/changed files) and --endpoint-url makes it work with any S3-compatible
# provider; aws-cli was chosen over mc/rclone because the official image is
# well-maintained and `s3 sync` semantics are widely understood.
#
# DISABLED BY DEFAULT. The script exits 0 immediately unless
# OFFSITE_ENABLED=true is set — safe to install the timer before choosing a
# destination.
#
# Configuration (all env-driven):
#   Create /home/ubuntu/backups/.offsite.env (chmod 600) with e.g.:
#
#     OFFSITE_ENABLED=true
#     OFFSITE_S3_BUCKET=my-backup-bucket
#     OFFSITE_S3_ENDPOINT=https://s3.us-west-004.backblazeb2.com
#     OFFSITE_S3_PREFIX=growth-vm          # optional key prefix
#     AWS_ACCESS_KEY_ID=...
#     AWS_SECRET_ACCESS_KEY=...
#     AWS_DEFAULT_REGION=us-west-004       # optional; default us-east-1
#
#   OFFSITE_ENV_FILE overrides the env-file path (e.g. to reuse
#   /home/ubuntu/growth-server/deploy/.env.prod — just add the OFFSITE_* and
#   AWS_* vars there). Required when enabled: OFFSITE_S3_BUCKET,
#   OFFSITE_S3_ENDPOINT, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY.
#   Note: values in the env file take precedence over the ambient
#   environment — keep it limited to OFFSITE_*/AWS_* settings.
#
# Install on the VM:
#   install -m 755 deploy/scripts/offsite-sync.sh /home/ubuntu/offsite-sync.sh
#   install -m 644 deploy/systemd/growth-backup-offsite.{service,timer} /etc/systemd/system/
#   systemctl daemon-reload && systemctl enable --now growth-backup-offsite.timer
#
# Test without uploading (aws s3 sync supports --dryrun):
#   sudo bash -c 'DRYRUN=true OFFSITE_ENV_FILE=/home/ubuntu/backups/.offsite.env \
#     /home/ubuntu/offsite-sync.sh'
# or inspect what the timer will do:
#   sudo systemctl start growth-backup-offsite.service
#   journalctl -u growth-backup-offsite.service
# =============================================================================
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/home/ubuntu/backups}"
OFFSITE_ENV_FILE="${OFFSITE_ENV_FILE:-$BACKUP_DIR/.offsite.env}"
AWS_IMAGE="${AWS_IMAGE:-amazon/aws-cli:2}"
DRYRUN="${DRYRUN:-false}"

# Source credentials/config if the env file exists (enabling can live there).
# shellcheck disable=SC1090
if [[ -f "$OFFSITE_ENV_FILE" ]]; then
  . "$OFFSITE_ENV_FILE"
fi

if [[ "${OFFSITE_ENABLED:-false}" != "true" ]]; then
  echo "$(date -Is) offsite sync skipped: OFFSITE_ENABLED != true"
  exit 0
fi

# Validate required configuration before touching docker.
missing=()
for var in OFFSITE_S3_BUCKET OFFSITE_S3_ENDPOINT AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY; do
  if [[ -z "${!var:-}" ]]; then
    missing+=("$var")
  fi
done
if (( ${#missing[@]} > 0 )); then
  echo "$(date -Is) offsite sync failed: missing required vars: ${missing[*]} (set them in $OFFSITE_ENV_FILE)" >&2
  exit 1
fi

OFFSITE_S3_PREFIX="${OFFSITE_S3_PREFIX:-}"
OFFSITE_S3_PREFIX="${OFFSITE_S3_PREFIX#/}"   # strip accidental leading slash
AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}"

# Vars sourced from the env file are shell-local; export so docker -e forwards
# them into the container (avoids putting secrets in docker's argv).
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_DEFAULT_REGION

DEST="s3://${OFFSITE_S3_BUCKET}${OFFSITE_S3_PREFIX:+/$OFFSITE_S3_PREFIX}"

args=(s3 sync /backups "$DEST"
  --endpoint-url "$OFFSITE_S3_ENDPOINT"
  --exclude "*" --include "*.dump" --include "*.tar.gz")
if [[ "$DRYRUN" == "true" ]]; then
  args+=(--dryrun)
fi

docker run --rm \
  -v "$BACKUP_DIR":/backups:ro \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e AWS_DEFAULT_REGION \
  "$AWS_IMAGE" "${args[@]}"

echo "$(date -Is) offsite sync ok: $BACKUP_DIR -> $DEST (dryrun=$DRYRUN)"
