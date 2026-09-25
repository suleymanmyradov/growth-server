-- Restore the Stripe columns dropped by 060_drop_stripe_columns.up.sql.
-- Column definitions match migrations 020 (plans) and 021 (subscriptions).
-- Note: dropped values are not recoverable by this migration.
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS stripe_monthly_price_id varchar(255),
    ADD COLUMN IF NOT EXISTS stripe_annual_price_id  varchar(255);

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS stripe_customer_id     varchar(255),
    ADD COLUMN IF NOT EXISTS stripe_subscription_id varchar(255);

CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_customer
    ON subscriptions (stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_subscription
    ON subscriptions (stripe_subscription_id);
