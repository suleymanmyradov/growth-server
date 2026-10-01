-- Transactional event outbox (P1). Rows are written inside the domain
-- transaction and drained by the outbox relay (pkg/events/outbox), which
-- republishes the stored payload with the stored event ID until the broker
-- accepts it. Claims are leased via next_attempt_at so a crashed relay's
-- rows become claimable again.

-- name: EnqueueClientEvent :exec
INSERT INTO client_event_outbox (event_id, event_type, payload, occurred_at)
VALUES ($1, $2, $3, $4);

-- name: ClaimClientEvent :one
UPDATE client_event_outbox
SET next_attempt_at = now() + interval '60 seconds',
    attempts = attempts + 1
WHERE event_id = (
    SELECT event_id FROM client_event_outbox
    WHERE next_attempt_at <= now()
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING event_id, event_type, payload, occurred_at;

-- name: CompleteClientEvent :exec
DELETE FROM client_event_outbox WHERE event_id = $1;
