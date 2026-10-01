DROP INDEX IF EXISTS activities_dedupe_key_idx;
ALTER TABLE activities DROP COLUMN IF EXISTS dedupe_key;
ALTER TABLE check_ins DROP COLUMN IF EXISTS version;
