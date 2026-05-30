-- Apply ENUM types to all tables, dropping old CHECK constraints first

-- Helper: drop a CHECK constraint by table + expression fragment
-- check_ins
-- Drop partial index referencing status::text before converting to enum
DROP INDEX IF EXISTS idx_check_ins_missed_blocker;
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'check_ins'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%status%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE check_ins DROP CONSTRAINT %I', cname); END IF;
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'check_ins'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%mood%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE check_ins DROP CONSTRAINT %I', cname); END IF;
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'check_ins'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%energy%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE check_ins DROP CONSTRAINT %I', cname); END IF;
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'check_ins'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%blocker%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE check_ins DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE check_ins
    ALTER COLUMN status TYPE check_in_status USING status::check_in_status,
    ALTER COLUMN mood TYPE mood_type USING mood::mood_type,
    ALTER COLUMN energy TYPE energy_level USING energy::energy_level,
    ALTER COLUMN blocker TYPE blocker_type USING blocker::blocker_type;
-- Recreate partial index using enum comparison (no ::text cast needed)
CREATE INDEX IF NOT EXISTS idx_check_ins_missed_blocker ON check_ins USING btree (user_id, created_at) WHERE (status = 'missed' AND blocker IS NOT NULL);

-- notifications
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_item_type_check;
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'notifications'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%item_type%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE notifications DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE notifications ALTER COLUMN item_type TYPE notification_type USING item_type::notification_type;

-- activities
ALTER TABLE activities DROP CONSTRAINT IF EXISTS activities_item_type_check;
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'activities'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%item_type%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE activities DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE activities ALTER COLUMN item_type TYPE activity_type USING item_type::activity_type;

-- saved_items
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'saved_items'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%item_type%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE saved_items DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE saved_items ALTER COLUMN item_type TYPE saved_item_type USING item_type::saved_item_type;

-- conversations
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'conversations'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%item_type%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE conversations DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE conversations ALTER COLUMN item_type TYPE conversation_type USING item_type::conversation_type;

-- messages
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'messages'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%role%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE messages DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE messages ALTER COLUMN role TYPE message_role_type USING role::message_role_type;

-- user_settings
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'user_settings'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%theme%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE user_settings DROP CONSTRAINT %I', cname); END IF;
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'user_settings'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%accountability_style%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE user_settings DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE user_settings
    ALTER COLUMN theme DROP DEFAULT,
    ALTER COLUMN accountability_style DROP DEFAULT,
    ALTER COLUMN theme TYPE theme_type USING theme::theme_type,
    ALTER COLUMN accountability_style TYPE accountability_style_type USING accountability_style::accountability_style_type;
ALTER TABLE user_settings
    ALTER COLUMN theme SET DEFAULT 'system',
    ALTER COLUMN accountability_style SET DEFAULT 'balanced';

-- user_coaching_profiles
DO $$
DECLARE cname TEXT;
BEGIN
    FOR cname IN SELECT conname FROM pg_constraint WHERE conrelid = 'user_coaching_profiles'::regclass AND contype = 'c' LOOP
        EXECUTE format('ALTER TABLE user_coaching_profiles DROP CONSTRAINT %I', cname);
    END LOOP;
END $$;
ALTER TABLE user_coaching_profiles
    ALTER COLUMN accountability_style DROP DEFAULT,
    ALTER COLUMN preferred_tone DROP DEFAULT,
    ALTER COLUMN difficulty_preference DROP DEFAULT,
    ALTER COLUMN accountability_style TYPE accountability_style_type USING accountability_style::accountability_style_type,
    ALTER COLUMN preferred_tone TYPE coach_tone_type USING preferred_tone::coach_tone_type,
    ALTER COLUMN difficulty_preference TYPE difficulty_level_type USING difficulty_preference::difficulty_level_type;
ALTER TABLE user_coaching_profiles
    ALTER COLUMN accountability_style SET DEFAULT 'balanced',
    ALTER COLUMN preferred_tone SET DEFAULT 'supportive',
    ALTER COLUMN difficulty_preference SET DEFAULT 'adaptive';

-- user_subscriptions
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'user_subscriptions'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%status%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE user_subscriptions DROP CONSTRAINT %I', cname); END IF;
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'user_subscriptions'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%billing_interval%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE user_subscriptions DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE user_subscriptions
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE subscription_status_type USING status::subscription_status_type,
    ALTER COLUMN billing_interval TYPE billing_interval_type USING billing_interval::billing_interval_type;
ALTER TABLE user_subscriptions
    ALTER COLUMN status SET DEFAULT 'free';

-- upgrade_events
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'upgrade_events'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%event_type%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE upgrade_events DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE upgrade_events ALTER COLUMN event_type TYPE upgrade_event_type USING event_type::upgrade_event_type;

-- reminder_queue
-- Drop index that uses AT TIME ZONE (STABLE function) before table rewrite
DROP INDEX IF EXISTS uniq_reminder_queue_pending_per_day;
DO $$
DECLARE cname TEXT;
BEGIN
    SELECT conname INTO cname FROM pg_constraint WHERE conrelid = 'reminder_queue'::regclass AND contype = 'c' AND pg_get_expr(conbin, conrelid) LIKE '%type%';
    IF cname IS NOT NULL THEN EXECUTE format('ALTER TABLE reminder_queue DROP CONSTRAINT %I', cname); END IF;
END $$;
ALTER TABLE reminder_queue ALTER COLUMN type TYPE reminder_type USING type::reminder_type;
-- Recreate index after table rewrite
CREATE UNIQUE INDEX IF NOT EXISTS uniq_reminder_queue_pending_per_day ON reminder_queue USING btree (user_id, type, CAST((scheduled_at AT TIME ZONE 'UTC') AS date)) WHERE (sent = false);

-- plan_adjustment_suggestions
DO $$
DECLARE cname TEXT;
BEGIN
    FOR cname IN SELECT conname FROM pg_constraint WHERE conrelid = 'plan_adjustment_suggestions'::regclass AND contype = 'c' LOOP
        EXECUTE format('ALTER TABLE plan_adjustment_suggestions DROP CONSTRAINT %I', cname);
    END LOOP;
END $$;
ALTER TABLE plan_adjustment_suggestions
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN source TYPE plan_adjustment_source_type USING source::plan_adjustment_source_type,
    ALTER COLUMN adjustment_type TYPE plan_adjustment_type_type USING adjustment_type::plan_adjustment_type_type,
    ALTER COLUMN status TYPE plan_adjustment_status_type USING status::plan_adjustment_status_type;
ALTER TABLE plan_adjustment_suggestions
    ALTER COLUMN status SET DEFAULT 'pending';
