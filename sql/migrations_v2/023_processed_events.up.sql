-- Single idempotency table for every event consumer.
-- consumer examples: 'ai_coach', 'notifications', 'stripe_webhooks'.
-- event_id is text so it fits Kafka event UUIDs and Stripe ids (evt_*) alike.
CREATE TABLE processed_events (
    consumer     text NOT NULL,
    event_id     text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (consumer, event_id)
);

-- retention sweeps (delete rows older than N days)
CREATE INDEX idx_processed_events_processed_at ON processed_events (processed_at);
