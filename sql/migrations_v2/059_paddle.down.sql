ALTER TABLE subscriptions
    DROP CONSTRAINT subscriptions_status_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_status_check
    CHECK (status IN (
        'free', 'trialing', 'active', 'past_due', 'canceled', 'expired'));

DROP INDEX IF EXISTS idx_subscriptions_paddle_subscription;
DROP INDEX IF EXISTS idx_subscriptions_paddle_customer;

ALTER TABLE subscriptions
    DROP COLUMN IF EXISTS paddle_subscription_id,
    DROP COLUMN IF EXISTS paddle_customer_id;
