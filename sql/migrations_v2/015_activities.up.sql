-- Append-only user activity feed.
CREATE TABLE activities (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        text NOT NULL CHECK (type IN (
                    'habit_completed', 'goal_created', 'goal_completed',
                    'article_saved', 'check_in_completed', 'check_in_missed',
                    'weekly_review_generated')),
    title       varchar(200) NOT NULL,
    description text,
    metadata    jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_activities_user ON activities (user_id, created_at DESC);
