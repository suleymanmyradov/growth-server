CREATE TABLE upgrade_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(40) NOT NULL
        CHECK (event_type IN (
            'prompt_viewed',
            'prompt_clicked',
            'prompt_dismissed',
            'checkout_started',
            'checkout_completed',
            'checkout_canceled',
            'subscription_started',
            'subscription_canceled'
        )),
    surface VARCHAR(50) NOT NULL,
    trigger VARCHAR(50),
    plan_code VARCHAR(30),
    billing_interval VARCHAR(20),
    feedback_reason VARCHAR(100),
    feedback_note TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_upgrade_events_user_id ON upgrade_events(user_id);
CREATE INDEX idx_upgrade_events_event_type ON upgrade_events(event_type);
CREATE INDEX idx_upgrade_events_surface ON upgrade_events(surface);
CREATE INDEX idx_upgrade_events_created_at ON upgrade_events(created_at DESC);
