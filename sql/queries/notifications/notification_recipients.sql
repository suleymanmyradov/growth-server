-- name: UpsertNotificationRecipient :one
INSERT INTO notification_recipients (user_id, email, name, email_verified)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE SET
    email = EXCLUDED.email,
    name = EXCLUDED.name,
    email_verified = EXCLUDED.email_verified,
    updated_at = now()
RETURNING *;

-- name: GetNotificationRecipient :one
SELECT * FROM notification_recipients WHERE user_id = $1;

-- name: DeleteNotificationRecipient :exec
DELETE FROM notification_recipients WHERE user_id = $1;
