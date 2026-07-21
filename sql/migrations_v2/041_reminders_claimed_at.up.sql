-- Add claimed_at column to support a claim/lease/ack model for reminders.
-- The scheduler claims rows (sets claimed_at) without marking them sent.
-- Only after successful Kafka publish does it ack (set sent_at).
-- Stale claims (claimed_at older than the lease window, sent_at still NULL)
-- are re-claimed by the next tick, so a crashed scheduler does not lose
-- reminders.
ALTER TABLE reminders
    ADD COLUMN IF NOT EXISTS claimed_at timestamptz;

-- Index for finding due, unclaimed, unsent reminders.
CREATE INDEX IF NOT EXISTS idx_reminders_claimable
    ON reminders (scheduled_at)
    WHERE sent_at IS NULL AND claimed_at IS NULL;

-- Index for releasing stale claims (claimed_at older than threshold, unsent).
CREATE INDEX IF NOT EXISTS idx_reminders_stale_claims
    ON reminders (claimed_at)
    WHERE sent_at IS NULL AND claimed_at IS NOT NULL;
