DROP INDEX IF EXISTS channels_short_id_idx;
ALTER TABLE channels DROP COLUMN IF EXISTS short_id;
