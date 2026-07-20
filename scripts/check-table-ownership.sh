#!/usr/bin/env bash
set -euo pipefail

# CI ownership check: verifies that each service's SQL queries only reference
# tables owned by that service. Fails on any cross-service table reference.
#
# Usage: bash scripts/check-table-ownership.sh
# Make:  make check-ownership

cd "$(dirname "$0")/.."

# ---------------------------------------------------------------------------
# Get allowed tables for a service (bash 3.x compatible)
# ---------------------------------------------------------------------------
# ---------------------------------------------------------------------------
# Get allowed tables for a service (bash 3.x compatible)
#
# Notes on ownership:
#   - ai_feedback is owned by ai-coach-consumer (hand-written SQL, no query dir
#     to scan). It is NOT in client's list — client never touches it.
#   - subscriptions is co-owned by client and billing-reconciler. Both write
#     via ON CONFLICT (user_id) DO UPDATE: client creates the default free
#     subscription on signup and upserts on upgrade; billing-reconciler
#     backfills/syncs from Stripe webhooks. This is intentional.
#   - billing-reconciler has no sql/queries/ dir (hand-written SQL in Go), so
#     it is not scanned by this script.
#   - ai-coach-consumer likewise has no sql/queries/ dir.
# ---------------------------------------------------------------------------
get_allowed_tables() {
    case "$1" in
        auth)          echo "users user_oauth_accounts" ;;
        client)        echo "user_preferences coaching_profiles categories articles article_likes article_shares article_tags tags saved_articles saved_goals saved_habits goals habits goal_habits check_ins activities weekly_reviews plan_adjustments plans subscriptions upgrade_events user_profiles reports report_comments" ;;
        notifications) echo "notifications reminders notification_preferences reminder_state processed_events" ;;
        adminway)      echo "internal_users" ;;
        conversations) echo "conversations conversation_messages" ;;
        *)             echo "" ;;
    esac
}

# ---------------------------------------------------------------------------
# Extract referenced table names from a SQL file.
# Strips comments, then matches table names after SQL keywords.
# ---------------------------------------------------------------------------
extract_tables() {
    local file="$1"
    # Strip line comments (-- ...) and block comments (/* ... */)
    sed 's/--.*$//' "$file" \
        | tr '\n' ' ' \
        | sed 's|/\*[^*]*\*/||g' \
        | sed 's/ON CONFLICT.*DO UPDATE/ON_CONFLICT_DO_UPDATE/g' \
        | grep -oiE '(FROM|JOIN|INTO|UPDATE)[[:space:]]+[a-z_]+' \
        | sed -E 's/.*[[:space:]]+([a-z_]+)$/\1/' \
        | tr '[:upper:]' '[:lower:]' \
        | sort -u
}

# ---------------------------------------------------------------------------
# Check each service's queries
# ---------------------------------------------------------------------------
violations=0

for service in auth client notifications adminway conversations; do
    query_dir="sql/queries/$service"
    [ -d "$query_dir" ] || continue
    allowed=$(get_allowed_tables "$service")

    for sql_file in "$query_dir"/*.sql; do
        [ -f "$sql_file" ] || continue

        referenced_tables=$(extract_tables "$sql_file")

        for table in $referenced_tables; do
            # Skip CTE names, aliases, SQL keywords, and common false positives
            case "$table" in
                # SQL keywords / syntax
                set|update|select|insert|values|where|with|returning|delete|into|from|join|left|right|inner|outer|on|and|or|not|null|exists|distinct|order|group|having|limit|offset|as|case|when|then|else|end|count|coalesce|round|filter|max|min|sum|avg|row_number|unnest|now|date|numeric|text|varchar|uuid|bool|true|false|skip|locked) continue ;;
                # CTE names used in the codebase
                subquery|cte|tmp|temp|new_sort|ins|upd|user_tz|today|completed|islands|last_dates|last_date|bounds|days|numbered|groups|streaks) continue ;;
                # Single-letter aliases (e.g., "FROM habits h", "FROM user_preferences s")
                a|b|c|d|e|f|g|h|i|j|k|l|m|n|o|p|q|r|s|t|u|v|w|x|y|z) continue ;;
                # Common non-table words from SQL
                due|each|previous|the) continue ;;
            esac
            if ! echo "$allowed" | grep -qw "$table"; then
                echo "VIOLATION: $sql_file references table '$table' not owned by service '$service'"
                violations=$((violations + 1))
            fi
        done
    done
done

if [ "$violations" -gt 0 ]; then
    echo ""
    echo "Found $violations ownership violation(s)."
    exit 1
fi

echo "All table ownership checks passed."
