#!/usr/bin/env bash
# retention-cleanup.sh — enforce the data-retention policy (#22) by deleting
# expired rows from Postgres. Runs nightly via growth-retention.timer on the
# VM; safe to run by hand at any time (all deletes are idempotent).
#
# Policy (days, env-overridable):
#   AI transcripts   conversation_messages.created_at / conversations.updated_at   90
#   AI feedback      ai_feedback.created_at                                        90
#   Activity log     activities.created_at                                        365
#   Notifications    notifications.created_at (deliveries cascade)                 90
#   Push receipts    push_tickets.created_at                                       30
#   Idempotency      *_processed_events.processed_at + billing_webhook_events      90
#   Search outbox    search_outbox.created_at (undrained rows are stuck anyway)    30
#   Reminder dedup   reminder_state.updated_at / reminders (sent only)             90
#
# Kept indefinitely: user content (habits, goals, check-ins, plans,
# weekly_reviews, saved_*, user_facts, coaching_profiles, reports), catalog
# data (articles, categories, templates), subscriptions (billing record),
# aggregate analytics (daily_metrics, *_rollups, cohorts — no raw PII).
# Sessions/refresh tokens live in Redis with TTLs — nothing to clean.
# Account deletion is already a hard delete (auth.users + user_deleted event
# cascade) — no grace-period cleanup needed.
#
# Install on the VM:
#   install -m 755 deploy/scripts/retention-cleanup.sh /home/ubuntu/retention-cleanup.sh
#   install -m 644 deploy/systemd/growth-retention.{service,timer} /etc/systemd/system/
#   systemctl daemon-reload && systemctl enable --now growth-retention.timer
#
# Test without deleting:  DRY_RUN=true /home/ubuntu/retention-cleanup.sh

set -euo pipefail

ENV_FILE="${ENV_FILE:-/home/ubuntu/growth-server/deploy/.env.prod}"
DRY_RUN="${DRY_RUN:-false}"

CONVERSATION_DAYS="${CONVERSATION_DAYS:-90}"
AI_FEEDBACK_DAYS="${AI_FEEDBACK_DAYS:-90}"
ACTIVITY_DAYS="${ACTIVITY_DAYS:-365}"
NOTIFICATION_DAYS="${NOTIFICATION_DAYS:-90}"
PUSH_TICKET_DAYS="${PUSH_TICKET_DAYS:-30}"
IDEMPOTENCY_DAYS="${IDEMPOTENCY_DAYS:-90}"
SEARCH_OUTBOX_DAYS="${SEARCH_OUTBOX_DAYS:-30}"
REMINDER_DAYS="${REMINDER_DAYS:-90}"

ts() { date -Is; }

# Build DATABASE_URL from .env.prod with grep — never `source` it: the
# unquoted PEM values in there contain spaces and break shell sourcing.
pgvar() { grep -E "^$1=" "$ENV_FILE" | head -1 | cut -d= -f2-; }
PGUSER="$(pgvar POSTGRES_USER)"
PGPASS="$(pgvar POSTGRES_PASSWORD)"
PGDB="$(pgvar POSTGRES_DB)"
export PGPASSWORD="$PGPASS"

PSQL="docker exec deploy-postgres-1 psql -U $PGUSER -d $PGDB -tA"

# run <table> <label> <sql-delete> <sql-count>
# Skips missing tables (dev/prod schema drift) instead of erroring.
# In DRY_RUN mode runs the count query instead of the delete.
run() {
  local table="$1" label="$2" delete_sql="$3" count_sql="$4"
  if [ "$($PSQL -c "SELECT to_regclass('$table') IS NOT NULL")" != "t" ]; then
    echo "$(ts) $label: table $table absent — skipped"
    return 0
  fi
  if [ "$DRY_RUN" = "true" ]; then
    local n
    n=$($PSQL -c "$count_sql")
    echo "$(ts) [dry-run] $label: $n rows would be deleted"
  else
    local n
    n=$($PSQL -c "$delete_sql")
    echo "$(ts) $label: deleted $n rows"
  fi
}

echo "$(ts) retention-cleanup starting (dry_run=$DRY_RUN)"

# AI transcripts — messages first (cheap, indexed), then stale conversation
# shells; ON DELETE CASCADE cleans any remaining messages of deleted
# conversations and SET NULL keeps user_facts.source_message_id intact.
run conversation_messages "conversation_messages >${CONVERSATION_DAYS}d" \
  "WITH d AS (DELETE FROM conversation_messages WHERE created_at < now() - interval '$CONVERSATION_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM conversation_messages WHERE created_at < now() - interval '$CONVERSATION_DAYS days'"

run conversations "conversations >${CONVERSATION_DAYS}d" \
  "WITH d AS (DELETE FROM conversations WHERE updated_at < now() - interval '$CONVERSATION_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM conversations WHERE updated_at < now() - interval '$CONVERSATION_DAYS days'"

run ai_feedback "ai_feedback >${AI_FEEDBACK_DAYS}d" \
  "WITH d AS (DELETE FROM ai_feedback WHERE created_at < now() - interval '$AI_FEEDBACK_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM ai_feedback WHERE created_at < now() - interval '$AI_FEEDBACK_DAYS days'"

run activities "activities >${ACTIVITY_DAYS}d" \
  "WITH d AS (DELETE FROM activities WHERE created_at < now() - interval '$ACTIVITY_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM activities WHERE created_at < now() - interval '$ACTIVITY_DAYS days'"

# notification_deliveries + notification_recipients cascade off notifications.
run notifications "notifications >${NOTIFICATION_DAYS}d" \
  "WITH d AS (DELETE FROM notifications WHERE created_at < now() - interval '$NOTIFICATION_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM notifications WHERE created_at < now() - interval '$NOTIFICATION_DAYS days'"

run push_tickets "push_tickets >${PUSH_TICKET_DAYS}d" \
  "WITH d AS (DELETE FROM push_tickets WHERE created_at < now() - interval '$PUSH_TICKET_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM push_tickets WHERE created_at < now() - interval '$PUSH_TICKET_DAYS days'"

# Idempotency tables — retention must exceed any provider retry window
# (Stripe/Paddle retry for ~3 days; 90d is far past it).
for t in processed_events client_processed_events analytics_processed_events billing_webhook_events; do
  run "$t" "$t >${IDEMPOTENCY_DAYS}d" \
    "WITH d AS (DELETE FROM $t WHERE processed_at < now() - interval '$IDEMPOTENCY_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
    "SELECT count(*) FROM $t WHERE processed_at < now() - interval '$IDEMPOTENCY_DAYS days'"
done

run search_outbox "search_outbox >${SEARCH_OUTBOX_DAYS}d" \
  "WITH d AS (DELETE FROM search_outbox WHERE created_at < now() - interval '$SEARCH_OUTBOX_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM search_outbox WHERE created_at < now() - interval '$SEARCH_OUTBOX_DAYS days'"

# reminder_state is per-user dedup state — expiring stale rows is safe.
run reminder_state "reminder_state >${REMINDER_DAYS}d" \
  "WITH d AS (DELETE FROM reminder_state WHERE updated_at < now() - interval '$REMINDER_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM reminder_state WHERE updated_at < now() - interval '$REMINDER_DAYS days'"

# Sent reminders only — never touch scheduled-but-unsent rows.
run reminders "reminders sent >${REMINDER_DAYS}d" \
  "WITH d AS (DELETE FROM reminders WHERE sent_at IS NOT NULL AND sent_at < now() - interval '$REMINDER_DAYS days' RETURNING 1) SELECT count(*) FROM d" \
  "SELECT count(*) FROM reminders WHERE sent_at IS NOT NULL AND sent_at < now() - interval '$REMINDER_DAYS days'"

echo "$(ts) retention-cleanup done"
