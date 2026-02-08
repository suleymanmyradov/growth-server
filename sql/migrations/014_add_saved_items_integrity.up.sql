-- Add referential integrity for saved_items using trigger-based validation
-- This ensures item_id references a valid entity based on item_type

CREATE OR REPLACE FUNCTION validate_saved_item_reference()
RETURNS TRIGGER AS $$
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
$$ LANGUAGE plpgsql;

CREATE TRIGGER validate_saved_item_before_insert
    BEFORE INSERT OR UPDATE ON saved_items
    FOR EACH ROW
    EXECUTE FUNCTION validate_saved_item_reference();

-- Create a function to clean up orphaned saved_items when referenced entities are deleted
CREATE OR REPLACE FUNCTION cleanup_saved_items_on_article_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM saved_items WHERE item_type = 'article' AND item_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION cleanup_saved_items_on_goal_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM saved_items WHERE item_type = 'goal' AND item_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION cleanup_saved_items_on_habit_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM saved_items WHERE item_type = 'habit' AND item_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Attach cleanup triggers to parent tables
CREATE TRIGGER cleanup_saved_items_articles
    BEFORE DELETE ON articles
    FOR EACH ROW
    EXECUTE FUNCTION cleanup_saved_items_on_article_delete();

CREATE TRIGGER cleanup_saved_items_goals
    BEFORE DELETE ON goals
    FOR EACH ROW
    EXECUTE FUNCTION cleanup_saved_items_on_goal_delete();

CREATE TRIGGER cleanup_saved_items_habits
    BEFORE DELETE ON habits
    FOR EACH ROW
    EXECUTE FUNCTION cleanup_saved_items_on_habit_delete();

-- Add index for faster cleanup operations
CREATE INDEX IF NOT EXISTS idx_saved_items_item_type_item_id ON saved_items(item_type, item_id);
