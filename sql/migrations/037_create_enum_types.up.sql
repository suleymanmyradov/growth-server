-- Create ENUM types to replace VARCHAR + CHECK constraints
-- This improves data integrity, storage efficiency, and makes value changes explicit

CREATE TYPE check_in_status AS ENUM ('completed', 'missed');
CREATE TYPE mood_type AS ENUM ('great', 'okay', 'low', 'stressed');
CREATE TYPE energy_level AS ENUM ('high', 'medium', 'low');
CREATE TYPE blocker_type AS ENUM ('lack_of_time', 'low_motivation', 'too_distracted', 'unclear_plan', 'other');
CREATE TYPE notification_type AS ENUM ('habit_reminder','missed_check_in','goal_deadline','achievement','weekly_review','encouragement','system');
CREATE TYPE activity_type AS ENUM ('habit_completed','goal_created','goal_completed','article_saved','check_in_completed','check_in_missed','weekly_review_generated');
CREATE TYPE conversation_type AS ENUM ('coach', 'therapist');
CREATE TYPE saved_item_type AS ENUM ('article', 'goal', 'habit');
CREATE TYPE theme_type AS ENUM ('light', 'dark', 'system');
CREATE TYPE accountability_style_type AS ENUM ('gentle', 'balanced', 'strict');
CREATE TYPE coach_tone_type AS ENUM ('supportive', 'direct', 'warm', 'practical', 'challenging');
CREATE TYPE difficulty_level_type AS ENUM ('easy', 'adaptive', 'ambitious');
CREATE TYPE subscription_status_type AS ENUM ('free', 'trialing', 'active', 'past_due', 'canceled', 'expired');
CREATE TYPE billing_interval_type AS ENUM ('monthly', 'annual');
CREATE TYPE upgrade_event_type AS ENUM ('prompt_viewed','prompt_clicked','prompt_dismissed','checkout_started','checkout_completed','checkout_canceled','subscription_started','subscription_canceled');
CREATE TYPE reminder_type AS ENUM ('habit_reminder','missed_check_in','weekly_review','encouragement');
CREATE TYPE plan_adjustment_status_type AS ENUM ('pending', 'accepted', 'dismissed', 'applied');
CREATE TYPE plan_adjustment_source_type AS ENUM ('check_in', 'weekly_review', 'assistant', 'pattern_analysis');
CREATE TYPE plan_adjustment_type_type AS ENUM ('reduce_difficulty', 'increase_difficulty', 'change_time', 'clarify_plan', 'pause', 'keep_same');
CREATE TYPE message_role_type AS ENUM ('user', 'assistant');
