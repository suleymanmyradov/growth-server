-- Reminders: sent_at IS NULL means pending.

-- name: EnqueueReminder :one
-- Excludes goal_deadline (which has its own per-goal unique index and query).
INSERT INTO reminders (user_id, type, scheduled_at, metadata)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, type, ((scheduled_at AT TIME ZONE 'UTC')::date)) WHERE sent_at IS NULL AND type <> 'goal_deadline'
DO UPDATE SET scheduled_at = EXCLUDED.scheduled_at,
              metadata = EXCLUDED.metadata
RETURNING id, user_id, type, scheduled_at, sent_at, metadata, created_at;

-- name: EnqueueGoalDeadlineReminder :one
-- Inserts or updates a per-goal deadline reminder. The unique index
-- uniq_reminders_goal_deadline_per_goal ensures one pending reminder per
-- (user, goal), so re-enqueuing on a deadline change upserts in place.
INSERT INTO reminders (user_id, type, scheduled_at, metadata)
VALUES ($1, 'goal_deadline', $2, $3)
ON CONFLICT (user_id, (metadata->>'goalId')) WHERE sent_at IS NULL AND type = 'goal_deadline'
DO UPDATE SET scheduled_at = EXCLUDED.scheduled_at,
              metadata = EXCLUDED.metadata
RETURNING id, user_id, type, scheduled_at, sent_at, metadata, created_at;

-- name: CancelPendingGoalDeadlineReminder :execrows
-- Cancel the pending goal_deadline reminder for a specific (user, goal).
DELETE FROM reminders
WHERE user_id = $1
  AND type = 'goal_deadline'
  AND sent_at IS NULL
  AND metadata->>'goalId' = $2::text;

-- name: CancelPendingReminderForDate :exec
DELETE FROM reminders
WHERE user_id = $1
  AND type = $2
  AND sent_at IS NULL
  AND (scheduled_at AT TIME ZONE $4::text)::date = $3::date;

-- name: CancelPendingByType :execrows
-- Cancel all unsent reminders of a given type for a user. Used when the user
-- disables a notification preference (e.g. habit reminders) so no further
-- reminders of that type fire until re-enabled.
DELETE FROM reminders
WHERE user_id = $1
  AND type = $2
  AND sent_at IS NULL;

-- name: ClaimDueReminders :many
-- Claim due, unclaimed, unsent reminders by setting claimed_at (a lease).
-- Does NOT set sent_at — that happens only after successful Kafka publish
-- via MarkReminderSent. Stale claims (claimed_at older than the lease window)
-- are released by ReleaseStaleClaims so a crashed scheduler does not lose
-- reminders. Uses FOR UPDATE SKIP LOCKED so multiple instances don't claim
-- the same rows.
WITH due AS (
    SELECT id FROM reminders
    WHERE sent_at IS NULL
      AND claimed_at IS NULL
      AND scheduled_at <= now()
    ORDER BY scheduled_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE reminders r SET claimed_at = now()
FROM due
WHERE r.id = due.id
RETURNING r.id, r.user_id, r.type, r.scheduled_at, r.sent_at, r.metadata, r.created_at;

-- name: ReleaseStaleClaims :execrows
-- Release claims older than the lease window (minutes) so they can be
-- re-claimed by the next tick. This handles scheduler crashes: a reminder
-- that was claimed but never acked (sent_at still NULL) becomes claimable
-- again after the lease expires.
UPDATE reminders
SET claimed_at = NULL
WHERE sent_at IS NULL
  AND claimed_at IS NOT NULL
  AND claimed_at < now() - ($1::int * interval '1 minute');

-- name: GetPendingByUser :many
SELECT id, user_id, type, scheduled_at, sent_at, metadata, created_at
FROM reminders
WHERE user_id = $1 AND sent_at IS NULL
ORDER BY scheduled_at;

-- name: MarkReminderSent :one
UPDATE reminders
SET sent_at = now()
WHERE id = $1
RETURNING id, user_id, type, scheduled_at, sent_at, metadata, created_at;
