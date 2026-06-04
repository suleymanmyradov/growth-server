CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;

--
-- Name: accountability_style_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.accountability_style_type AS ENUM (
    'gentle',
    'balanced',
    'strict'
);

--
-- Name: activity_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.activity_type AS ENUM (
    'habit_completed',
    'goal_created',
    'goal_completed',
    'article_saved',
    'check_in_completed',
    'check_in_missed',
    'weekly_review_generated'
);

--
-- Name: billing_interval_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.billing_interval_type AS ENUM (
    'monthly',
    'annual'
);

--
-- Name: blocker_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.blocker_type AS ENUM (
    'lack_of_time',
    'low_motivation',
    'too_distracted',
    'unclear_plan',
    'other'
);

--
-- Name: check_in_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.check_in_status AS ENUM (
    'completed',
    'missed'
);

--
-- Name: coach_tone_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.coach_tone_type AS ENUM (
    'supportive',
    'direct',
    'warm',
    'practical',
    'challenging'
);


--
-- Name: difficulty_level_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.difficulty_level_type AS ENUM (
    'easy',
    'adaptive',
    'ambitious'
);

--
-- Name: energy_level; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.energy_level AS ENUM (
    'high',
    'medium',
    'low'
);

--
-- Name: entity_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.entity_type AS ENUM (
    'article',
    'habit',
    'goal'
);

--
-- Name: goal_status_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.goal_status_type AS ENUM (
    'active',
    'completed',
    'archived'
);


--
-- Name: mood_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.mood_type AS ENUM (
    'great',
    'okay',
    'low',
    'stressed'
);

--
-- Name: notification_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.notification_type AS ENUM (
    'habit_reminder',
    'missed_check_in',
    'goal_deadline',
    'achievement',
    'weekly_review',
    'encouragement',
    'system'
);

--
-- Name: plan_adjustment_source_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.plan_adjustment_source_type AS ENUM (
    'check_in',
    'weekly_review',
    'assistant',
    'pattern_analysis'
);

--
-- Name: plan_adjustment_status_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.plan_adjustment_status_type AS ENUM (
    'pending',
    'accepted',
    'dismissed',
    'applied'
);

--
-- Name: plan_adjustment_type_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.plan_adjustment_type_type AS ENUM (
    'reduce_difficulty',
    'increase_difficulty',
    'change_time',
    'clarify_plan',
    'pause',
    'keep_same'
);

--
-- Name: reminder_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.reminder_type AS ENUM (
    'habit_reminder',
    'missed_check_in',
    'weekly_review',
    'encouragement'
);

--
-- Name: saved_item_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.saved_item_type AS ENUM (
    'article',
    'goal',
    'habit'
);

--
-- Name: subscription_status_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.subscription_status_type AS ENUM (
    'free',
    'trialing',
    'active',
    'past_due',
    'canceled',
    'expired'
);

--
-- Name: theme_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.theme_type AS ENUM (
    'light',
    'dark',
    'system'
);

--
-- Name: upgrade_event_type; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.upgrade_event_type AS ENUM (
    'prompt_viewed',
    'prompt_clicked',
    'prompt_dismissed',
    'checkout_started',
    'checkout_completed',
    'checkout_canceled',
    'subscription_started',
    'subscription_canceled'
);

--
-- Name: cleanup_saved_items_on_article_delete(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cleanup_saved_items_on_article_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    DELETE FROM saved_items WHERE item_type = 'article' AND item_id = OLD.id;
    RETURN OLD;
END;
$$;

--
-- Name: cleanup_saved_items_on_goal_delete(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cleanup_saved_items_on_goal_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    DELETE FROM saved_items WHERE item_type = 'goal' AND item_id = OLD.id;
    RETURN OLD;
END;
$$;

--
-- Name: cleanup_saved_items_on_habit_delete(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.cleanup_saved_items_on_habit_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    DELETE FROM saved_items WHERE item_type = 'habit' AND item_id = OLD.id;
    RETURN OLD;
END;
$$;

--
-- Name: current_app_user_id(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.current_app_user_id() RETURNS uuid
    LANGUAGE plpgsql STABLE
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
    RETURN nullif(current_setting('app.current_user_id', true), '')::UUID;
END;
$$;

--
-- Name: prevent_check_in_update(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.prevent_check_in_update() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'check_ins are immutable events and cannot be updated';
END;
$$;

--
-- Name: sync_goals_completed_from_status(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_goals_completed_from_status() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.completed := (NEW.status = 'completed');
    RETURN NEW;
END;
$$;

--
-- Name: update_article_search_vector(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_article_search_vector() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        COALESCE(NEW.title, '') || ' ' ||
        COALESCE(NEW.excerpt, '') || ' ' ||
        COALESCE(NEW.content, '') || ' ' ||
        COALESCE((SELECT name FROM categories WHERE id = NEW.category_id), '') || ' ' ||
        COALESCE(NEW.author, '')
    );
    RETURN NEW;
END;
$$;

--
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

--
-- Name: uuid_generate_v7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.uuid_generate_v7() RETURNS uuid
    LANGUAGE plpgsql
    AS $$
DECLARE
    unix_ts_ms BIGINT;
    rand_bytes BYTEA;
    uuid_bytes BYTEA;
BEGIN
    unix_ts_ms := FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000);
    rand_bytes := gen_random_bytes(10); -- remaining 80 bits
    -- 48-bit timestamp (6 bytes) + 10 random bytes = 16 bytes
    uuid_bytes := decode(lpad(to_hex(unix_ts_ms), 12, '0'), 'hex') || rand_bytes;

    -- Set version (7)
    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 0x0f) | 0x70);
    -- Set variant (RFC 4122)
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 0x3f) | 0x80);

    RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;

--
-- Name: validate_saved_item_reference(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.validate_saved_item_reference() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    item_exists BOOLEAN := FALSE;
BEGIN
    -- Validate that item_id exists in the corresponding table based on item_type
    CASE NEW.item_type
        WHEN 'article' THEN
            SELECT EXISTS(SELECT 1 FROM articles WHERE id = NEW.item_id) INTO item_exists;
        WHEN 'goal' THEN
            SELECT EXISTS(SELECT 1 FROM goals WHERE id = NEW.item_id) INTO item_exists;
        WHEN 'habit' THEN
            SELECT EXISTS(SELECT 1 FROM habits WHERE id = NEW.item_id) INTO item_exists;
        ELSE
            RAISE EXCEPTION 'Invalid item_type: %', NEW.item_type;
    END CASE;

    IF NOT item_exists THEN
        RAISE EXCEPTION 'Referenced % with id % does not exist', NEW.item_type, NEW.item_id;
    END IF;

    RETURN NEW;
END;
$$;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: activities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.activities (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    item_type public.activity_type NOT NULL,
    title character varying(200) NOT NULL,
    description text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.activities FORCE ROW LEVEL SECURITY;

--
-- Name: ai_coach_processed_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_coach_processed_events (
    event_id uuid NOT NULL,
    processed_at timestamp with time zone DEFAULT now() NOT NULL
);

--
-- Name: ai_feedback; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_feedback (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    check_in_id uuid NOT NULL,
    habit_id uuid NOT NULL,
    content text NOT NULL,
    model text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.ai_feedback FORCE ROW LEVEL SECURITY;

--
-- Name: article_shares; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.article_shares (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    article_id uuid NOT NULL,
    user_id uuid NOT NULL,
    platform character varying(50) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.article_shares FORCE ROW LEVEL SECURITY;

--
-- Name: articles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.articles (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    title character varying(200) NOT NULL,
    excerpt text,
    content text NOT NULL,
    read_time integer NOT NULL,
    image_url character varying(500),
    author character varying(100) NOT NULL,
    published_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    search_vector tsvector,
    category_id uuid,
    ai_metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT chk_articles_read_time_positive CHECK ((read_time > 0)),
    CONSTRAINT chk_articles_title_not_empty CHECK ((length(TRIM(BOTH FROM title)) > 0))
);

--
-- Name: categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.categories (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    name character varying(50) NOT NULL,
    slug character varying(50) NOT NULL,
    entity_type public.entity_type NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

--
-- Name: check_ins; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.check_ins (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    habit_id uuid NOT NULL,
    status public.check_in_status NOT NULL,
    mood public.mood_type,
    energy public.energy_level,
    blocker public.blocker_type,
    note text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    local_date date NOT NULL
);

ALTER TABLE ONLY public.check_ins FORCE ROW LEVEL SECURITY;


--
-- Name: goal_habit_relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goal_habit_relations (
    goal_id uuid NOT NULL,
    habit_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.goal_habit_relations FORCE ROW LEVEL SECURITY;

--
-- Name: goals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goals (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    title character varying(200) NOT NULL,
    description text,
    category character varying(50) NOT NULL,
    due_date timestamp with time zone,
    progress integer DEFAULT 0 NOT NULL,
    completed boolean DEFAULT false NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    status public.goal_status_type DEFAULT 'active'::public.goal_status_type NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT goals_progress_check CHECK (((progress >= 0) AND (progress <= 100)))
);

ALTER TABLE ONLY public.goals FORCE ROW LEVEL SECURITY;

--
-- Name: habits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.habits (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    streak integer DEFAULT 0 NOT NULL,
    completed boolean DEFAULT false NOT NULL,
    category character varying(50) NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    CONSTRAINT chk_habits_description_length CHECK ((length(description) <= 5000)),
    CONSTRAINT chk_habits_streak_non_negative CHECK ((streak >= 0))
);

ALTER TABLE ONLY public.habits FORCE ROW LEVEL SECURITY;


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    title character varying(200) NOT NULL,
    message text NOT NULL,
    item_type public.notification_type NOT NULL,
    is_read boolean DEFAULT false NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.notifications FORCE ROW LEVEL SECURITY;

--
-- Name: plan_adjustment_suggestions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plan_adjustment_suggestions (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    goal_id uuid,
    habit_id uuid,
    source public.plan_adjustment_source_type NOT NULL,
    adjustment_type public.plan_adjustment_type_type NOT NULL,
    reason text NOT NULL,
    suggestion text NOT NULL,
    status public.plan_adjustment_status_type DEFAULT 'pending'::public.plan_adjustment_status_type NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    week_start date,
    target_id uuid GENERATED ALWAYS AS (COALESCE(goal_id, habit_id)) STORED,
    CONSTRAINT chk_exactly_one_target CHECK ((((goal_id IS NOT NULL) AND (habit_id IS NULL)) OR ((goal_id IS NULL) AND (habit_id IS NOT NULL))))
);

ALTER TABLE ONLY public.plan_adjustment_suggestions FORCE ROW LEVEL SECURITY;

--
-- Name: plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plans (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    code character varying(30) NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    price_monthly_cents integer DEFAULT 0 NOT NULL,
    price_annual_cents integer DEFAULT 0 NOT NULL,
    active_goal_limit integer NOT NULL,
    active_habit_limit integer NOT NULL,
    weekly_review_history_limit integer NOT NULL,
    plan_adjustment_limit integer NOT NULL,
    personalized_ai_enabled boolean DEFAULT false NOT NULL,
    stripe_monthly_price_id character varying(255),
    stripe_annual_price_id character varying(255),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

--
-- Name: processed_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.processed_events (
    event_id uuid NOT NULL,
    processed_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

--
-- Name: profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.profiles (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    bio text,
    location character varying(100),
    website character varying(255),
    interests text[],
    avatar_url character varying(500),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.profiles FORCE ROW LEVEL SECURITY;

--
-- Name: reminder_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reminder_queue (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    type public.reminder_type NOT NULL,
    scheduled_at timestamp with time zone NOT NULL,
    sent boolean DEFAULT false NOT NULL,
    sent_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.reminder_queue FORCE ROW LEVEL SECURITY;

--
-- Name: saved_articles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saved_articles (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    article_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.saved_articles FORCE ROW LEVEL SECURITY;

--
-- Name: saved_goals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saved_goals (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    goal_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.saved_goals FORCE ROW LEVEL SECURITY;

--
-- Name: saved_habits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saved_habits (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    habit_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.saved_habits FORCE ROW LEVEL SECURITY;

--
-- Name: saved_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.saved_items (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    item_type public.saved_item_type NOT NULL,
    item_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

--
-- Name: upgrade_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upgrade_events (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    event_type public.upgrade_event_type NOT NULL,
    surface character varying(50) NOT NULL,
    trigger character varying(50),
    plan_code character varying(30),
    billing_interval public.billing_interval_type,
    feedback_reason character varying(100),
    feedback_note text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    plan_id uuid
);

ALTER TABLE ONLY public.upgrade_events FORCE ROW LEVEL SECURITY;

--
-- Name: user_coaching_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_coaching_profiles (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    accountability_style public.accountability_style_type DEFAULT 'balanced'::public.accountability_style_type NOT NULL,
    preferred_tone public.coach_tone_type DEFAULT 'supportive'::public.coach_tone_type NOT NULL,
    difficulty_preference public.difficulty_level_type DEFAULT 'adaptive'::public.difficulty_level_type NOT NULL,
    primary_motivation text,
    common_blockers jsonb DEFAULT '[]'::jsonb NOT NULL,
    coaching_notes jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_context_refresh_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

ALTER TABLE ONLY public.user_coaching_profiles FORCE ROW LEVEL SECURITY;

--
-- Name: user_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_settings (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    theme public.theme_type DEFAULT 'system'::public.theme_type NOT NULL,
    language character varying(10) DEFAULT 'en'::character varying NOT NULL,
    timezone character varying(50) DEFAULT 'UTC'::character varying NOT NULL,
    email_notifications boolean DEFAULT true NOT NULL,
    push_notifications boolean DEFAULT true NOT NULL,
    habit_reminders boolean DEFAULT true NOT NULL,
    goal_reminders boolean DEFAULT true NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    accountability_style public.accountability_style_type DEFAULT 'balanced'::public.accountability_style_type NOT NULL,
    check_in_time time without time zone DEFAULT '09:00:00'::time without time zone NOT NULL,
    onboarding_completed boolean DEFAULT false NOT NULL,
    version integer DEFAULT 1 NOT NULL
);

ALTER TABLE ONLY public.user_settings FORCE ROW LEVEL SECURITY;

--
-- Name: user_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_subscriptions (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    status public.subscription_status_type DEFAULT 'free'::public.subscription_status_type NOT NULL,
    billing_interval public.billing_interval_type,
    current_period_start timestamp with time zone,
    current_period_end timestamp with time zone,
    trial_end timestamp with time zone,
    cancel_at_period_end boolean DEFAULT false NOT NULL,
    stripe_customer_id character varying(255),
    stripe_subscription_id character varying(255),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_subscription_active_has_dates CHECK (((status <> ALL (ARRAY['active'::public.subscription_status_type, 'trialing'::public.subscription_status_type])) OR ((billing_interval IS NOT NULL) AND (current_period_start IS NOT NULL) AND (current_period_end IS NOT NULL))))
);

ALTER TABLE ONLY public.user_subscriptions FORCE ROW LEVEL SECURITY;

--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    full_name character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_username_format CHECK (((username)::text ~* '^[a-z0-9_-]+$'::text))
);

--
-- Name: weekly_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.weekly_reviews (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    user_id uuid NOT NULL,
    week_start date NOT NULL,
    week_end date NOT NULL,
    total_habits integer DEFAULT 0 NOT NULL,
    completed_check_ins integer DEFAULT 0 NOT NULL,
    missed_check_ins integer DEFAULT 0 NOT NULL,
    completion_rate numeric(5,2) DEFAULT 0 NOT NULL,
    best_day character varying(20),
    hardest_day character varying(20),
    top_blocker character varying(50),
    mood_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    energy_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    habit_breakdown jsonb DEFAULT '[]'::jsonb NOT NULL,
    ai_summary text,
    suggested_adjustments jsonb DEFAULT '[]'::jsonb NOT NULL,
    next_week_plan jsonb DEFAULT '{}'::jsonb NOT NULL,
    generated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT chk_completion_rate_range CHECK (((completion_rate >= (0)::numeric) AND (completion_rate <= (100)::numeric))),
    CONSTRAINT weekly_reviews_check CHECK ((week_end > week_start))
);

ALTER TABLE ONLY public.weekly_reviews FORCE ROW LEVEL SECURITY;

--
-- Name: activities activities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activities
    ADD CONSTRAINT activities_pkey PRIMARY KEY (id);

--
-- Name: ai_coach_processed_events ai_coach_processed_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_coach_processed_events
    ADD CONSTRAINT ai_coach_processed_events_pkey PRIMARY KEY (event_id);

--
-- Name: ai_feedback ai_feedback_check_in_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_feedback
    ADD CONSTRAINT ai_feedback_check_in_id_key UNIQUE (check_in_id);

--
-- Name: ai_feedback ai_feedback_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_feedback
    ADD CONSTRAINT ai_feedback_pkey PRIMARY KEY (id);

--
-- Name: article_shares article_shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.article_shares
    ADD CONSTRAINT article_shares_pkey PRIMARY KEY (id);

--
-- Name: articles articles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.articles
    ADD CONSTRAINT articles_pkey PRIMARY KEY (id);

--
-- Name: categories categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_pkey PRIMARY KEY (id);

--
-- Name: categories categories_slug_entity_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.categories
    ADD CONSTRAINT categories_slug_entity_type_key UNIQUE (slug, entity_type);

--
-- Name: check_ins check_ins_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.check_ins
    ADD CONSTRAINT check_ins_pkey PRIMARY KEY (id);


--
-- Name: goal_habit_relations goal_habit_relations_goal_id_habit_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goal_habit_relations
    ADD CONSTRAINT goal_habit_relations_goal_id_habit_id_key UNIQUE (goal_id, habit_id);

--
-- Name: goal_habit_relations goal_habit_relations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goal_habit_relations
    ADD CONSTRAINT goal_habit_relations_pkey PRIMARY KEY (goal_id, habit_id);

--
-- Name: goals goals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goals
    ADD CONSTRAINT goals_pkey PRIMARY KEY (id);

--
-- Name: habits habits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.habits
    ADD CONSTRAINT habits_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);

--
-- Name: plan_adjustment_suggestions plan_adjustment_suggestions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plan_adjustment_suggestions
    ADD CONSTRAINT plan_adjustment_suggestions_pkey PRIMARY KEY (id);

--
-- Name: plan_adjustment_suggestions plan_adjustment_suggestions_unique_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plan_adjustment_suggestions
    ADD CONSTRAINT plan_adjustment_suggestions_unique_key UNIQUE (user_id, source, week_start, target_id, adjustment_type);

--
-- Name: plans plans_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plans
    ADD CONSTRAINT plans_code_key UNIQUE (code);

--
-- Name: plans plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plans
    ADD CONSTRAINT plans_pkey PRIMARY KEY (id);

--
-- Name: processed_events processed_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.processed_events
    ADD CONSTRAINT processed_events_pkey PRIMARY KEY (event_id);

--
-- Name: profiles profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profiles
    ADD CONSTRAINT profiles_pkey PRIMARY KEY (id);

--
-- Name: profiles profiles_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profiles
    ADD CONSTRAINT profiles_user_id_key UNIQUE (user_id);

--
-- Name: reminder_queue reminder_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reminder_queue
    ADD CONSTRAINT reminder_queue_pkey PRIMARY KEY (id);

--
-- Name: saved_articles saved_articles_article_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_articles
    ADD CONSTRAINT saved_articles_article_id_user_id_key UNIQUE (article_id, user_id);

--
-- Name: saved_articles saved_articles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_articles
    ADD CONSTRAINT saved_articles_pkey PRIMARY KEY (id);

--
-- Name: saved_goals saved_goals_goal_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_goals
    ADD CONSTRAINT saved_goals_goal_id_user_id_key UNIQUE (goal_id, user_id);

--
-- Name: saved_goals saved_goals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_goals
    ADD CONSTRAINT saved_goals_pkey PRIMARY KEY (id);

--
-- Name: saved_habits saved_habits_habit_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_habits
    ADD CONSTRAINT saved_habits_habit_id_user_id_key UNIQUE (habit_id, user_id);

--
-- Name: saved_habits saved_habits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_habits
    ADD CONSTRAINT saved_habits_pkey PRIMARY KEY (id);

--
-- Name: saved_items saved_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_items
    ADD CONSTRAINT saved_items_pkey PRIMARY KEY (id);

--
-- Name: saved_items saved_items_user_id_item_type_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_items
    ADD CONSTRAINT saved_items_user_id_item_type_item_id_key UNIQUE (user_id, item_type, item_id);

--
-- Name: article_shares unique_article_share_per_platform; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.article_shares
    ADD CONSTRAINT unique_article_share_per_platform UNIQUE (article_id, user_id, platform);

--
-- Name: upgrade_events upgrade_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upgrade_events
    ADD CONSTRAINT upgrade_events_pkey PRIMARY KEY (id);

--
-- Name: user_coaching_profiles user_coaching_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_coaching_profiles
    ADD CONSTRAINT user_coaching_profiles_pkey PRIMARY KEY (id);

--
-- Name: user_coaching_profiles user_coaching_profiles_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_coaching_profiles
    ADD CONSTRAINT user_coaching_profiles_user_id_key UNIQUE (user_id);

--
-- Name: user_settings user_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_settings
    ADD CONSTRAINT user_settings_pkey PRIMARY KEY (id);

--
-- Name: user_settings user_settings_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_settings
    ADD CONSTRAINT user_settings_user_id_key UNIQUE (user_id);

--
-- Name: user_subscriptions user_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT user_subscriptions_pkey PRIMARY KEY (id);

--
-- Name: user_subscriptions user_subscriptions_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT user_subscriptions_user_id_key UNIQUE (user_id);

--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);

--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);

--
-- Name: weekly_reviews weekly_reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.weekly_reviews
    ADD CONSTRAINT weekly_reviews_pkey PRIMARY KEY (id);

--
-- Name: weekly_reviews weekly_reviews_user_id_week_start_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.weekly_reviews
    ADD CONSTRAINT weekly_reviews_user_id_week_start_key UNIQUE (user_id, week_start);

--
-- Name: idx_activities_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activities_created_at ON public.activities USING btree (created_at);

--
-- Name: idx_activities_item_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activities_item_type ON public.activities USING btree (item_type);

--
-- Name: idx_activities_user_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activities_user_created_at ON public.activities USING btree (user_id, created_at);

--
-- Name: idx_activities_user_created_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activities_user_created_desc ON public.activities USING btree (user_id, created_at DESC);

--
-- Name: idx_activities_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activities_user_date ON public.activities USING btree (user_id, created_at);

--
-- Name: idx_activities_user_type_created_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_activities_user_type_created_desc ON public.activities USING btree (user_id, item_type, created_at DESC);

--
-- Name: idx_ai_coach_processed_events_processed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_coach_processed_events_processed_at ON public.ai_coach_processed_events USING btree (processed_at);

--
-- Name: idx_ai_feedback_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_feedback_user_created ON public.ai_feedback USING btree (user_id, created_at DESC);

--
-- Name: idx_article_shares_article_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_article_shares_article_id ON public.article_shares USING btree (article_id);

--
-- Name: idx_article_shares_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_article_shares_user_id ON public.article_shares USING btree (user_id);

--
-- Name: idx_articles_ai_metadata; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_ai_metadata ON public.articles USING gin (ai_metadata);

--
-- Name: idx_articles_author; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_author ON public.articles USING btree (author);

--
-- Name: idx_articles_author_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_author_trgm ON public.articles USING gin (author public.gin_trgm_ops);

--
-- Name: idx_articles_category_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_category_id ON public.articles USING btree (category_id);

--
-- Name: idx_articles_category_published; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_category_published ON public.articles USING btree (category_id, published_at DESC);

--
-- Name: idx_articles_published_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_published_at ON public.articles USING btree (published_at);

--
-- Name: idx_articles_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_search ON public.articles USING gin (search_vector);

--
-- Name: idx_articles_title_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_articles_title_trgm ON public.articles USING gin (title public.gin_trgm_ops);

--
-- Name: idx_categories_entity_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_categories_entity_type ON public.categories USING btree (entity_type);

--
-- Name: idx_categories_name_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_categories_name_trgm ON public.categories USING gin (name public.gin_trgm_ops);

--
-- Name: idx_categories_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_categories_slug ON public.categories USING btree (slug);

--
-- Name: idx_check_ins_habit_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_habit_date ON public.check_ins USING btree (habit_id, created_at);

--
-- Name: idx_check_ins_habit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_habit_id ON public.check_ins USING btree (habit_id);

--
-- Name: idx_check_ins_missed_blocker; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_missed_blocker ON public.check_ins USING btree (user_id, created_at) WHERE ((status = 'missed'::public.check_in_status) AND (blocker IS NOT NULL));

--
-- Name: idx_check_ins_user_completed_local; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_user_completed_local ON public.check_ins USING btree (user_id, local_date) WHERE (status = 'completed'::public.check_in_status);

--
-- Name: idx_check_ins_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_user_date ON public.check_ins USING btree (user_id, created_at);

--
-- Name: idx_check_ins_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_user_id ON public.check_ins USING btree (user_id);

--
-- Name: idx_check_ins_user_local_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_user_local_date ON public.check_ins USING btree (user_id, local_date);

--
-- Name: idx_check_ins_user_local_date_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_check_ins_user_local_date_status ON public.check_ins USING btree (user_id, local_date, status);

--
-- Name: idx_coaching_blockers; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_coaching_blockers ON public.user_coaching_profiles USING gin (common_blockers);

--
-- Name: idx_coaching_notes; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_coaching_notes ON public.user_coaching_profiles USING gin (coaching_notes);


--
-- Name: idx_goal_habit_relations_habit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goal_habit_relations_habit_id ON public.goal_habit_relations USING btree (habit_id);

--
-- Name: idx_goals_active_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goals_active_status ON public.goals USING btree (user_id) WHERE (status <> 'completed'::public.goal_status_type);

--
-- Name: idx_goals_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goals_category ON public.goals USING btree (category);

--
-- Name: idx_goals_due_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goals_due_date ON public.goals USING btree (due_date);

--
-- Name: idx_goals_title_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goals_title_trgm ON public.goals USING gin (title public.gin_trgm_ops);

--
-- Name: idx_goals_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goals_user_id ON public.goals USING btree (user_id);

--
-- Name: idx_goals_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goals_version ON public.goals USING btree (id, version);

--
-- Name: idx_habits_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_habits_category ON public.habits USING btree (category);

--
-- Name: idx_habits_name_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_habits_name_trgm ON public.habits USING gin (name public.gin_trgm_ops);

--
-- Name: idx_habits_not_completed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_habits_not_completed ON public.habits USING btree (user_id) WHERE (completed = false);

--
-- Name: idx_habits_user_completed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_habits_user_completed ON public.habits USING btree (user_id) WHERE (completed = true);

--
-- Name: idx_habits_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_habits_user_id ON public.habits USING btree (user_id);

--
-- Name: idx_habits_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_habits_version ON public.habits USING btree (id, version);


--
-- Name: idx_notifications_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_created_at ON public.notifications USING btree (created_at);

--
-- Name: idx_notifications_item_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_item_type ON public.notifications USING btree (item_type);

--
-- Name: idx_notifications_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_unread ON public.notifications USING btree (user_id, created_at) WHERE (is_read = false);

--
-- Name: idx_notifications_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_created ON public.notifications USING btree (user_id, created_at DESC);

--
-- Name: idx_notifications_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_id ON public.notifications USING btree (user_id);

--
-- Name: idx_notifications_user_unread; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_user_unread ON public.notifications USING btree (user_id) WHERE (is_read = false);

--
-- Name: idx_plan_adjustment_suggestions_goal_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_adjustment_suggestions_goal_id ON public.plan_adjustment_suggestions USING btree (goal_id);

--
-- Name: idx_plan_adjustment_suggestions_habit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_adjustment_suggestions_habit_id ON public.plan_adjustment_suggestions USING btree (habit_id);

--
-- Name: idx_plan_adjustment_suggestions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_adjustment_suggestions_status ON public.plan_adjustment_suggestions USING btree (status);

--
-- Name: idx_plan_adjustment_suggestions_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_adjustment_suggestions_user_id ON public.plan_adjustment_suggestions USING btree (user_id);

--
-- Name: idx_plan_adjustment_suggestions_user_status_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_adjustment_suggestions_user_status_created ON public.plan_adjustment_suggestions USING btree (user_id, status, created_at DESC);

--
-- Name: idx_plan_adjustment_suggestions_week_start; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plan_adjustment_suggestions_week_start ON public.plan_adjustment_suggestions USING btree (week_start);

--
-- Name: idx_plans_is_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_plans_is_active ON public.plans USING btree (is_active);

--
-- Name: idx_processed_events_processed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_processed_events_processed_at ON public.processed_events USING btree (processed_at);

--
-- Name: idx_reminder_queue_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminder_queue_due ON public.reminder_queue USING btree (scheduled_at) WHERE (sent = false);

--
-- Name: idx_reminder_queue_due_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reminder_queue_due_id ON public.reminder_queue USING btree (scheduled_at, id) WHERE (sent = false);

--
-- Name: idx_saved_articles_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_articles_created_at ON public.saved_articles USING btree (created_at DESC);

--
-- Name: idx_saved_articles_user_created_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_articles_user_created_desc ON public.saved_articles USING btree (user_id, created_at DESC);

--
-- Name: idx_saved_articles_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_articles_user_id ON public.saved_articles USING btree (user_id);

--
-- Name: idx_saved_goals_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_goals_created_at ON public.saved_goals USING btree (created_at DESC);

--
-- Name: idx_saved_goals_user_created_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_goals_user_created_desc ON public.saved_goals USING btree (user_id, created_at DESC);

--
-- Name: idx_saved_goals_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_goals_user_id ON public.saved_goals USING btree (user_id);

--
-- Name: idx_saved_habits_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_habits_created_at ON public.saved_habits USING btree (created_at DESC);

--
-- Name: idx_saved_habits_user_created_desc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_habits_user_created_desc ON public.saved_habits USING btree (user_id, created_at DESC);

--
-- Name: idx_saved_habits_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_habits_user_id ON public.saved_habits USING btree (user_id);

--
-- Name: idx_saved_items_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_items_created_at ON public.saved_items USING btree (created_at);

--
-- Name: idx_saved_items_item_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_items_item_type ON public.saved_items USING btree (item_type);

--
-- Name: idx_saved_items_item_type_item_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_items_item_type_item_id ON public.saved_items USING btree (item_type, item_id);

--
-- Name: idx_saved_items_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_items_user_id ON public.saved_items USING btree (user_id);

--
-- Name: idx_saved_items_user_type_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_saved_items_user_type_created ON public.saved_items USING btree (user_id, item_type, created_at DESC);

--
-- Name: idx_upgrade_events_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upgrade_events_created_at ON public.upgrade_events USING btree (created_at DESC);

--
-- Name: idx_upgrade_events_event_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upgrade_events_event_type ON public.upgrade_events USING btree (event_type);

--
-- Name: idx_upgrade_events_surface; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upgrade_events_surface ON public.upgrade_events USING btree (surface);

--
-- Name: idx_upgrade_events_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upgrade_events_user_created ON public.upgrade_events USING btree (user_id, created_at DESC);

--
-- Name: idx_upgrade_events_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_upgrade_events_user_id ON public.upgrade_events USING btree (user_id);

--
-- Name: idx_user_settings_user_id_covering; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_settings_user_id_covering ON public.user_settings USING btree (user_id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, accountability_style, check_in_time, onboarding_completed, created_at, updated_at, version);

--
-- Name: idx_user_settings_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_settings_version ON public.user_settings USING btree (user_id, version);

--
-- Name: idx_user_subscriptions_plan_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_subscriptions_plan_id ON public.user_subscriptions USING btree (plan_id);

--
-- Name: idx_user_subscriptions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_subscriptions_status ON public.user_subscriptions USING btree (status);

--
-- Name: idx_user_subscriptions_stripe_customer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_subscriptions_stripe_customer_id ON public.user_subscriptions USING btree (stripe_customer_id);

--
-- Name: idx_user_subscriptions_stripe_subscription_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_subscriptions_stripe_subscription_id ON public.user_subscriptions USING btree (stripe_subscription_id);

--
-- Name: idx_weekly_reviews_energy; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_weekly_reviews_energy ON public.weekly_reviews USING gin (energy_summary);

--
-- Name: idx_weekly_reviews_mood; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_weekly_reviews_mood ON public.weekly_reviews USING gin (mood_summary);

--
-- Name: idx_weekly_reviews_user_week; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_weekly_reviews_user_week ON public.weekly_reviews USING btree (user_id, week_start DESC);

--
-- Name: uniq_check_ins_user_habit_local_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_check_ins_user_habit_local_date ON public.check_ins USING btree (user_id, habit_id, local_date);

--
-- Name: uniq_reminder_queue_pending_per_day; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uniq_reminder_queue_pending_per_day ON public.reminder_queue USING btree (user_id, type, (((scheduled_at AT TIME ZONE 'UTC'::text))::date)) WHERE (sent = false);

--
-- Name: check_ins check_ins_no_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER check_ins_no_update BEFORE UPDATE ON public.check_ins FOR EACH ROW EXECUTE FUNCTION public.prevent_check_in_update();

--
-- Name: articles cleanup_saved_items_articles; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cleanup_saved_items_articles BEFORE DELETE ON public.articles FOR EACH ROW EXECUTE FUNCTION public.cleanup_saved_items_on_article_delete();

--
-- Name: goals cleanup_saved_items_goals; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cleanup_saved_items_goals BEFORE DELETE ON public.goals FOR EACH ROW EXECUTE FUNCTION public.cleanup_saved_items_on_goal_delete();

--
-- Name: habits cleanup_saved_items_habits; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER cleanup_saved_items_habits BEFORE DELETE ON public.habits FOR EACH ROW EXECUTE FUNCTION public.cleanup_saved_items_on_habit_delete();

--
-- Name: goals goals_sync_completed; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER goals_sync_completed BEFORE INSERT OR UPDATE ON public.goals FOR EACH ROW EXECUTE FUNCTION public.sync_goals_completed_from_status();

--
-- Name: articles update_articles_search_vector; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_articles_search_vector BEFORE INSERT OR UPDATE ON public.articles FOR EACH ROW EXECUTE FUNCTION public.update_article_search_vector();

--
-- Name: articles update_articles_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_articles_updated_at BEFORE UPDATE ON public.articles FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: categories update_categories_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_categories_updated_at BEFORE UPDATE ON public.categories FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: goals update_goals_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_goals_updated_at BEFORE UPDATE ON public.goals FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: habits update_habits_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_habits_updated_at BEFORE UPDATE ON public.habits FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: plan_adjustment_suggestions update_plan_adjustment_suggestions_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_plan_adjustment_suggestions_updated_at BEFORE UPDATE ON public.plan_adjustment_suggestions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: plans update_plans_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_plans_updated_at BEFORE UPDATE ON public.plans FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: profiles update_profiles_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_profiles_updated_at BEFORE UPDATE ON public.profiles FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: reminder_queue update_reminder_queue_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_reminder_queue_updated_at BEFORE UPDATE ON public.reminder_queue FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: user_coaching_profiles update_user_coaching_profiles_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_user_coaching_profiles_updated_at BEFORE UPDATE ON public.user_coaching_profiles FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: user_settings update_user_settings_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_user_settings_updated_at BEFORE UPDATE ON public.user_settings FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: user_subscriptions update_user_subscriptions_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_user_subscriptions_updated_at BEFORE UPDATE ON public.user_subscriptions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: users update_users_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: weekly_reviews update_weekly_reviews_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_weekly_reviews_updated_at BEFORE UPDATE ON public.weekly_reviews FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

--
-- Name: saved_items validate_saved_item_before_insert; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER validate_saved_item_before_insert BEFORE INSERT OR UPDATE ON public.saved_items FOR EACH ROW EXECUTE FUNCTION public.validate_saved_item_reference();

--
-- Name: activities activities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activities
    ADD CONSTRAINT activities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: ai_feedback ai_feedback_check_in_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_feedback
    ADD CONSTRAINT ai_feedback_check_in_id_fkey FOREIGN KEY (check_in_id) REFERENCES public.check_ins(id) ON DELETE CASCADE;

--
-- Name: ai_feedback ai_feedback_habit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_feedback
    ADD CONSTRAINT ai_feedback_habit_id_fkey FOREIGN KEY (habit_id) REFERENCES public.habits(id) ON DELETE CASCADE;

--
-- Name: ai_feedback ai_feedback_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_feedback
    ADD CONSTRAINT ai_feedback_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: article_shares article_shares_article_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.article_shares
    ADD CONSTRAINT article_shares_article_id_fkey FOREIGN KEY (article_id) REFERENCES public.articles(id) ON DELETE CASCADE;

--
-- Name: article_shares article_shares_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.article_shares
    ADD CONSTRAINT article_shares_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: check_ins check_ins_habit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.check_ins
    ADD CONSTRAINT check_ins_habit_id_fkey FOREIGN KEY (habit_id) REFERENCES public.habits(id) ON DELETE CASCADE;

--
-- Name: check_ins check_ins_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.check_ins
    ADD CONSTRAINT check_ins_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: articles fk_articles_category; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.articles
    ADD CONSTRAINT fk_articles_category FOREIGN KEY (category_id) REFERENCES public.categories(id) ON DELETE SET NULL;

--
-- Name: goal_habit_relations goal_habit_relations_goal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goal_habit_relations
    ADD CONSTRAINT goal_habit_relations_goal_id_fkey FOREIGN KEY (goal_id) REFERENCES public.goals(id) ON DELETE CASCADE;

--
-- Name: goal_habit_relations goal_habit_relations_habit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goal_habit_relations
    ADD CONSTRAINT goal_habit_relations_habit_id_fkey FOREIGN KEY (habit_id) REFERENCES public.habits(id) ON DELETE CASCADE;

--
-- Name: goals goals_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goals
    ADD CONSTRAINT goals_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: habits habits_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.habits
    ADD CONSTRAINT habits_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: plan_adjustment_suggestions plan_adjustment_suggestions_goal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plan_adjustment_suggestions
    ADD CONSTRAINT plan_adjustment_suggestions_goal_id_fkey FOREIGN KEY (goal_id) REFERENCES public.goals(id) ON DELETE CASCADE;

--
-- Name: plan_adjustment_suggestions plan_adjustment_suggestions_habit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plan_adjustment_suggestions
    ADD CONSTRAINT plan_adjustment_suggestions_habit_id_fkey FOREIGN KEY (habit_id) REFERENCES public.habits(id) ON DELETE CASCADE;

--
-- Name: plan_adjustment_suggestions plan_adjustment_suggestions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plan_adjustment_suggestions
    ADD CONSTRAINT plan_adjustment_suggestions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: profiles profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profiles
    ADD CONSTRAINT profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: reminder_queue reminder_queue_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reminder_queue
    ADD CONSTRAINT reminder_queue_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: saved_articles saved_articles_article_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_articles
    ADD CONSTRAINT saved_articles_article_id_fkey FOREIGN KEY (article_id) REFERENCES public.articles(id) ON DELETE CASCADE;

--
-- Name: saved_articles saved_articles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_articles
    ADD CONSTRAINT saved_articles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: saved_goals saved_goals_goal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_goals
    ADD CONSTRAINT saved_goals_goal_id_fkey FOREIGN KEY (goal_id) REFERENCES public.goals(id) ON DELETE CASCADE;

--
-- Name: saved_goals saved_goals_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_goals
    ADD CONSTRAINT saved_goals_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: saved_habits saved_habits_habit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_habits
    ADD CONSTRAINT saved_habits_habit_id_fkey FOREIGN KEY (habit_id) REFERENCES public.habits(id) ON DELETE CASCADE;

--
-- Name: saved_habits saved_habits_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_habits
    ADD CONSTRAINT saved_habits_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: saved_items saved_items_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.saved_items
    ADD CONSTRAINT saved_items_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: upgrade_events upgrade_events_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upgrade_events
    ADD CONSTRAINT upgrade_events_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.plans(id) ON DELETE SET NULL;

--
-- Name: upgrade_events upgrade_events_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upgrade_events
    ADD CONSTRAINT upgrade_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: user_coaching_profiles user_coaching_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_coaching_profiles
    ADD CONSTRAINT user_coaching_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: user_settings user_settings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_settings
    ADD CONSTRAINT user_settings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: user_subscriptions user_subscriptions_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT user_subscriptions_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.plans(id) ON DELETE RESTRICT;

--
-- Name: user_subscriptions user_subscriptions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_subscriptions
    ADD CONSTRAINT user_subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: weekly_reviews weekly_reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.weekly_reviews
    ADD CONSTRAINT weekly_reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

--
-- Name: activities; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.activities ENABLE ROW LEVEL SECURITY;

--
-- Name: activities activities_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY activities_user_isolation ON public.activities USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: ai_feedback; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.ai_feedback ENABLE ROW LEVEL SECURITY;

--
-- Name: ai_feedback ai_feedback_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY ai_feedback_user_isolation ON public.ai_feedback USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: article_shares; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.article_shares ENABLE ROW LEVEL SECURITY;

--
-- Name: article_shares article_shares_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY article_shares_user_isolation ON public.article_shares USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: check_ins; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.check_ins ENABLE ROW LEVEL SECURITY;

--
-- Name: check_ins check_ins_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY check_ins_user_isolation ON public.check_ins USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));


--
-- Name: goal_habit_relations; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.goal_habit_relations ENABLE ROW LEVEL SECURITY;

--
-- Name: goal_habit_relations goal_habit_relations_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY goal_habit_relations_user_isolation ON public.goal_habit_relations USING ((goal_id IN ( SELECT goals.id
   FROM public.goals
  WHERE (goals.user_id = public.current_app_user_id())))) WITH CHECK ((goal_id IN ( SELECT goals.id
   FROM public.goals
  WHERE (goals.user_id = public.current_app_user_id()))));

--
-- Name: goals; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.goals ENABLE ROW LEVEL SECURITY;

--
-- Name: goals goals_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY goals_user_isolation ON public.goals USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: habits; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.habits ENABLE ROW LEVEL SECURITY;

--
-- Name: habits habits_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY habits_user_isolation ON public.habits USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));


--
-- Name: notifications; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.notifications ENABLE ROW LEVEL SECURITY;

--
-- Name: notifications notifications_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY notifications_user_isolation ON public.notifications USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: plan_adjustment_suggestions; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.plan_adjustment_suggestions ENABLE ROW LEVEL SECURITY;

--
-- Name: plan_adjustment_suggestions plan_adjustment_suggestions_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY plan_adjustment_suggestions_user_isolation ON public.plan_adjustment_suggestions USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: profiles; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;

--
-- Name: profiles profiles_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY profiles_user_isolation ON public.profiles USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: reminder_queue; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.reminder_queue ENABLE ROW LEVEL SECURITY;

--
-- Name: reminder_queue reminder_queue_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY reminder_queue_user_isolation ON public.reminder_queue USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: saved_articles; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.saved_articles ENABLE ROW LEVEL SECURITY;

--
-- Name: saved_articles saved_articles_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY saved_articles_user_isolation ON public.saved_articles USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: saved_goals; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.saved_goals ENABLE ROW LEVEL SECURITY;

--
-- Name: saved_goals saved_goals_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY saved_goals_user_isolation ON public.saved_goals USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: saved_habits; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.saved_habits ENABLE ROW LEVEL SECURITY;

--
-- Name: saved_habits saved_habits_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY saved_habits_user_isolation ON public.saved_habits USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: upgrade_events; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.upgrade_events ENABLE ROW LEVEL SECURITY;

--
-- Name: upgrade_events upgrade_events_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY upgrade_events_user_isolation ON public.upgrade_events USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: user_coaching_profiles; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.user_coaching_profiles ENABLE ROW LEVEL SECURITY;

--
-- Name: user_coaching_profiles user_coaching_profiles_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY user_coaching_profiles_user_isolation ON public.user_coaching_profiles USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: user_settings; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.user_settings ENABLE ROW LEVEL SECURITY;

--
-- Name: user_settings user_settings_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY user_settings_user_isolation ON public.user_settings USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: user_subscriptions; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.user_subscriptions ENABLE ROW LEVEL SECURITY;

--
-- Name: user_subscriptions user_subscriptions_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY user_subscriptions_user_isolation ON public.user_subscriptions USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));

--
-- Name: weekly_reviews; Type: ROW SECURITY; Schema: public; Owner: -
--

ALTER TABLE public.weekly_reviews ENABLE ROW LEVEL SECURITY;

--
-- Name: weekly_reviews weekly_reviews_user_isolation; Type: POLICY; Schema: public; Owner: -
--

CREATE POLICY weekly_reviews_user_isolation ON public.weekly_reviews USING ((user_id = public.current_app_user_id())) WITH CHECK ((user_id = public.current_app_user_id()));


