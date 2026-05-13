-- Remove saved_items integrity triggers and functions

DROP INDEX IF EXISTS idx_saved_items_item_type_item_id;

DROP TRIGGER IF EXISTS cleanup_saved_items_habits ON habits;
DROP TRIGGER IF EXISTS cleanup_saved_items_goals ON goals;
DROP TRIGGER IF EXISTS cleanup_saved_items_articles ON articles;

DROP FUNCTION IF EXISTS cleanup_saved_items_on_habit_delete();
DROP FUNCTION IF EXISTS cleanup_saved_items_on_goal_delete();
DROP FUNCTION IF EXISTS cleanup_saved_items_on_article_delete();

DROP TRIGGER IF EXISTS validate_saved_item_before_insert ON saved_items;
DROP FUNCTION IF EXISTS validate_saved_item_reference();
