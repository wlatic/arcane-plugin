ALTER TABLE projects ADD COLUMN git_poll_interval INTEGER DEFAULT 5;
ALTER TABLE projects ADD COLUMN git_last_sync_at DATETIME;
