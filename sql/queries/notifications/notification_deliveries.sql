-- name: CreateNotificationDelivery :one
INSERT INTO notification_deliveries (notification_id, user_id, channel)
VALUES ($1, $2, $3)
ON CONFLICT (notification_id, channel) DO UPDATE
SET notification_id = EXCLUDED.notification_id
RETURNING *;

-- name: ClaimNotificationDeliveries :many
WITH due AS (
    SELECT id
    FROM notification_deliveries
    WHERE status = 'pending'
      AND next_attempt_at <= now()
      AND claimed_at IS NULL
    ORDER BY next_attempt_at, created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE notification_deliveries d
SET status = 'processing', claimed_at = now(), attempt_count = attempt_count + 1
FROM due
WHERE d.id = due.id
RETURNING d.*;

-- name: ReleaseStaleNotificationDeliveries :execrows
UPDATE notification_deliveries
SET status = 'pending', claimed_at = NULL
WHERE status = 'processing'
  AND claimed_at < now() - ($1::int * interval '1 minute');

-- name: MarkNotificationDeliverySent :exec
UPDATE notification_deliveries
SET status = 'sent', sent_at = now(), claimed_at = NULL,
    provider_message_id = $2, last_error_code = NULL, last_error_message = NULL
WHERE id = $1;

-- name: MarkNotificationDeliverySuppressed :exec
UPDATE notification_deliveries
SET status = 'suppressed', claimed_at = NULL,
    last_error_code = $2, last_error_message = $3
WHERE id = $1;

-- name: MarkNotificationDeliveryFailed :exec
UPDATE notification_deliveries
SET status = 'failed', claimed_at = NULL,
    last_error_code = $2, last_error_message = $3
WHERE id = $1;

-- name: RetryNotificationDelivery :exec
UPDATE notification_deliveries
SET status = 'pending', claimed_at = NULL, next_attempt_at = $2,
    last_error_code = $3, last_error_message = $4
WHERE id = $1;
