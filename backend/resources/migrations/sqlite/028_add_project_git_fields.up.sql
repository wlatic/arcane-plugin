ALTER TABLE projects ADD COLUMN git_repo_url TEXT;
ALTER TABLE projects ADD COLUMN git_branch TEXT;
ALTER TABLE projects ADD COLUMN git_path TEXT;
ALTER TABLE projects ADD COLUMN git_auth_user TEXT;
ALTER TABLE projects ADD COLUMN git_auth_token TEXT;
ALTER TABLE projects ADD COLUMN git_auto_pull BOOLEAN DEFAULT 0;
