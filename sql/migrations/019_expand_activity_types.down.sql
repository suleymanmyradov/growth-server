ALTER TABLE activities
DROP CONSTRAINT IF EXISTS activities_item_type_check;

ALTER TABLE activities
ADD CONSTRAINT activities_item_type_check
CHECK (item_type IN (
    'habit_completed',
    'goal_created',
    'goal_completed',
    'article_saved'
));
