-- Revert ENUM columns back to VARCHAR/TEXT and re-add CHECK constraints

-- check_ins
ALTER TABLE check_ins
    ALTER COLUMN status TYPE VARCHAR(10) USING status::text,
    ALTER COLUMN mood TYPE VARCHAR(20) USING mood::text,
    ALTER COLUMN energy TYPE VARCHAR(20) USING energy::text,
    ALTER COLUMN blocker TYPE VARCHAR(50) USING blocker::text;
ALTER TABLE check_ins ADD CONSTRAINT check_ins_status_check CHECK (status IN ('completed', 'missed'));
ALTER TABLE check_ins ADD CONSTRAINT check_ins_mood_check CHECK (mood IN ('great', 'okay', 'low', 'stressed'));
ALTER TABLE check_ins ADD CONSTRAINT check_ins_energy_check CHECK (energy IN ('high', 'medium', 'low'));
ALTER TABLE check_ins ADD CONSTRAINT check_ins_blocker_check CHECK (blocker IS NULL OR blocker IN ('lack_of_time', 'low_motivation', 'too_distracted', 'unclear_plan', 'other'));

-- notifications
ALTER TABLE notifications ALTER COLUMN item_type TYPE VARCHAR(30) USING item_type::text;
ALTER TABLE notifications ADD CONSTRAINT notifications_item_type_check CHECK (item_type IN ('habit_reminder','missed_check_in','goal_deadline','achievement','weekly_review','encouragement','system'));

-- activities
ALTER TABLE activities ALTER COLUMN item_type TYPE VARCHAR(30) USING item_type::text;
ALTER TABLE activities ADD CONSTRAINT activities_item_type_check CHECK (item_type IN ('habit_completed','goal_created','goal_completed','article_saved','check_in_completed','check_in_missed','weekly_review_generated'));

-- saved_items
ALTER TABLE saved_items ALTER COLUMN item_type TYPE VARCHAR(20) USING item_type::text;
ALTER TABLE saved_items ADD CONSTRAINT saved_items_item_type_check CHECK (item_type IN ('article', 'goal', 'habit'));

-- conversations
ALTER TABLE conversations ALTER COLUMN item_type TYPE VARCHAR(20) USING item_type::text;
ALTER TABLE conversations ADD CONSTRAINT conversations_item_type_check CHECK (item_type IN ('coach', 'therapist'));

-- messages
ALTER TABLE messages ALTER COLUMN role TYPE VARCHAR(20) USING role::text;
ALTER TABLE messages ADD CONSTRAINT messages_role_check CHECK (role IN ('user', 'assistant'));

-- user_settings
ALTER TABLE user_settings
    ALTER COLUMN theme TYPE VARCHAR(20) USING theme::text,
    ALTER COLUMN accountability_style TYPE VARCHAR(20) USING accountability_style::text;
ALTER TABLE user_settings ADD CONSTRAINT user_settings_theme_check CHECK (theme IN ('light', 'dark', 'system'));
ALTER TABLE user_settings ADD CONSTRAINT user_settings_accountability_style_check CHECK (accountability_style IN ('gentle', 'balanced', 'strict'));

-- user_coaching_profiles
ALTER TABLE user_coaching_profiles
    ALTER COLUMN accountability_style TYPE VARCHAR(20) USING accountability_style::text,
    ALTER COLUMN preferred_tone TYPE VARCHAR(30) USING preferred_tone::text,
    ALTER COLUMN difficulty_preference TYPE VARCHAR(20) USING difficulty_preference::text;
ALTER TABLE user_coaching_profiles ADD CONSTRAINT user_coaching_profiles_accountability_style_check CHECK (accountability_style IN ('gentle', 'balanced', 'strict'));
ALTER TABLE user_coaching_profiles ADD CONSTRAINT user_coaching_profiles_preferred_tone_check CHECK (preferred_tone IN ('supportive', 'direct', 'warm', 'practical', 'challenging'));
ALTER TABLE user_coaching_profiles ADD CONSTRAINT user_coaching_profiles_difficulty_preference_check CHECK (difficulty_preference IN ('easy', 'adaptive', 'ambitious'));

-- user_subscriptions
ALTER TABLE user_subscriptions
    ALTER COLUMN status TYPE VARCHAR(30) USING status::text,
    ALTER COLUMN billing_interval TYPE VARCHAR(20) USING billing_interval::text;
ALTER TABLE user_subscriptions ADD CONSTRAINT user_subscriptions_status_check CHECK (status IN ('free', 'trialing', 'active', 'past_due', 'canceled', 'expired'));
ALTER TABLE user_subscriptions ADD CONSTRAINT user_subscriptions_billing_interval_check CHECK (billing_interval IN ('monthly', 'annual'));

-- upgrade_events
ALTER TABLE upgrade_events ALTER COLUMN event_type TYPE VARCHAR(40) USING event_type::text;
ALTER TABLE upgrade_events ADD CONSTRAINT upgrade_events_event_type_check CHECK (event_type IN ('prompt_viewed','prompt_clicked','prompt_dismissed','checkout_started','checkout_completed','checkout_canceled','subscription_started','subscription_canceled'));

-- reminder_queue
ALTER TABLE reminder_queue ALTER COLUMN type TYPE VARCHAR(30) USING type::text;
ALTER TABLE reminder_queue ADD CONSTRAINT reminder_queue_type_check CHECK (type IN ('habit_reminder','missed_check_in','weekly_review','encouragement'));

-- plan_adjustment_suggestions
ALTER TABLE plan_adjustment_suggestions
    ALTER COLUMN source TYPE VARCHAR(30) USING source::text,
    ALTER COLUMN adjustment_type TYPE VARCHAR(30) USING adjustment_type::text,
    ALTER COLUMN status TYPE VARCHAR(20) USING status::text;
ALTER TABLE plan_adjustment_suggestions ADD CONSTRAINT plan_adjustment_suggestions_source_check CHECK (source IN ('check_in', 'weekly_review', 'assistant', 'pattern_analysis'));
ALTER TABLE plan_adjustment_suggestions ADD CONSTRAINT plan_adjustment_suggestions_adjustment_type_check CHECK (adjustment_type IN ('reduce_difficulty', 'increase_difficulty', 'change_time', 'clarify_plan', 'pause', 'keep_same'));
ALTER TABLE plan_adjustment_suggestions ADD CONSTRAINT plan_adjustment_suggestions_status_check CHECK (status IN ('pending', 'accepted', 'dismissed', 'applied'));
