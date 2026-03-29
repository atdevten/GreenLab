ALTER TABLE devices ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX ON devices(workspace_id, deleted_at) WHERE deleted_at IS NULL;
