-- Replace the search_outbox table with pg_notify.
--
-- The search-sync service now LISTENs on the 'search_sync' channel and
-- reconciles Meilisearch against Postgres periodically, so the durable
-- outbox table is no longer needed. Triggers fire pg_notify() with a JSON
-- payload: {"e":"<entity_type>","id":"<uuid>","op":"upsert"|"delete"}.
--
-- If search-sync is down when a notification fires, the event is lost — but
-- the periodic reconciliation loop (incremental every few minutes + full on
-- startup) catches up by comparing Postgres rows against Meilisearch docs.

-- ---------------------------------------------------------------------------
-- New trigger functions (notify-based)
-- ---------------------------------------------------------------------------

-- Replaces enqueue_search_event: fires a pg_notify for the changed row.
-- TG_ARGV[0] is the entity type ('article' | 'goal' | 'habit' | 'check_in' |
-- 'conversation_message' | 'weekly_review').
CREATE OR REPLACE FUNCTION notify_search_event() RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    payload json;
    op     text := CASE WHEN TG_OP = 'DELETE' THEN 'delete' ELSE 'upsert' END;
    row_id uuid;
BEGIN
    IF TG_OP = 'DELETE' THEN
        row_id := OLD.id;
    ELSE
        row_id := NEW.id;
    END IF;

    payload := json_build_object('e', TG_ARGV[0], 'id', row_id, 'op', op);
    PERFORM pg_notify('search_sync', payload::text);
    -- A trigger function must return the row (OLD for DELETE, NEW for
    -- INSERT/UPDATE), not the scalar id. Returning row_id (a uuid) raises
    -- "cannot return non-composite value from function returning composite type".
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

-- Replaces enqueue_category_articles_search_event: when a category name/slug
-- changes, notify for every article in that category so their denormalized
-- category fields are re-indexed.
CREATE OR REPLACE FUNCTION notify_category_articles_search_event() RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    a record;
    payload json;
BEGIN
    FOR a IN SELECT id FROM articles WHERE category_id = NEW.id LOOP
        payload := json_build_object('e', 'article', 'id', a.id, 'op', 'upsert');
        PERFORM pg_notify('search_sync', payload::text);
    END LOOP;
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Replace triggers: drop old, create new with the notify functions.
-- The WHEN clauses from migration 032 (check_ins/weekly_reviews guards) are
-- preserved so empty notes / empty summaries don't fire spurious notifications.
-- ---------------------------------------------------------------------------

-- articles
DROP TRIGGER IF EXISTS articles_search_sync ON articles;
CREATE TRIGGER articles_search_sync
    AFTER INSERT OR UPDATE OR DELETE ON articles
    FOR EACH ROW EXECUTE FUNCTION notify_search_event('article');

-- goals
DROP TRIGGER IF EXISTS goals_search_sync ON goals;
CREATE TRIGGER goals_search_sync
    AFTER INSERT OR UPDATE OR DELETE ON goals
    FOR EACH ROW EXECUTE FUNCTION notify_search_event('goal');

-- habits
DROP TRIGGER IF EXISTS habits_search_sync ON habits;
CREATE TRIGGER habits_search_sync
    AFTER INSERT OR UPDATE OR DELETE ON habits
    FOR EACH ROW EXECUTE FUNCTION notify_search_event('habit');

-- categories (fan-out to articles)
DROP TRIGGER IF EXISTS categories_search_sync ON categories;
CREATE TRIGGER categories_search_sync
    AFTER UPDATE OF name, slug ON categories
    FOR EACH ROW EXECUTE FUNCTION notify_category_articles_search_event();

-- check_ins: only rows with a non-empty note. Separate insert/delete triggers
-- because WHEN can't cover both NEW-bound and OLD-bound in one trigger.
DROP TRIGGER IF EXISTS check_ins_search_sync_insert ON check_ins;
CREATE TRIGGER check_ins_search_sync_insert
    AFTER INSERT ON check_ins
    FOR EACH ROW
    WHEN (NEW.note IS NOT NULL AND NEW.note <> '')
    EXECUTE FUNCTION notify_search_event('check_in');

DROP TRIGGER IF EXISTS check_ins_search_sync_delete ON check_ins;
CREATE TRIGGER check_ins_search_sync_delete
    AFTER DELETE ON check_ins
    FOR EACH ROW EXECUTE FUNCTION notify_search_event('check_in');

-- conversation_messages: content is NOT NULL, every row is indexable.
DROP TRIGGER IF EXISTS conversation_messages_search_sync ON conversation_messages;
CREATE TRIGGER conversation_messages_search_sync
    AFTER INSERT OR DELETE ON conversation_messages
    FOR EACH ROW EXECUTE FUNCTION notify_search_event('conversation_message');

-- weekly_reviews: only when ai_summary is non-empty.
DROP TRIGGER IF EXISTS weekly_reviews_search_sync_upsert ON weekly_reviews;
CREATE TRIGGER weekly_reviews_search_sync_upsert
    AFTER INSERT OR UPDATE OF ai_summary ON weekly_reviews
    FOR EACH ROW
    WHEN (NEW.ai_summary IS NOT NULL AND NEW.ai_summary <> '')
    EXECUTE FUNCTION notify_search_event('weekly_review');

DROP TRIGGER IF EXISTS weekly_reviews_search_sync_delete ON weekly_reviews;
CREATE TRIGGER weekly_reviews_search_sync_delete
    AFTER DELETE ON weekly_reviews
    FOR EACH ROW EXECUTE FUNCTION notify_search_event('weekly_review');

-- ---------------------------------------------------------------------------
-- Drop the old outbox infrastructure
-- ---------------------------------------------------------------------------

DROP FUNCTION IF EXISTS enqueue_search_event();
DROP FUNCTION IF EXISTS enqueue_category_articles_search_event();
DROP TABLE IF EXISTS search_outbox;
