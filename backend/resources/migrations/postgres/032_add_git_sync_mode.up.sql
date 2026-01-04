ALTER TABLE projects ADD COLUMN git_sync_mode TEXT DEFAULT 'manual';
UPDATE projects SET git_sync_mode = 'pull' WHERE git_auto_pull = true;
ALTER TABLE projects DROP COLUMN git_auto_pull;
