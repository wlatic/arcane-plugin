ALTER TABLE projects ADD COLUMN git_identity_id TEXT REFERENCES git_identities(id) ON DELETE SET NULL;
CREATE INDEX idx_projects_git_identity_id ON projects(git_identity_id);
