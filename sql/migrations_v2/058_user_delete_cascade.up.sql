-- Orphan cleanup + FK cascade for auth-owned per-user tables.
--
-- DeleteUser hard-deletes users and publishes user_deleted; consumers clean
-- their own tables. But auth's own satellite tables (user_profiles,
-- user_preferences) had no FK to users, so their rows orphaned on every
-- deletion — leftover PII for users who exercised their right to erasure.
-- (user_oauth_accounts already cascades.)
--
-- Step 1: remove existing orphans (data of already-deleted users).
DELETE FROM user_profiles p
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = p.id);

DELETE FROM user_preferences p
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = p.user_id);

-- Step 2: make the invariant structural so it cannot regress.
ALTER TABLE user_profiles
    ADD CONSTRAINT user_profiles_id_fkey
    FOREIGN KEY (id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_preferences
    ADD CONSTRAINT user_preferences_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
