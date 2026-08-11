-- Add streak-warnings and Sunday-review notification preference flags.
-- Both default to false (off): streak warnings are off by default per the
-- product decision that "pressure isn't the point", and the Sunday review
-- email is opt-in.
ALTER TABLE notification_preferences
    ADD COLUMN streak_warnings boolean NOT NULL DEFAULT false,
    ADD COLUMN sunday_review   boolean NOT NULL DEFAULT false;
