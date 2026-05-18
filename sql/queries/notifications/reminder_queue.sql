-- name: EnqueueReminder :one
INSERT INTO reminder_queue (user_id, type, scheduled_at, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, type, (scheduled_at::date)) WHERE sent = FALSE
DO UPDATE SET scheduled_at = EXCLUDED.scheduled_at,
              metadata = EXCLUDED.metadata,
              updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: CancelPendingReminderForDate :exec
DELETE FROM reminder_queue
WHERE user_id = $1
  AND type = $2
  AND sent = FALSE
  AND (scheduled_at AT TIME ZONE $4::text)::date = $3::date;

-- name: ClaimDueReminders :many
WITH due AS (
    SELECT id FROM reminder_queue
    WHERE sent = FALSE AND scheduled_at <= CURRENT_TIMESTAMP
    ORDER BY scheduled_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE reminder_queue r SET sent = TRUE, sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
FROM due
WHERE r.id = due.id
RETURNING r.*;

-- name: GetPendingByUser :many
SELECT * FROM reminder_queue
WHERE user_id = $1 AND sent = FALSE
ORDER BY scheduled_at;
