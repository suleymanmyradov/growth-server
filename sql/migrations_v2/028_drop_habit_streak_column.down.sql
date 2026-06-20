-- Restore the habits.streak column. Values default to 0; re-deriving the
-- historical streak from check_ins is handled at read time by the application.
ALTER TABLE habits
    ADD COLUMN streak integer NOT NULL DEFAULT 0 CHECK (streak >= 0);
