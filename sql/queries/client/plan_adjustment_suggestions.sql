-- name: CreatePlanAdjustmentSuggestion :one
INSERT INTO plan_adjustments (
    user_id,
    goal_id,
    habit_id,
    source,
    adjustment_type,
    reason,
    suggestion,
    metadata,
    week_start
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, source, adjustment_type, COALESCE(goal_id, habit_id), week_start)
DO UPDATE SET
    reason = EXCLUDED.reason,
    suggestion = EXCLUDED.suggestion,
    metadata = EXCLUDED.metadata
RETURNING *;

-- name: GetPlanAdjustmentSuggestion :one
SELECT * FROM plan_adjustments
WHERE id = $1 AND user_id = $2;

-- name: ListPendingPlanAdjustmentSuggestions :many
SELECT * FROM plan_adjustments
WHERE user_id = $1 AND status = 'pending'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAllPlanAdjustmentSuggestions :many
SELECT * FROM plan_adjustments
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPlanAdjustmentSuggestionsByHabit :many
SELECT * FROM plan_adjustments
WHERE user_id = $1 AND habit_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListPlanAdjustmentSuggestionsByGoal :many
SELECT * FROM plan_adjustments
WHERE user_id = $1 AND goal_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdatePlanAdjustmentSuggestionStatus :one
UPDATE plan_adjustments
SET status = $3
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdatePlanAdjustmentSuggestion :one
UPDATE plan_adjustments
SET adjustment_type = $3,
    reason = $4,
    suggestion = $5,
    metadata = $6
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeletePlanAdjustmentSuggestion :exec
DELETE FROM plan_adjustments
WHERE id = $1 AND user_id = $2;

-- name: CountPendingPlanAdjustmentSuggestions :one
SELECT COUNT(*) FROM plan_adjustments
WHERE user_id = $1 AND status = 'pending';

-- name: DismissOldPendingSuggestions :exec
UPDATE plan_adjustments
SET status = 'dismissed'
WHERE user_id = $1
  AND status = 'pending'
  AND created_at < now() - INTERVAL '30 days';

-- name: ApplyPlanAdjustmentSuggestion :one
UPDATE plan_adjustments
SET status = 'applied'
WHERE id = $1 AND user_id = $2
RETURNING *;
