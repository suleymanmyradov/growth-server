DROP TRIGGER IF EXISTS categories_search_sync ON public.categories;
DROP TRIGGER IF EXISTS habits_search_sync ON public.habits;
DROP TRIGGER IF EXISTS goals_search_sync ON public.goals;
DROP TRIGGER IF EXISTS articles_search_sync ON public.articles;

DROP FUNCTION IF EXISTS public.enqueue_search_category_articles_sync();
DROP FUNCTION IF EXISTS public.enqueue_search_sync_event();

DROP INDEX IF EXISTS uniq_search_outbox_pending_entity;
DROP INDEX IF EXISTS idx_search_outbox_entity;
DROP INDEX IF EXISTS idx_search_outbox_pending;

DROP TABLE IF EXISTS public.search_outbox;

DROP TYPE IF EXISTS public.search_event_operation;
