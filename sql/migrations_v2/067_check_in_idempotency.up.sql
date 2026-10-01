-- P3: request-level idempotency for CreateCheckIn.
--
-- check_ins.version increments on every upsert transition. The published
-- check_in_created event ID is derived deterministically from
-- (check_in_id, version), so a redelivery or relay replay carries the same
-- event ID and consumer-side processed_events dedup makes it a no-op, while
-- a real content change (missed -> completed) always yields a fresh ID.
ALTER TABLE check_ins
    ADD COLUMN version integer NOT NULL DEFAULT 1;

-- activities.dedupe_key gives activity writes a race-safe idempotency key:
-- two identical check-in requests racing through the upsert can both pass
-- the "unchanged" check, but only one INSERT survives the unique index.
ALTER TABLE activities
    ADD COLUMN dedupe_key text;
CREATE UNIQUE INDEX activities_dedupe_key_idx
    ON activities (dedupe_key)
    WHERE dedupe_key IS NOT NULL;
