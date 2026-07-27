DROP INDEX IF EXISTS idx_subscriptions_revenuecat_customer;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS revenuecat_customer_id;
