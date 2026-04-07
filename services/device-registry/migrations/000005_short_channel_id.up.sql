ALTER TABLE channels ADD COLUMN IF NOT EXISTS short_id SERIAL UNIQUE;
CREATE INDEX IF NOT EXISTS channels_short_id_idx ON channels(short_id);
