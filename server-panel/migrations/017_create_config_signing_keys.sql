-- 017_create_config_signing_keys.sql
-- Ed25519 signing keys

CREATE TABLE IF NOT EXISTS config_signing_keys (
    key_id                TEXT PRIMARY KEY,
    public_key            TEXT NOT NULL,
    private_key_ciphertext TEXT NOT NULL,
    private_key_nonce     TEXT NOT NULL,
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retired')),
    not_before            TEXT NOT NULL,
    not_after             TEXT,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    retired_at            TEXT
);

CREATE INDEX IF NOT EXISTS idx_config_signing_keys_status ON config_signing_keys(status);

-- Signing keys table (used by signing module)
CREATE TABLE IF NOT EXISTS signing_keys (
    id           TEXT PRIMARY KEY,
    public_key   TEXT NOT NULL,
    private_key  TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT '',
    active       INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_signing_keys_active ON signing_keys(active);
