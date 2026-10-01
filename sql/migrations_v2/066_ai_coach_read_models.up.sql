-- ai-coach-consumer owned tables (P2). The consumer previously read
-- check_ins / habits / coaching_profiles (client-owned) and
-- conversations / conversation_messages (ai-coach-owned) directly, plus the
-- notifications-owned processed_events table. These local read models are
-- fed by events instead:
--   * ai_coach_check_ins      — upserted from check_in_created events
--   * ai_coach_profiles       — upserted from coaching_profile_changed events
--   * ai_coach_processed_events — consumer dedup (was processed_events)

CREATE TABLE ai_coach_check_ins (
    check_in_id uuid PRIMARY KEY,
    user_id     uuid NOT NULL,
    habit_id    uuid NOT NULL,
    habit_name  text NOT NULL DEFAULT '',
    status      text NOT NULL,
    mood        text,
    energy      text,
    blocker     text,
    note        text,
    local_date  date,
    occurred_at timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ai_coach_check_ins_user_date_idx
    ON ai_coach_check_ins (user_id, local_date);
CREATE INDEX ai_coach_check_ins_user_created_idx
    ON ai_coach_check_ins (user_id, created_at DESC);

CREATE TABLE ai_coach_profiles (
    user_id              uuid PRIMARY KEY,
    accountability_style text NOT NULL DEFAULT 'balanced',
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ai_coach_processed_events (
    consumer     text NOT NULL,
    event_id     text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX ai_coach_processed_events_processed_at_idx
    ON ai_coach_processed_events (processed_at);
