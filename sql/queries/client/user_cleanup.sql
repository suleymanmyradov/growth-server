-- Bulk cleanup queries for user_deleted event consumers.
-- Each deletes all rows owned by the given user from a client-owned table.

-- name: DeleteHabitsByUser :exec
DELETE FROM habits WHERE user_id = $1;

-- name: DeleteGoalsByUser :exec
DELETE FROM goals WHERE user_id = $1;

-- name: DeleteCheckInsByUser :exec
DELETE FROM check_ins WHERE user_id = $1;

-- name: DeleteWeeklyReviewsByUser :exec
DELETE FROM weekly_reviews WHERE user_id = $1;

-- name: DeletePlanAdjustmentsByUser :exec
DELETE FROM plan_adjustments WHERE user_id = $1;

-- name: DeleteSavedArticlesByUser :exec
DELETE FROM saved_articles WHERE user_id = $1;

-- name: DeleteSavedGoalsByUser :exec
DELETE FROM saved_goals WHERE user_id = $1;

-- name: DeleteSavedHabitsByUser :exec
DELETE FROM saved_habits WHERE user_id = $1;

-- name: DeleteArticleLikesByUser :exec
DELETE FROM article_likes WHERE user_id = $1;

-- name: DeleteArticleSharesByUser :exec
DELETE FROM article_shares WHERE user_id = $1;

-- name: DeleteSubscriptionsByUser :exec
DELETE FROM subscriptions WHERE user_id = $1;

-- name: DeleteUpgradeEventsByUser :exec
DELETE FROM upgrade_events WHERE user_id = $1;
