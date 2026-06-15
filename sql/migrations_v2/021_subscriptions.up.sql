-- One subscription row per user.
CREATE TABLE subscriptions (
    id                     uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id                uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    plan_id                uuid NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
    status                 text NOT NULL DEFAULT 'free' CHECK (status IN (
                               'free', 'trialing', 'active', 'past_due', 'canceled', 'expired')),
    billing_interval       text CHECK (billing_interval IN ('monthly', 'annual')),
    current_period_start   timestamptz,
    current_period_end     timestamptz,
    trial_end              timestamptz,
    cancel_at_period_end   boolean NOT NULL DEFAULT false,
    stripe_customer_id     varchar(255),
    stripe_subscription_id varchar(255),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),

    -- paid statuses must carry billing period data
    CONSTRAINT subscriptions_active_has_period CHECK (
        status NOT IN ('active', 'trialing')
        OR (billing_interval IS NOT NULL
            AND current_period_start IS NOT NULL
            AND current_period_end IS NOT NULL)
    )
);

-- Stripe webhook lookups
CREATE INDEX idx_subscriptions_stripe_customer ON subscriptions (stripe_customer_id);
CREATE INDEX idx_subscriptions_stripe_subscription ON subscriptions (stripe_subscription_id);
-- reconciliation of expired subscriptions
CREATE INDEX idx_subscriptions_period_end ON subscriptions (current_period_end)
    WHERE status IN ('active', 'trialing');

CREATE TRIGGER subscriptions_set_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
