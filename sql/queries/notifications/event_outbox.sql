-- Transactional event outbox for the notifications service (P1). See
-- sql/queries/client/event_outbox.sql for the pattern.

-- name: EnqueueNotificationEvent :exec
INSERT INTO notification_event_outbox (event_id, event_type, payload, occurred_at)
VALUES ($1, $2, $3, $4);

-- name: ClaimNotificationEvent :one
UPDATE notification_event_outbox
SET next_attempt_at = now() + interval '60 seconds',
    attempts = attempts + 1
WHERE event_id = (
    SELECT event_id FROM notification_event_outbox
    WHERE next_attempt_at <= now()
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING event_id, event_type, payload, occurred_at;

-- name: CompleteNotificationEvent :exec
DELETE FROM notification_event_outbox WHERE event_id = $1;
