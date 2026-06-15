-- Insert-only outbox feeding the search-sync service.
-- Worker contract (visibility-timeout queue):
--   claim:   bump attempts and push available_at into the future for a batch
--            (FOR UPDATE SKIP LOCKED keeps concurrent workers apart)
--   success: DELETE the row
--   failure: set last_error and available_at = retry time
-- A crashed worker needs no cleanup: rows reappear when the timeout passes.
CREATE TABLE search_outbox (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    entity_type  text NOT NULL CHECK (entity_type IN ('article', 'goal', 'habit')),
    entity_id    uuid NOT NULL,
    operation    text NOT NULL CHECK (operation IN ('upsert', 'delete')),
    attempts     integer NOT NULL DEFAULT 0,
    last_error   text,
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_search_outbox_available ON search_outbox (available_at);

-- Enqueue a search sync event for the changed row.
-- TG_ARGV[0] is the entity type ('article' | 'goal' | 'habit').
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

-- A category rename changes the search documents of all its articles.
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
