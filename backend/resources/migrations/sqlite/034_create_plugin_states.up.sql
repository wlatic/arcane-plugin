CREATE TABLE plugin_states (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL,
    plugin_id TEXT NOT NULL,
    base_url TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 0,
    config TEXT,
    shared_secret TEXT NOT NULL,
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY(environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_plugin_states_env_plugin ON plugin_states(environment_id, plugin_id);
