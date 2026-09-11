#!/usr/bin/env bash
# =============================================================================
# Nightly PostgreSQL backup for the growth stack.
#
# Dumps the growthmind database in pg_dump custom format (compressed,
# restorable with pg_restore) into /home/ubuntu/backups and prunes dumps
# older than RETENTION_DAYS.
#
# Install on the VM (already done if you followed deploy/README.md):
#   install -m 755 deploy/scripts/backup-postgres.sh /home/ubuntu/backup-postgres.sh
#   systemctl enable --now growth-backup.timer
#
# Restore (into the compose Postgres):
#   docker exec -i deploy-postgres-1 pg_restore -U growthmind -d growthmind \
#     --clean --if-exists < /home/ubuntu/backups/growthmind-<ts>.dump
# =============================================================================
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/home/ubuntu/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
CONTAINER="${CONTAINER:-deploy-postgres-1}"
DB_USER="${POSTGRES_USER:-growthmind}"
DB_NAME="${POSTGRES_DB:-growthmind}"

mkdir -p "$BACKUP_DIR"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
OUT="$BACKUP_DIR/$DB_USER-$STAMP.dump"

docker exec "$CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$OUT"

# Prune dumps older than the retention window.
find "$BACKUP_DIR" -name "$DB_USER-*.dump" -type f -mtime "+$RETENTION_DAYS" -delete

SIZE="$(du -h "$OUT" | cut -f1)"
echo "$(date -Is) backup ok: $OUT ($SIZE)"
