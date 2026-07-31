-- 002_create_users.sql
-- User accounts with roles, status, quotas

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    password_hash    TEXT NOT NULL,
    role             TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    must_change_password INTEGER NOT NULL DEFAULT 1 CHECK (must_change_password IN (0, 1)),
    auth_version     INTEGER NOT NULL DEFAULT 1,

    -- Quotas
    max_clients                    INTEGER NOT NULL DEFAULT 5,
    max_mappings                   INTEGER NOT NULL DEFAULT 20,
    max_domains                    INTEGER NOT NULL DEFAULT 10,
    max_pending_mappings           INTEGER NOT NULL DEFAULT 5,
    max_pending_port_leases        INTEGER NOT NULL DEFAULT 5,
    max_pending_domain_operations  INTEGER NOT NULL DEFAULT 3,
    max_certificate_jobs           INTEGER NOT NULL DEFAULT 3,

    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    deleted_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_users_status ON users(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role) WHERE deleted_at IS NULL;
