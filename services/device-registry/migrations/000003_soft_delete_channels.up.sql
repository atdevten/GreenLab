ALTER TABLE channels ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX ON channels(workspace_id, deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX ON channels(device_id, deleted_at) WHERE deleted_at IS NULL;
