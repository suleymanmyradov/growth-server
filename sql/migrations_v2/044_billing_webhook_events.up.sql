-- Billing-owned webhook event idempotency tables.
--
-- The client service's billing domain processes Stripe and RevenueCat webhooks
-- and needs idempotent event handling. Previously these used the shared
-- processed_events table (notifications-owned), which violated table ownership.
-- These tables are owned exclusively by the client service's billing domain so
-- the ownership checker stays green without fake co-ownership.
--
-- billing_webhook_events: dedup for Stripe + RevenueCat webhook deliveries.
--   consumer values: 'stripe_webhooks', 'revenuecat_webhooks'.
-- client_processed_events: dedup for the client service's own Kafka consumer.

CREATE TABLE billing_webhook_events (
    consumer     text NOT NULL,
    event_id     text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (consumer, event_id)
);

CREATE INDEX idx_billing_webhook_events_processed_at
    ON billing_webhook_events (processed_at);

CREATE TABLE client_processed_events (
    consumer     text NOT NULL,
    event_id     text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (consumer, event_id)
);

CREATE INDEX idx_client_processed_events_processed_at
    ON client_processed_events (processed_at);
