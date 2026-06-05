-- Remove seeded billing plans.
-- Note: this will fail if user_subscriptions still reference these plans;
-- roll back dependent subscription data first if required.
DELETE FROM public.plans WHERE code IN ('free', 'pro');
