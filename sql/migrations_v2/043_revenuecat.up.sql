-- RevenueCat integration: add revenuecat_customer_id to subscriptions and
-- create a revenuecat_webhook_events table for webhook event idempotency.
--
-- RevenueCat (https://www.revenuecat.com) abstracts App Store, Play Store, and
-- Amazon Store subscriptions behind a single REST API and webhook. The mobile
-- app calls Purchases.logIn(userId) to associate the RevenueCat customer with
-- our backend user ID; RevenueCat then sends webhooks on entitlement changes.
--
-- We reuse the existing `processed_events` table for idempotency (consumer =
-- 'revenuecat_webhooks'), so no new idempotency table is needed.
--
-- The `revenuecat_customer_id` column mirrors `stripe_customer_id` — it is the
-- RevenueCat customer ID (usually equal to our user UUID after logIn). It is
-- nullable because not all users use mobile subscriptions (some use Stripe
-- web checkout).

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS revenuecat_customer_id varchar(255);

CREATE INDEX IF NOT EXISTS idx_subscriptions_revenuecat_customer
    ON subscriptions (revenuecat_customer_id)
    WHERE revenuecat_customer_id IS NOT NULL;
