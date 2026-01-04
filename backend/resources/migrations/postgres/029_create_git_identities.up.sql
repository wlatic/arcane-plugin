CREATE TABLE git_identities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    username TEXT,
    token TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_git_identities_created_at ON git_identities(created_at);
