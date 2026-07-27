-- Event dedup for the client service's Kafka consumer.
-- Uses the client-owned client_processed_events table (not the notifications-
-- owned processed_events table) to respect table ownership boundaries.

-- name: IsClientEventProcessed :one
SELECT EXISTS(SELECT 1 FROM client_processed_events WHERE consumer = 'client' AND event_id = $1);

-- name: MarkClientEventProcessed :exec
INSERT INTO client_processed_events (consumer, event_id)
VALUES ('client', $1)
ON CONFLICT DO NOTHING;
