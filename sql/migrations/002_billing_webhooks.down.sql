DROP INDEX IF EXISTS idx_user_subscriptions_period_end;
ALTER TABLE public.plans DROP COLUMN IF EXISTS currency;
DROP INDEX IF EXISTS idx_processed_stripe_events_processed_at;
DROP TABLE IF EXISTS public.processed_stripe_events;
