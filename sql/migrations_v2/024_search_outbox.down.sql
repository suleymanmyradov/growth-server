DROP TRIGGER IF EXISTS categories_search_sync ON categories;
DROP TRIGGER IF EXISTS habits_search_sync ON habits;
DROP TRIGGER IF EXISTS goals_search_sync ON goals;
DROP TRIGGER IF EXISTS articles_search_sync ON articles;
DROP FUNCTION IF EXISTS enqueue_category_articles_search_event();
DROP FUNCTION IF EXISTS enqueue_search_event();
DROP TABLE IF EXISTS search_outbox;
