-- Paddle integration: add paddle_customer_id / paddle_subscription_id to
-- subscriptions and allow the 'paused' subscription status.
--
-- Paddle Billing (https://www.paddle.com) handles web checkout subscriptions.
-- The client service's billing domain verifies Paddle-Signature webhook
-- deliveries and mirrors subscription state into this table, mirroring the
-- Stripe columns. We reuse the billing_webhook_events table for idempotency
-- (consumer = 'paddle_webhooks').
--
-- paddle_customer_id mirrors stripe_customer_id: it links the Paddle customer
-- (ctm_...) to the user's subscription row once a checkout completes.
-- paddle_subscription_id holds the Paddle subscription (sub_...) so webhook
-- updates can be matched and stale events for superseded subscriptions can
-- be ignored.
--
-- 'paused' is added to the status CHECK because Paddle subscriptions can be
-- paused (no billing, no service, resumable) — distinct from canceled.

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS paddle_customer_id varchar(255),
    ADD COLUMN IF NOT EXISTS paddle_subscription_id varchar(255);

CREATE INDEX IF NOT EXISTS idx_subscriptions_paddle_customer
    ON subscriptions (paddle_customer_id)
    WHERE paddle_customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_subscriptions_paddle_subscription
    ON subscriptions (paddle_subscription_id)
    WHERE paddle_subscription_id IS NOT NULL;

ALTER TABLE subscriptions
    DROP CONSTRAINT subscriptions_status_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_status_check
    CHECK (status IN (
        'free', 'trialing', 'active', 'past_due', 'paused', 'canceled', 'expired'));
