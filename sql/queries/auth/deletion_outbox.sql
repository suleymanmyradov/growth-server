-- name: ClaimAuthDeletion :one
UPDATE auth_deletion_outbox
SET attempts = attempts + 1, next_attempt_at = now() + interval '1 minute'
WHERE event_id = (
    SELECT event_id FROM auth_deletion_outbox
    WHERE next_attempt_at <= now()
    ORDER BY created_at, event_id
    LIMIT 1 FOR UPDATE SKIP LOCKED
)
RETURNING event_id, user_id, created_at;

-- name: CompleteAuthDeletion :exec
DELETE FROM auth_deletion_outbox WHERE event_id = $1;
