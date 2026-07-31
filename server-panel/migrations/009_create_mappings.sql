-- 009_create_mappings.sql
-- Proxy mappings (tcp/udp/http)

CREATE TABLE IF NOT EXISTS mappings (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL,
    client_id         TEXT NOT NULL,
    name              TEXT NOT NULL,
    proxy_type        TEXT NOT NULL CHECK (proxy_type IN ('tcp', 'udp', 'http')),
    local_ip          TEXT NOT NULL,
    local_port        INTEGER NOT NULL CHECK (local_port > 0 AND local_port <= 65535),
    lifecycle_status  TEXT NOT NULL DEFAULT 'reserved' CHECK (lifecycle_status IN (
        'reserved', 'pending_apply', 'running', 'offline', 'config_error',
        'disabled', 'deleting'
    )),
    desired_state     TEXT NOT NULL DEFAULT 'enabled' CHECK (desired_state IN ('enabled', 'disabled')),
    observed_state    TEXT,
    active_revision   INTEGER NOT NULL DEFAULT 0,
    pending_revision  INTEGER,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (user_id)   REFERENCES users(id)   ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE RESTRICT ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mappings_user_id ON mappings(user_id);
CREATE INDEX IF NOT EXISTS idx_mappings_client_id ON mappings(client_id);
CREATE INDEX IF NOT EXISTS idx_mappings_lifecycle_status ON mappings(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_mappings_proxy_type ON mappings(proxy_type);
CREATE INDEX IF NOT EXISTS idx_mappings_user_client ON mappings(user_id, client_id);
