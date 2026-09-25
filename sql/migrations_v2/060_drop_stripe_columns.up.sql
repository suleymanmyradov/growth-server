-- Stripe was removed as a billing provider in favor of Paddle
-- (pkg/paddle + Paddle.js checkout; mobile stays on RevenueCat).
-- Drop the Stripe linkage columns and their lookup indexes.
-- Subscription state now syncs via the paddle_*/revenuecat_* columns only.
DROP INDEX IF EXISTS idx_subscriptions_stripe_subscription;
DROP INDEX IF EXISTS idx_subscriptions_stripe_customer;

ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS stripe_subscription_id,
    DROP COLUMN IF EXISTS stripe_customer_id;

ALTER TABLE plans
    DROP COLUMN IF EXISTS stripe_annual_price_id,
    DROP COLUMN IF EXISTS stripe_monthly_price_id;
