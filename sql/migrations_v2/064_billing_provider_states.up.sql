-- Billing correctness: per-provider subscription state + server-recorded
-- Paddle checkouts.
--
-- Until now both billing providers (Paddle web, RevenueCat mobile) wrote the
-- SAME shared columns on `subscriptions` (status, plan_id, period, cancel
-- flag). A cancel/expiry from one provider could clobber the other's active
-- subscription, and out-of-order webhook deliveries silently won over newer
-- state (no occurred_at watermark anywhere).
--
-- subscription_provider_states keeps one row per (user, provider) carrying
-- that provider's last-applied state plus the provider-side event watermark
-- (last_event_at / last_event_id). The shared `subscriptions` row becomes a
-- merged projection recomputed by the client service after every provider
-- state change: any provider state that grants access wins; non-active
-- states can never overwrite an active state from another provider.
--
-- paddle_checkouts records which transactions the backend created for which
-- user. Paddle checkouts can also be opened client-side where custom_data is
-- attacker-controlled, so the webhook only trusts custom_data.user_id when
-- the transaction was created server-side for that same user.

CREATE TABLE subscription_provider_states (
    user_id                  uuid         NOT NULL,
    provider                 text         NOT NULL CHECK (provider IN ('paddle', 'revenuecat')),
    status                   text         NOT NULL,
    billing_interval         text         CHECK (billing_interval IN ('monthly', 'annual')),
    current_period_start     timestamptz,
    current_period_end       timestamptz,
    trial_end                timestamptz,
    cancel_at_period_end     boolean      NOT NULL DEFAULT false,
    provider_customer_id     varchar(255),
    provider_subscription_id varchar(255),
    last_event_at            timestamptz,
    last_event_id            text,
    created_at               timestamptz  NOT NULL DEFAULT now(),
    updated_at               timestamptz  NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, provider)
);

-- webhook resolution by provider customer id (Paddle ctm_..., RevenueCat
-- app_user_id). Same user may be reachable by both providers.
CREATE INDEX idx_sub_provider_states_customer
    ON subscription_provider_states (provider, provider_customer_id)
    WHERE provider_customer_id IS NOT NULL;
CREATE INDEX idx_sub_provider_states_subscription
    ON subscription_provider_states (provider_subscription_id)
    WHERE provider_subscription_id IS NOT NULL;

CREATE TRIGGER subscription_provider_states_set_updated_at
    BEFORE UPDATE ON subscription_provider_states
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Transactions created via CreatePaddleCheckout (server-side). The webhook
-- treats custom_data.user_id as authoritative only for txns recorded here.
CREATE TABLE paddle_checkouts (
    transaction_id varchar(255) PRIMARY KEY,
    user_id        uuid         NOT NULL,
    created_at     timestamptz  NOT NULL DEFAULT now()
);

CREATE INDEX idx_paddle_checkouts_user ON paddle_checkouts (user_id);

-- Backfill: existing paid rows already carry whichever provider wrote last.
-- Attribute the shared columns to each provider whose link columns are set so
-- current subscribers keep their entitlement; the next webhook per provider
-- converges each state to that provider's truth.
INSERT INTO subscription_provider_states (
    user_id, provider, status, billing_interval,
    current_period_start, current_period_end, trial_end,
    cancel_at_period_end, provider_customer_id, provider_subscription_id
)
SELECT user_id, 'paddle', status, billing_interval,
       current_period_start, current_period_end, trial_end,
       cancel_at_period_end, paddle_customer_id, paddle_subscription_id
FROM subscriptions
WHERE paddle_customer_id IS NOT NULL OR paddle_subscription_id IS NOT NULL
ON CONFLICT (user_id, provider) DO NOTHING;

INSERT INTO subscription_provider_states (
    user_id, provider, status, billing_interval,
    current_period_start, current_period_end, trial_end,
    cancel_at_period_end, provider_customer_id, provider_subscription_id
)
SELECT user_id, 'revenuecat', status, billing_interval,
       current_period_start, current_period_end, trial_end,
       cancel_at_period_end, revenuecat_customer_id, NULL
FROM subscriptions
WHERE revenuecat_customer_id IS NOT NULL
ON CONFLICT (user_id, provider) DO NOTHING;

-- Refund/chargeback and restore bookkeeping on the audit trail.
ALTER TABLE upgrade_events
    DROP CONSTRAINT upgrade_events_event_type_check;
ALTER TABLE upgrade_events
    ADD CONSTRAINT upgrade_events_event_type_check
    CHECK (event_type IN (
        'prompt_viewed', 'prompt_clicked', 'prompt_dismissed',
        'checkout_started', 'checkout_completed', 'checkout_canceled',
        'subscription_started', 'subscription_canceled',
        'subscription_expired',
        'subscription_refunded', 'subscription_chargeback',
        'subscription_restored'));
