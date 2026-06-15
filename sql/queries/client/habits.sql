-- Habit rows carry a resolved category slug and a derived `completed` flag:
-- completed = a 'completed' check-in exists for the habit today (in the
-- owner's timezone). There is no stored boolean to keep in sync.

-- name: ListHabits :many
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.streak, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.user_id = $1
ORDER BY h.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHabit :one
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.streak, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.id = $1;

-- name: CreateHabit :one
WITH ins AS (
    INSERT INTO habits (name, description, category_id, user_id)
    VALUES ($1, $2, (SELECT c2.id FROM categories c2 WHERE c2.slug = $3), $4)
    RETURNING *
)
SELECT ins.id, ins.user_id, ins.category_id, ins.name, ins.description, ins.streak, ins.created_at, ins.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       false AS completed
FROM ins
LEFT JOIN categories c ON c.id = ins.category_id;

-- name: UpdateHabit :one
WITH upd AS (
    UPDATE habits
    SET name = $2, description = $3,
        category_id = (SELECT c2.id FROM categories c2 WHERE c2.slug = $4)
    WHERE habits.id = $1
    RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.name, upd.description, upd.streak, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = upd.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = upd.user_id), 'UTC'))::date
       ) AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: UpdateHabitStreak :one
WITH upd AS (
    UPDATE habits SET streak = $2 WHERE habits.id = $1 RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.name, upd.description, upd.streak, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = upd.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = upd.user_id), 'UTC'))::date
       ) AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: DeleteHabit :exec
DELETE FROM habits WHERE id = $1;

-- name: ToggleHabit :one
-- Toggles today's completion by inserting or deleting a completed check-in
-- for today (owner's timezone) and adjusting the streak accordingly.
WITH today AS (
    SELECT (now() AT TIME ZONE COALESCE(
        (SELECT s.timezone FROM user_settings s
         JOIN habits h ON h.user_id = s.user_id
         WHERE h.id = $1), 'UTC'))::date AS d
), existing AS (
    SELECT ci.id FROM check_ins ci
    WHERE ci.habit_id = $1
      AND ci.local_date = (SELECT d FROM today)
      AND ci.status = 'completed'
), removed AS (
    DELETE FROM check_ins WHERE id IN (SELECT id FROM existing) RETURNING id
), added AS (
    INSERT INTO check_ins (user_id, habit_id, local_date, status)
    SELECT h.user_id, h.id, (SELECT d FROM today), 'completed'
    FROM habits h
    WHERE h.id = $1 AND NOT EXISTS (SELECT 1 FROM existing)
    ON CONFLICT (habit_id, local_date) DO UPDATE SET status = 'completed'
    RETURNING id
), upd AS (
    UPDATE habits
    SET streak = CASE WHEN EXISTS (SELECT 1 FROM added)
                      THEN streak + 1
                      ELSE GREATEST(streak - 1, 0) END
    WHERE id = $1
    RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.name, upd.description, upd.streak, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (SELECT 1 FROM added) AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: MarkHabitCompleted :one
-- Idempotent: completing an already-completed habit changes nothing.
WITH today AS (
    SELECT (now() AT TIME ZONE COALESCE(
        (SELECT s.timezone FROM user_settings s
         JOIN habits h ON h.user_id = s.user_id
         WHERE h.id = $1), 'UTC'))::date AS d
), added AS (
    INSERT INTO check_ins (user_id, habit_id, local_date, status)
    SELECT h.user_id, h.id, (SELECT d FROM today), 'completed'
    FROM habits h
    WHERE h.id = $1
    ON CONFLICT (habit_id, local_date) DO UPDATE SET status = 'completed'
        WHERE check_ins.status <> 'completed'
    RETURNING id
), upd AS (
    UPDATE habits
    SET streak = streak + (CASE WHEN EXISTS (SELECT 1 FROM added) THEN 1 ELSE 0 END)
    WHERE id = $1
    RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.name, upd.description, upd.streak, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       true AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: ResetTodayHabits :execrows
-- "Uncompletes" all of today's habits by deleting today's completed check-ins.
DELETE FROM check_ins ci
WHERE ci.user_id = $1
  AND ci.status = 'completed'
  AND ci.local_date = (now() AT TIME ZONE COALESCE(
      (SELECT s.timezone FROM user_settings s WHERE s.user_id = $1), 'UTC'))::date;

-- name: GetHabitsByIDs :many
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.streak, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.id = ANY($1::uuid[]);

-- name: CountHabitsByUser :one
SELECT COUNT(*) FROM habits WHERE user_id = $1;

-- name: ListHabitsKeyset :many
-- Keyset pagination: pass last_created_at from the previous page (or NULL).
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.streak, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.user_id = $1
  AND ($2::timestamptz IS NULL OR h.created_at < $2)
ORDER BY h.created_at DESC
LIMIT $3;
