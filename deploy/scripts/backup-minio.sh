#!/usr/bin/env bash
# =============================================================================
# Nightly MinIO backup for the growth stack.
#
# Archives the deploy_minio_data volume (all buckets + objects) into
# /home/ubuntu/backups as a tarball and prunes archives older than
# RETENTION_DAYS. Runs via a throwaway alpine container so no MinIO
# credentials or published ports are needed.
#
# Consistency note: the volume is mounted read-only but MinIO keeps
# serving writes during the archive — a file written mid-tar may be
# captured torn. At this scale (user uploads, append-mostly) that risk
# is acceptable; for strict consistency, stop the minio container first.
#
# Install on the VM:
#   install -m 755 deploy/scripts/backup-minio.sh /home/ubuntu/backup-minio.sh
#   systemctl enable --now growth-backup-minio.timer
#
# Restore (recreate the volume contents):
#   docker run --rm -v deploy_minio_data:/data \
#     -v /home/ubuntu/backups:/backup alpine:3.20 \
#     sh -c 'tar xzf /backup/minio-<ts>.tar.gz -C /data'
#   docker restart deploy-minio-1
# =============================================================================
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/home/ubuntu/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
VOLUME="${VOLUME:-deploy_minio_data}"
IMAGE="${IMAGE:-alpine:3.20}"

mkdir -p "$BACKUP_DIR"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
OUT="$BACKUP_DIR/minio-$STAMP.tar.gz"

docker run --rm \
  -v "$VOLUME":/data:ro \
  -v "$BACKUP_DIR":/backup \
  "$IMAGE" tar czf "/backup/minio-$STAMP.tar.gz" -C /data .

# Prune archives older than the retention window.
find "$BACKUP_DIR" -name "minio-*.tar.gz" -type f -mtime "+$RETENTION_DAYS" -delete

SIZE="$(du -h "$OUT" | cut -f1)"
echo "$(date -Is) backup ok: $OUT ($SIZE)"
