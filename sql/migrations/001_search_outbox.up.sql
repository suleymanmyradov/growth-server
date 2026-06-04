CREATE TYPE public.search_event_operation AS ENUM (
    'upsert',
    'delete'
);

CREATE TABLE public.search_outbox (
    id uuid DEFAULT public.uuid_generate_v7() NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    operation public.search_event_operation NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'pending' NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    available_at timestamptz DEFAULT now() NOT NULL,
    locked_at timestamptz,
    locked_by text,
    processed_at timestamptz,
    error text,
    created_at timestamptz DEFAULT now() NOT NULL,
    CONSTRAINT search_outbox_status_check CHECK (status IN ('pending', 'processing', 'processed', 'failed'))
);

CREATE INDEX idx_search_outbox_pending
ON public.search_outbox (status, available_at, created_at)
WHERE status IN ('pending', 'failed');

CREATE INDEX idx_search_outbox_entity
ON public.search_outbox (entity_type, entity_id);

CREATE UNIQUE INDEX uniq_search_outbox_pending_entity
ON public.search_outbox (entity_type, entity_id)
WHERE status IN ('pending', 'failed');

CREATE OR REPLACE FUNCTION public.enqueue_search_sync_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_entity_type text := TG_ARGV[0];
    v_entity_id uuid;
    v_operation public.search_event_operation;
BEGIN
    IF TG_OP = 'DELETE' THEN
        v_entity_id := OLD.id;
        v_operation := 'delete';
    ELSE
        v_entity_id := NEW.id;
        v_operation := 'upsert';
    END IF;

    INSERT INTO public.search_outbox (
        entity_type,
        entity_id,
        operation,
        payload,
        status,
        attempts,
        available_at
    )
    VALUES (
        v_entity_type,
        v_entity_id,
        v_operation,
        jsonb_build_object('table', TG_TABLE_NAME, 'op', TG_OP),
        'pending',
        0,
        now()
    )
    ON CONFLICT (entity_type, entity_id)
    WHERE status IN ('pending', 'failed')
    DO UPDATE SET
        operation = CASE
            WHEN EXCLUDED.operation = 'delete' THEN 'delete'::public.search_event_operation
            ELSE public.search_outbox.operation
        END,
        payload = EXCLUDED.payload,
        status = 'pending',
        available_at = now(),
        attempts = 0,
        error = NULL,
        created_at = now();

    PERFORM pg_notify('search_sync', json_build_object(
        'entity_type', v_entity_type,
        'entity_id', v_entity_id,
        'operation', v_operation
    )::text);

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.enqueue_search_category_articles_sync()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO public.search_outbox (
        entity_type,
        entity_id,
        operation,
        payload,
        status,
        attempts,
        available_at
    )
    SELECT
        'article',
        a.id,
        'upsert',
        jsonb_build_object('table', 'articles', 'op', 'UPDATE', 'reason', 'category_changed'),
        'pending',
        0,
        now()
    FROM public.articles a
    WHERE a.category_id = NEW.id
    ON CONFLICT (entity_type, entity_id)
    WHERE status IN ('pending', 'failed')
    DO UPDATE SET
        operation = 'upsert',
        payload = EXCLUDED.payload,
        status = 'pending',
        available_at = now(),
        attempts = 0,
        error = NULL,
        created_at = now();

    PERFORM pg_notify('search_sync', json_build_object(
        'entity_type', 'article',
        'reason', 'category_changed',
        'category_id', NEW.id
    )::text);

    RETURN NEW;
END;
$$;

CREATE TRIGGER articles_search_sync
AFTER INSERT OR UPDATE OF title, excerpt, content, author, category_id, published_at, updated_at
OR DELETE ON public.articles
FOR EACH ROW EXECUTE FUNCTION public.enqueue_search_sync_event('article');

CREATE TRIGGER goals_search_sync
AFTER INSERT OR UPDATE OF title, description, category, status, updated_at
OR DELETE ON public.goals
FOR EACH ROW EXECUTE FUNCTION public.enqueue_search_sync_event('goal');

CREATE TRIGGER habits_search_sync
AFTER INSERT OR UPDATE OF name, description, category, updated_at
OR DELETE ON public.habits
FOR EACH ROW EXECUTE FUNCTION public.enqueue_search_sync_event('habit');

CREATE TRIGGER categories_search_sync
AFTER UPDATE OF name, slug
ON public.categories
FOR EACH ROW EXECUTE FUNCTION public.enqueue_search_category_articles_sync();
