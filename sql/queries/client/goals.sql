-- Goal rows are returned with a resolved category slug and a derived
-- `completed` flag so callers never deal with category_id directly.
-- All goal SELECTs include the typed-measurement columns so the logic
-- layer can compute/return progress per measurement type.

-- name: ListGoals :many
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       g.measurement, g.start_value, g.current_value, g.target_value, g.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.user_id = $1
ORDER BY g.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetGoal :one
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       g.measurement, g.start_value, g.current_value, g.target_value, g.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.id = $1;

-- name: CreateGoal :one
WITH ins AS (
    INSERT INTO goals (title, description, category_id, due_date, user_id,
                       measurement, start_value, current_value, target_value, unit)
    VALUES ($1, $2, (SELECT c2.id FROM categories c2 WHERE c2.slug = $3), $4, $5,
            $6, $7, $8, $9, $10)
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT ins.id, ins.user_id, ins.category_id, ins.title, ins.description, ins.status, ins.progress, ins.due_date, ins.created_at, ins.updated_at,
       ins.measurement, ins.start_value, ins.current_value, ins.target_value, ins.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (ins.status = 'completed') AS completed
FROM ins
LEFT JOIN categories c ON c.id = ins.category_id;

-- name: UpdateGoal :one
WITH upd AS (
    UPDATE goals
    SET title = $2, description = $3,
        category_id = (SELECT c2.id FROM categories c2 WHERE c2.slug = $4),
        due_date = $5,
        measurement = $6,
        start_value = $7,
        current_value = $8,
        target_value = $9,
        unit = $10
    WHERE goals.id = $1
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       upd.measurement, upd.start_value, upd.current_value, upd.target_value, upd.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1;

-- name: ToggleGoal :one
-- Toggles status between active/completed. For binary goals this is the
-- progress input (done/not done); for derived types the caller recomputes
-- progress after toggling back to active. Progress is set to 100 when
-- completing and reset to 0 when reactivating — for manual goals 0 is the
-- correct value (the user explicitly un-completed it), and for derived
-- types the recompute in the logic layer overwrites it with the true value.
WITH upd AS (
    UPDATE goals
    SET status = CASE WHEN status = 'completed' THEN 'active' ELSE 'completed' END,
        progress = CASE WHEN status = 'completed' THEN 0 ELSE 100 END
    WHERE goals.id = $1
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       upd.measurement, upd.start_value, upd.current_value, upd.target_value, upd.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: UpdateGoalProgress :one
-- Manual progress update (only valid for measurement='manual'; the logic
-- layer rejects other types with FailedPrecondition).
WITH upd AS (
    UPDATE goals
    SET progress = $2,
        status = CASE WHEN $2 >= 100 THEN 'completed' ELSE status END
    WHERE goals.id = $1
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       upd.measurement, upd.start_value, upd.current_value, upd.target_value, upd.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: LogGoalValue :one
-- Writes a new current_value for a numeric goal. Progress recomputation is
-- handled by the logic layer via RecomputeGoalProgressWithRepo (single source
-- of truth in ComputeProgress), not inline SQL.
WITH upd AS (
    UPDATE goals
    SET current_value = $2
    WHERE goals.id = $1
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       upd.measurement, upd.start_value, upd.current_value, upd.target_value, upd.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: RecomputeGoalProgress :one
-- Writes a computed progress value and flips status accordingly. Called by
-- the progress engine after any write that can move the needle (milestone
-- toggle, habit link change, check-in create/delete).
WITH upd AS (
    UPDATE goals
    SET progress = $2,
        status = CASE WHEN $2 >= 100 THEN 'completed'
                      WHEN status = 'completed' AND $2 < 100 THEN 'active'
                      ELSE status END
    WHERE goals.id = $1
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       upd.measurement, upd.start_value, upd.current_value, upd.target_value, upd.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: GetGoalsByIDs :many
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       g.measurement, g.start_value, g.current_value, g.target_value, g.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.id = ANY($1::uuid[]);

-- name: CountGoalsByUser :one
SELECT COUNT(*) FROM goals WHERE user_id = $1;

-- name: CountActiveGoalsByUser :one
SELECT COUNT(*) FROM goals WHERE user_id = $1 AND status != 'completed';

-- name: ListGoalsKeyset :many
-- Keyset pagination: pass last_created_at from the previous page (or NULL).
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       g.measurement, g.start_value, g.current_value, g.target_value, g.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.user_id = $1
  AND ($2::timestamptz IS NULL OR g.created_at < $2)
ORDER BY g.created_at DESC
LIMIT $3;

-- ─── Goal-habit links ───────────────────────────────────────────────────────

-- name: ListGoalHabitIDs :many
-- Batch-fetch all (goal_id, habit_id) pairs for a user's goals so the logic
-- layer can group them per-goal without N+1 queries.
SELECT gh.goal_id, gh.habit_id
FROM goal_habits gh
JOIN goals g ON g.id = gh.goal_id
WHERE g.user_id = $1;

-- name: ListGoalHabitIDsByGoal :many
-- Fetch habit IDs linked to a single goal.
SELECT habit_id FROM goal_habits WHERE goal_id = $1;

-- name: ListGoalIDsByHabit :many
-- Fetch goal IDs linked to a single habit. Drives recompute-on-check-in.
SELECT goal_id FROM goal_habits WHERE habit_id = $1;

-- name: UnlinkAllGoalHabits :exec
-- Remove all habit links for a goal. Call before LinkGoalHabitsBatch to replace.
DELETE FROM goal_habits WHERE goal_id = $1;

-- name: LinkGoalHabitsBatch :exec
-- Link multiple habits to a goal at once.
-- $1 = goal_id, $2 = array of habit_ids to link.
INSERT INTO goal_habits (goal_id, habit_id)
SELECT $1, unnest($2::uuid[])
ON CONFLICT DO NOTHING;

-- ─── Goal milestones ────────────────────────────────────────────────────────

-- name: ListGoalMilestones :many
SELECT id, goal_id, title, sort_order, done_at, created_at
FROM goal_milestones
WHERE goal_id = $1
ORDER BY sort_order, created_at;

-- name: ListGoalMilestonesByGoals :many
-- Batch-fetch milestones for multiple goals (avoids N+1 in ListGoals).
-- $1 = array of goal_ids.
SELECT id, goal_id, title, sort_order, done_at, created_at
FROM goal_milestones
WHERE goal_id = ANY($1::uuid[])
ORDER BY goal_id, sort_order, created_at;

-- name: CreateGoalMilestone :one
INSERT INTO goal_milestones (goal_id, title, sort_order)
VALUES ($1, $2, $3)
RETURNING id, goal_id, title, sort_order, done_at, created_at;

-- name: ToggleGoalMilestone :one
-- Flips done_at between NULL and now(). Scoped to goal_id to prevent
-- cross-goal milestone access via a mismatched (goalId, milestoneId) pair.
UPDATE goal_milestones
SET done_at = CASE WHEN done_at IS NULL THEN now() ELSE NULL END
WHERE id = $1 AND goal_id = $2
RETURNING id, goal_id, title, sort_order, done_at, created_at;

-- name: DeleteGoalMilestone :exec
-- Scoped to goal_id to prevent cross-goal milestone deletion.
DELETE FROM goal_milestones WHERE id = $1 AND goal_id = $2;

-- name: UpdateGoalMilestone :one
-- Updates title and sort_order for an existing milestone. Scoped to goal_id
-- to prevent cross-goal access. done_at is preserved (not in SET clause).
UPDATE goal_milestones
SET title = $3, sort_order = $4
WHERE id = $1 AND goal_id = $2
RETURNING id, goal_id, title, sort_order, done_at, created_at;

-- name: CountGoalMilestones :one
-- Returns (total, done) counts for a goal's milestone progress.
SELECT COUNT(*) AS total,
       COUNT(*) FILTER (WHERE done_at IS NOT NULL) AS done
FROM goal_milestones
WHERE goal_id = $1;

-- ─── Habit-driven progress inputs ───────────────────────────────────────────
-- Note: CountCompletedCheckInDays lives in check_ins.sql (owned by the
-- check-ins domain). The goals logic calls it through ICheckIns.

-- name: UpdateGoalDescription :one
-- Updates only the description column (used by apply-plan-adjustment for
-- clarify_plan). Leaves title, category, measurement, etc. intact.
WITH upd AS (
    UPDATE goals
    SET description = $2
    WHERE goals.id = $1
    RETURNING id, user_id, category_id, title, description, status, progress, due_date, created_at, updated_at,
              measurement, start_value, current_value, target_value, unit
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       upd.measurement, upd.start_value, upd.current_value, upd.target_value, upd.unit,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;
