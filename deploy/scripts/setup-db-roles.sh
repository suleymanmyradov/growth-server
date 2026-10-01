#!/usr/bin/env bash
# Create least-privilege Postgres roles for each service and grant them access
# ONLY to the tables that service owns (see scripts/check-table-ownership.sh —
# keep this map in sync; the script warns about unmapped tables).
#
# App services stop connecting as the POSTGRES_USER superuser. Each service's
# deploy/.env.<svc> gets a POSTGRES_DATASOURCE pointing at its own role; the
# migrate one-shot keeps using the superuser DATABASE_URL (DDL needs ownership).
#
# Idempotent — safe to re-run. deploy.sh runs it on every deploy so tables
# added by the latest migrations get their grants immediately.
#
# Reads POSTGRES_USER/POSTGRES_PASSWORD/POSTGRES_DB from deploy/.env.prod.
# Role passwords are generated once and written back into the service env
# files (re-running keeps existing passwords).
#
# Usage (on the VM):  deploy/scripts/setup-db-roles.sh
set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE=(docker compose -f "$DEPLOY_DIR/docker-compose.prod.yml" --env-file "$DEPLOY_DIR/.env.prod")

getvar() { grep "^$1=" "$DEPLOY_DIR/.env.prod" | head -1 | cut -d= -f2-; }

PG_SUPERUSER=$(getvar POSTGRES_USER)
PG_DB=$(getvar POSTGRES_DB)
: "${PG_SUPERUSER:?POSTGRES_USER missing in deploy/.env.prod}"
: "${PG_DB:?POSTGRES_DB missing in deploy/.env.prod}"

# ---------------------------------------------------------------------------
# Role → table map. MUST stay in sync with get_allowed_tables() in
# scripts/check-table-ownership.sh — update both when a migration adds a table.
# "<svc>" = deploy/.env.<svc> file + role "svc_<svc>" (+ DML
# on the listed tables); prefix "RO:" grants SELECT only (read models /
# consumers that legitimately read another service's tables — kept minimal and
# reviewed like ownership exceptions).
# ---------------------------------------------------------------------------
ROLES=(auth client notifications adminway ai-coach ai-coach-consumer filemanager search-sync analytics-consumer)

tables_for() {
  case "$1" in
    auth)               echo "users user_oauth_accounts auth_deletion_outbox auth_event_outbox" ;;
    client)             echo "user_preferences coaching_profiles categories articles article_likes article_shares article_tags tags saved_articles saved_goals saved_habits goals habits goal_habits goal_milestones check_ins activities weekly_reviews plan_adjustments plans subscriptions subscription_provider_states paddle_checkouts upgrade_events user_profiles reports report_comments site_settings goal_templates habit_templates billing_webhook_events client_processed_events habit_missed_streaks client_event_outbox" ;;
    notifications)      echo "notifications reminders notification_preferences reminder_state processed_events notification_devices push_tickets notification_deliveries notification_recipients notification_habit_state notification_goal_state notification_event_outbox" ;;
    adminway)           echo "internal_users admin_audit_log RO:user_lifecycle_events RO:daily_metrics RO:retention_cohorts RO:conversion_funnels" ;;
    ai-coach)           echo "conversations conversation_messages user_facts" ;;
    ai-coach-consumer)  echo "ai_feedback ai_coach_check_ins ai_coach_profiles ai_coach_processed_events ai_coach_event_outbox" ;;
    filemanager)        echo "file_objects" ;;
    search-sync)        echo "RO:articles RO:categories RO:check_ins RO:conversation_messages RO:conversations RO:goals RO:habits RO:weekly_reviews" ;;
    analytics-consumer) echo "user_lifecycle_events daily_metrics retention_cohorts conversion_funnels analytics_processed_events" ;;
    *) echo "" ;;
  esac
}

role_name() { echo "svc_$(echo "$1" | tr '-' '_')"; }
env_file_for() { echo "$DEPLOY_DIR/.env.$1"; }

psql() { "${COMPOSE[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U "$PG_SUPERUSER" -d "$PG_DB" "$@"; }

# Generate (or reuse) a role password and persist the datasource line into the
# service env file.
ensure_datasource() {
  local svc="$1" env_file="$2" role
  role=$(role_name "$svc")
  local existing
  existing=$(grep '^POSTGRES_DATASOURCE=' "$env_file" 2>/dev/null | head -1 | cut -d= -f2- || true)
  if [ -n "$existing" ] && grep -q "user=${role} " <<<"$existing"; then
    return 0  # already configured for this role — keep password
  fi
  local pass
  pass=$(openssl rand -hex 24)
  if grep -q '^POSTGRES_DATASOURCE=' "$env_file" 2>/dev/null; then
    # Replace the stale line (e.g. copied over with the superuser).
    grep -v '^POSTGRES_DATASOURCE=' "$env_file" > "$env_file.tmp"
    mv "$env_file.tmp" "$env_file"
  fi
  echo "POSTGRES_DATASOURCE=host=postgres port=5432 user=${role} dbname=${PG_DB} password=${pass} sslmode=disable" >> "$env_file"
}

# Extract the password for a service's role from its env file (single source of
# truth — the file was just written/normalized by ensure_datasource).
pass_for() {
  sed -n 's/.*password=\([^ ]*\).*/\1/p' "$(env_file_for "$1")" | head -1
}

for svc in "${ROLES[@]}"; do
  env_file="$(env_file_for "$svc")"
  touch "$env_file"
  ensure_datasource "$svc" "$env_file"
done

# Build the SQL: create/rotate roles, then grant per ownership map.
sql="$(mktemp)"
trap 'rm -f "$sql"' EXIT

for svc in "${ROLES[@]}"; do
  role="$(role_name "$svc")"
  pass="$(pass_for "$svc")"
  : "${pass:?no password for ${role} in $(env_file_for "$svc")}"
  cat >>"$sql" <<SQL
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${role}') THEN
    CREATE ROLE ${role} LOGIN PASSWORD '${pass}';
  ELSE
    ALTER ROLE ${role} WITH LOGIN PASSWORD '${pass}';
  END IF;
END \$\$;
REVOKE ALL ON DATABASE ${PG_DB} FROM ${role};
GRANT CONNECT ON DATABASE ${PG_DB} TO ${role};
GRANT USAGE ON SCHEMA public TO ${role};
SQL
  for entry in $(tables_for "$svc"); do
    if [[ "$entry" == RO:* ]]; then
      tbl="${entry#RO:}"; privs="SELECT"
    else
      tbl="$entry"; privs="SELECT, INSERT, UPDATE, DELETE"
    fi
    # Table may not exist yet (fresh deploy before migrations) — guard so the
    # script can run against any schema state.
    cat >>"$sql" <<SQL
DO \$\$ BEGIN
  IF to_regclass('public.${tbl}') IS NOT NULL THEN
    EXECUTE 'GRANT ${privs} ON ${tbl} TO ${role}';
  END IF;
END \$\$;
SQL
  done
  # Sequences (serial columns) — no data exposure, needed for INSERTs.
  cat >>"$sql" <<SQL
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO ${role};
SQL
done

psql < "$sql"

# Warn about tables nobody was granted — either a mapping is missing here or a
# migration added a table without updating this script + check-table-ownership.sh.
unmapped=$(psql -tA </dev/null -c "
  SELECT tablename FROM pg_tables
  WHERE schemaname = 'public'
    AND tablename NOT LIKE 'schema_migrations%'
  EXCEPT
  SELECT DISTINCT unnest(ARRAY[
$(for svc in "${ROLES[@]}"; do
    for entry in $(tables_for "$svc"); do
      [[ "$entry" == RO:* ]] && t="${entry#RO:}" || t="$entry"
      printf "    '%s',\n" "$t"
    done
  done | sort -u | sed '$ s/,$//')
  ]::text[]) ORDER BY 1;")
if [ -n "$unmapped" ]; then
  echo "!! WARNING: tables with no role grants (update $0 + check-table-ownership.sh):" >&2
  echo "$unmapped" | sed 's/^/!!   /' >&2
fi

echo "==> per-service roles applied; env datasources written to deploy/.env.<svc>"
