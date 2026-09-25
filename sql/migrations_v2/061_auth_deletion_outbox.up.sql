CREATE TABLE auth_deletion_outbox (
    event_id uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 0
);
CREATE INDEX auth_deletion_outbox_pending_idx ON auth_deletion_outbox (next_attempt_at, created_at);
