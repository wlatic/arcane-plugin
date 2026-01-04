-- SQLite does not support dropping columns easily in all versions, but for recent versions:
ALTER TABLE projects DROP COLUMN git_identity_id;
