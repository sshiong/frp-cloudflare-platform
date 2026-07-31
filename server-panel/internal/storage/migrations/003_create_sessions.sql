-- 003_create_sessions.sql
-- Server web sessions with single active per client_id

CREATE TABLE IF NOT EXISTS sessions (
    id                        TEXT PRIMARY KEY,
    user_id                   TEXT NOT NULL,
    client_id                 TEXT,
    installation_instance_id  TEXT,
    local_proxy_session_id    TEXT,
    session_hash              TEXT NOT NULL,
    csrf_secret_hash          TEXT NOT NULL,
    auth_version              INTEGER NOT NULL DEFAULT 1,
    session_generation        INTEGER NOT NULL DEFAULT 1,
    login_channel             TEXT NOT NULL DEFAULT 'web' CHECK (login_channel IN ('web', 'device_proxy')),
    browser_source_ip         TEXT,
    client_panel_source_ip    TEXT,
    user_agent                TEXT,
    expires_at                TEXT NOT NULL,
    idle_expires_at           TEXT NOT NULL,
    last_seen_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    revoked_at                TEXT,
    revoke_reason             TEXT,
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Unique constraint on (client_id, local_proxy_session_id) when both are present
CREATE UNIQUE INDEX IF NOT EXISTS ux_sessions_client_local_proxy
    ON sessions(client_id, local_proxy_session_id)
    WHERE client_id IS NOT NULL AND local_proxy_session_id IS NOT NULL;

-- Partial unique index: only one active (non-revoked) session per client_id
CREATE UNIQUE INDEX IF NOT EXISTS one_active_client_session
    ON sessions(client_id)
    WHERE client_id IS NOT NULL AND revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_session_hash ON sessions(session_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_revoked_at ON sessions(revoked_at) WHERE revoked_at IS NULL;
