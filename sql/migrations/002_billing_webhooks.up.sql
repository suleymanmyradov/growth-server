-- processed_stripe_events: idempotency guard for Stripe webhook events.
-- Stripe event IDs are strings (evt_*), so we use a VARCHAR primary key.
CREATE TABLE public.processed_stripe_events (
    stripe_event_id character varying(255) NOT NULL PRIMARY KEY,
    event_type character varying(100) NOT NULL,
    processed_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_processed_stripe_events_processed_at ON public.processed_stripe_events USING btree (processed_at);

-- Add currency to plans for future multi-currency support.
ALTER TABLE public.plans ADD COLUMN currency character varying(3) DEFAULT 'usd' NOT NULL;

-- Index for efficient reconciliation of expired subscriptions.
CREATE INDEX idx_user_subscriptions_period_end ON public.user_subscriptions USING btree (current_period_end)
WHERE status IN ('active', 'trialing');
