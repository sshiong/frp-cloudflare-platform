-- 014_create_cloudflare_credentials.sql
-- CF token storage (active/pending/retired)

CREATE TABLE IF NOT EXISTS cloudflare_credentials (
    id                 TEXT PRIMARY KEY,
    user_id            TEXT NOT NULL,
    token_version      INTEGER NOT NULL,
    ciphertext         TEXT NOT NULL,
    nonce              TEXT NOT NULL,
    key_version        INTEGER NOT NULL,
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'retired', 'invalid')),
    capabilities_json  TEXT,
    verified_at        TEXT,
    activated_at       TEXT,
    retired_at         TEXT,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,

    UNIQUE (user_id, token_version)
);

CREATE INDEX IF NOT EXISTS idx_cloudflare_credentials_user_id ON cloudflare_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_cloudflare_credentials_status ON cloudflare_credentials(status);
