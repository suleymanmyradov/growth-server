-- Transactional event outbox for auth (P1). Distinct from
-- auth_deletion_outbox, which carries only user_deleted rows.

-- name: EnqueueAuthEvent :exec
INSERT INTO auth_event_outbox (event_id, event_type, payload, occurred_at)
VALUES ($1, $2, $3, $4);

-- name: ClaimAuthEvent :one
UPDATE auth_event_outbox
SET next_attempt_at = now() + interval '60 seconds',
    attempts = attempts + 1
WHERE event_id = (
    SELECT event_id FROM auth_event_outbox
    WHERE next_attempt_at <= now()
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING event_id, event_type, payload, occurred_at;

-- name: CompleteAuthEvent :exec
DELETE FROM auth_event_outbox WHERE event_id = $1;
