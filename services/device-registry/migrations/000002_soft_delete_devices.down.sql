DROP INDEX IF EXISTS devices_workspace_id_deleted_at_idx;
ALTER TABLE devices DROP COLUMN IF EXISTS deleted_at;
