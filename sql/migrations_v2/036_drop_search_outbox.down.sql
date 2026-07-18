-- Reverse migration 036: restore the search_outbox table and outbox-based
-- triggers, and drop the notify-based trigger functions.
--
-- This down migration restores the state from migrations 024 + 032.

-- ---------------------------------------------------------------------------
-- Restore the outbox table (from migration 024)
-- ---------------------------------------------------------------------------
CREATE TABLE search_outbox (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type  text NOT NULL CHECK (entity_type IN (
        'article', 'goal', 'habit',
        'check_in', 'conversation_message', 'weekly_review'
    )),
    entity_id    uuid NOT NULL,
    operation    text NOT NULL CHECK (operation IN ('upsert', 'delete')),
    attempts     integer NOT NULL DEFAULT 0,
    last_error   text,
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_search_outbox_available ON search_outbox (available_at);

-- ---------------------------------------------------------------------------
-- Restore outbox trigger functions (from migration 024)
-- ---------------------------------------------------------------------------
CREATE FUNCTION enqueue_search_event() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        INSERT INTO search_outbox (entity_type, entity_id, operation)
        VALUES (TG_ARGV[0], OLD.id, 'delete');
        RETURN OLD;
    END IF;

    INSERT INTO search_outbox (entity_type, entity_id, operation)
    VALUES (TG_ARGV[0], NEW.id, 'upsert');
    RETURN NEW;
END;
$$;

CREATE FUNCTION enqueue_category_articles_search_event() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO search_outbox (entity_type, entity_id, operation)
    SELECT 'article', a.id, 'upsert'
    FROM articles a
    WHERE a.category_id = NEW.id;
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Restore triggers (from migrations 024 + 032)
-- ---------------------------------------------------------------------------

-- Drop notify-based triggers
DROP TRIGGER IF EXISTS articles_search_sync ON articles;
DROP TRIGGER IF EXISTS goals_search_sync ON goals;
DROP TRIGGER IF EXISTS habits_search_sync ON habits;
DROP TRIGGER IF EXISTS categories_search_sync ON categories;
DROP TRIGGER IF EXISTS check_ins_search_sync_insert ON check_ins;
DROP TRIGGER IF EXISTS check_ins_search_sync_delete ON check_ins;
DROP TRIGGER IF EXISTS conversation_messages_search_sync ON conversation_messages;
DROP TRIGGER IF EXISTS weekly_reviews_search_sync_upsert ON weekly_reviews;
DROP TRIGGER IF EXISTS weekly_reviews_search_sync_delete ON weekly_reviews;

-- Restore outbox-based triggers
CREATE TRIGGER articles_search_sync
    AFTER INSERT OR UPDATE OR DELETE ON articles
    FOR EACH ROW EXECUTE FUNCTION enqueue_search_event('article');

CREATE TRIGGER goals_search_sync
    AFTER INSERT OR UPDATE OR DELETE ON goals
    FOR EACH ROW EXECUTE FUNCTION enqueue_search_event('goal');

CREATE TRIGGER habits_search_sync
    AFTER INSERT OR UPDATE OR DELETE ON habits
    FOR EACH ROW EXECUTE FUNCTION enqueue_search_event('habit');

CREATE TRIGGER categories_search_sync
    AFTER UPDATE OF name, slug ON categories
    FOR EACH ROW EXECUTE FUNCTION enqueue_category_articles_search_event();

CREATE TRIGGER check_ins_search_sync_insert
    AFTER INSERT ON check_ins
    FOR EACH ROW
    WHEN (NEW.note IS NOT NULL AND NEW.note <> '')
    EXECUTE FUNCTION enqueue_search_event('check_in');

CREATE TRIGGER check_ins_search_sync_delete
    AFTER DELETE ON check_ins
    FOR EACH ROW EXECUTE FUNCTION enqueue_search_event('check_in');

CREATE TRIGGER conversation_messages_search_sync
    AFTER INSERT OR DELETE ON conversation_messages
    FOR EACH ROW EXECUTE FUNCTION enqueue_search_event('conversation_message');

CREATE TRIGGER weekly_reviews_search_sync_upsert
    AFTER INSERT OR UPDATE OF ai_summary ON weekly_reviews
    FOR EACH ROW
    WHEN (NEW.ai_summary IS NOT NULL AND NEW.ai_summary <> '')
    EXECUTE FUNCTION enqueue_search_event('weekly_review');

CREATE TRIGGER weekly_reviews_search_sync_delete
    AFTER DELETE ON weekly_reviews
    FOR EACH ROW EXECUTE FUNCTION enqueue_search_event('weekly_review');

-- Drop notify-based functions
DROP FUNCTION IF EXISTS notify_search_event();
DROP FUNCTION IF EXISTS notify_category_articles_search_event();
