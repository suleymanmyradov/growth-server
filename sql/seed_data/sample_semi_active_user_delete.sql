-- ============================================================
-- Removes the semi-active test user and ALL of their data.
--
-- This is the guaranteed-clean removal path. It covers the
-- ai-coach tables (conversations + conversation_messages) too,
-- which the normal account-deletion flow does NOT clean up
-- (ai-coach has no user_deleted consumer today).
--
-- Safe to run even if the user was already deleted.
--
--   psql -U postgres -d growth -f seed_data/sample_semi_active_user_delete.sql
-- ============================================================

BEGIN;

-- ai-coach: messages cascade from conversations, but delete explicitly.
DELETE FROM conversation_messages cm
    USING conversations c, users u
    WHERE c.id = cm.conversation_id
      AND c.user_id = u.id
      AND u.email = 'semiaactive.test@example.com';
DELETE FROM conversations c
    USING users u
    WHERE c.user_id = u.id AND u.email = 'semiaactive.test@example.com';

-- client: goal_habits + check_ins cascade from goals/habits, but delete explicitly.
DELETE FROM check_ins ci
    USING users u
    WHERE ci.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM goal_habits gh
    USING goals g, users u
    WHERE gh.goal_id = g.id AND g.user_id = u.id
      AND u.email = 'semiaactive.test@example.com';
DELETE FROM goals g
    USING users u
    WHERE g.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM habits h
    USING users u
    WHERE h.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM subscriptions s
    USING users u
    WHERE s.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM user_preferences up
    USING users u
    WHERE up.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM user_profiles up
    USING users u
    WHERE up.id = u.id AND u.email = 'semiaactive.test@example.com';

-- auth: the user row itself.
DELETE FROM users u
    WHERE u.email = 'semiaactive.test@example.com';

COMMIT;
