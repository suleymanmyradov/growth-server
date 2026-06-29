-- Reminders: sent_at IS NULL means pending.

-- name: EnqueueReminder :one
INSERT INTO reminders (user_id, type, scheduled_at, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, type, ((scheduled_at AT TIME ZONE 'UTC')::date)) WHERE sent_at IS NULL
DO UPDATE SET scheduled_at = EXCLUDED.scheduled_at,
              metadata = EXCLUDED.metadata
RETURNING id, user_id, type, scheduled_at, sent_at, metadata, created_at;

-- name: CancelPendingReminderForDate :exec
DELETE FROM reminders
WHERE user_id = $1
  AND type = $2
  AND sent_at IS NULL
  AND (scheduled_at AT TIME ZONE $4::text)::date = $3::date;

-- name: ClaimDueReminders :many
WITH due AS (
    SELECT id FROM reminders
    WHERE sent_at IS NULL AND scheduled_at <= now()
    ORDER BY scheduled_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE reminders r SET sent_at = now()
FROM due
WHERE r.id = due.id
RETURNING r.id, r.user_id, r.type, r.scheduled_at, r.sent_at, r.metadata, r.created_at;

-- name: GetPendingByUser :many
SELECT id, user_id, type, scheduled_at, sent_at, metadata, created_at
FROM reminders
WHERE user_id = $1 AND sent_at IS NULL
ORDER BY scheduled_at;
