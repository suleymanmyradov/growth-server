-- Analytics rollup tables owned by the analytics-consumer service.
-- These are populated by consuming domain events from the growth.events
-- Kafka topic and aggregating them into queryable rollups.

-- user_lifecycle_events: one row per user lifecycle milestone (signup,
-- onboarding complete, first goal, first habit, first check-in, etc.).
-- Used to compute activation funnel metrics and time-to-activation.
CREATE TABLE user_lifecycle_events (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id      uuid NOT NULL,
    event_type   text NOT NULL,  -- signup, onboarding_completed, first_goal, first_habit, first_check_in, etc.
    occurred_at  timestamptz NOT NULL DEFAULT now(),
    metadata     jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_lifecycle_user ON user_lifecycle_events (user_id, event_type);
CREATE INDEX idx_lifecycle_type_date ON user_lifecycle_events (event_type, occurred_at);

-- daily_metrics: per-day aggregate metrics (DAU, new signups, check-ins,
-- habit completions, goals created, etc.). One row per metric per day.
CREATE TABLE daily_metrics (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    metric_date  date NOT NULL,
    metric_name  text NOT NULL,  -- dau, new_signups, check_ins, habit_completions, goals_created, etc.
    metric_value integer NOT NULL DEFAULT 0,
    metadata     jsonb NOT NULL DEFAULT '{}',
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (metric_date, metric_name)
);

CREATE INDEX idx_daily_metrics_date ON daily_metrics (metric_date DESC);
CREATE INDEX idx_daily_metrics_name ON daily_metrics (metric_name, metric_date DESC);

-- retention_cohorts: cohort retention table. One row per (cohort_date,
-- cohort_size, period) showing how many users from that cohort are still
-- active at day N.
CREATE TABLE retention_cohorts (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    cohort_date   date NOT NULL,       -- signup date of the cohort
    cohort_size   integer NOT NULL DEFAULT 0,
    period_days   integer NOT NULL,    -- 1, 7, 14, 30, etc.
    retained_count integer NOT NULL DEFAULT 0,
    retention_rate numeric(5,4) NOT NULL DEFAULT 0,  -- 0.0000 - 1.0000
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (cohort_date, period_days)
);

CREATE INDEX idx_retention_cohort_date ON retention_cohorts (cohort_date DESC);
CREATE INDEX idx_retention_period ON retention_cohorts (period_days, cohort_date DESC);

-- conversion_funnels: tracks subscription conversion funnel stages.
-- One row per (user_id, stage) recording when the user entered that stage.
CREATE TABLE conversion_funnels (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id      uuid NOT NULL,
    stage        text NOT NULL,  -- signup, trial_started, trial_converted, paid, churned
    entered_at   timestamptz NOT NULL DEFAULT now(),
    metadata     jsonb NOT NULL DEFAULT '{}',
    UNIQUE (user_id, stage)
);

CREATE INDEX idx_conversion_stage ON conversion_funnels (stage, entered_at DESC);
CREATE INDEX idx_conversion_user ON conversion_funnels (user_id, stage);

-- analytics_processed_events: idempotency table for the analytics consumer.
CREATE TABLE analytics_processed_events (
    event_id   uuid PRIMARY KEY,
    processed_at timestamptz NOT NULL DEFAULT now()
);
