-- Append-only funnel analytics for upgrade prompts and checkout.
CREATE TABLE upgrade_events (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id          uuid REFERENCES plans(id) ON DELETE SET NULL,
    event_type       text NOT NULL CHECK (event_type IN (
                         'prompt_viewed', 'prompt_clicked', 'prompt_dismissed',
                         'checkout_started', 'checkout_completed', 'checkout_canceled',
                         'subscription_started', 'subscription_canceled')),
    surface          varchar(50) NOT NULL,
    trigger_source   varchar(50),
    billing_interval text CHECK (billing_interval IN ('monthly', 'annual')),
    feedback_reason  varchar(100),
    feedback_note    text,
    metadata         jsonb NOT NULL DEFAULT '{}',
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_upgrade_events_user ON upgrade_events (user_id, created_at DESC);
-- funnel reporting
CREATE INDEX idx_upgrade_events_type ON upgrade_events (event_type, created_at DESC);
