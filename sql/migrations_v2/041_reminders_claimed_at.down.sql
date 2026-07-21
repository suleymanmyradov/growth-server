-- Reverse the claim/lease/ack model added in 041.
DROP INDEX IF EXISTS idx_reminders_stale_claims;
DROP INDEX IF EXISTS idx_reminders_claimable;
ALTER TABLE reminders DROP COLUMN IF EXISTS claimed_at;
