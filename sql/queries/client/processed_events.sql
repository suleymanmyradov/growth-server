-- Event dedup for the client service's Kafka consumer.

-- name: IsClientEventProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_events WHERE consumer = 'client' AND event_id = $1);

-- name: MarkClientEventProcessed :exec
INSERT INTO processed_events (consumer, event_id)
VALUES ('client', $1)
ON CONFLICT DO NOTHING;
