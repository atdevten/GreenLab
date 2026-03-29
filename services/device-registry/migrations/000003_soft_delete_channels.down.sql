DROP INDEX IF EXISTS channels_workspace_id_deleted_at_idx;
DROP INDEX IF EXISTS channels_device_id_deleted_at_idx;
ALTER TABLE channels DROP COLUMN IF EXISTS deleted_at;
