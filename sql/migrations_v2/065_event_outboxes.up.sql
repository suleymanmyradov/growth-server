-- Transactional outboxes for lifecycle-critical domain events (P1).
-- Each service writes an outbox row in the SAME transaction as its domain
-- write, so the event cannot be lost between the DB commit and the broker
-- publish. A per-service relay worker claims due rows, republishes the stored
-- payload with the stored (stable) event_id — consumers dedupe on it — and
-- deletes the row only after a successful publish. next_attempt_at acts as
-- the claim lease: a crashed relay's claims become claimable again.
--
-- One table per publishing service so the ownership rule holds: each service
-- reads and writes only its own outbox.

CREATE TABLE client_event_outbox (
    event_id        uuid PRIMARY KEY,
    event_type      text NOT NULL,
    payload         jsonb NOT NULL,
    occurred_at     timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts        integer NOT NULL DEFAULT 0
);
CREATE INDEX client_event_outbox_pending_idx
    ON client_event_outbox (next_attempt_at, created_at);

CREATE TABLE notification_event_outbox (
    event_id        uuid PRIMARY KEY,
    event_type      text NOT NULL,
    payload         jsonb NOT NULL,
    occurred_at     timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts        integer NOT NULL DEFAULT 0
);
CREATE INDEX notification_event_outbox_pending_idx
    ON notification_event_outbox (next_attempt_at, created_at);

CREATE TABLE auth_event_outbox (
    event_id        uuid PRIMARY KEY,
    event_type      text NOT NULL,
    payload         jsonb NOT NULL,
    occurred_at     timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts        integer NOT NULL DEFAULT 0
);
CREATE INDEX auth_event_outbox_pending_idx
    ON auth_event_outbox (next_attempt_at, created_at);

CREATE TABLE ai_coach_event_outbox (
    event_id        uuid PRIMARY KEY,
    event_type      text NOT NULL,
    payload         jsonb NOT NULL,
    occurred_at     timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    attempts        integer NOT NULL DEFAULT 0
);
CREATE INDEX ai_coach_event_outbox_pending_idx
    ON ai_coach_event_outbox (next_attempt_at, created_at);
