ALTER TABLE upgrade_events
    DROP CONSTRAINT upgrade_events_event_type_check;
ALTER TABLE upgrade_events
    ADD CONSTRAINT upgrade_events_event_type_check
    CHECK (event_type IN (
        'prompt_viewed', 'prompt_clicked', 'prompt_dismissed',
        'checkout_started', 'checkout_completed', 'checkout_canceled',
        'subscription_started', 'subscription_canceled'));

DROP TABLE IF EXISTS paddle_checkouts;
DROP TABLE IF EXISTS subscription_provider_states;
