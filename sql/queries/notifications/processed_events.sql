-- name: IsEventProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_events WHERE consumer = 'notifications' AND event_id = $1);

-- name: MarkEventProcessed :exec
INSERT INTO processed_events (consumer, event_id)
VALUES ('notifications', $1)
ON CONFLICT DO NOTHING;
