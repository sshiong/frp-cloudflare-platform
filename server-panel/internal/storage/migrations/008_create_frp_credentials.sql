-- 008_create_frp_credentials.sql
-- FRP user credentials

CREATE TABLE IF NOT EXISTS frp_credentials (
    id                      TEXT PRIMARY KEY,
    user_id                 TEXT NOT NULL,
    frp_username            TEXT NOT NULL UNIQUE,
    manual_token_hash       TEXT NOT NULL,
    manual_token_ciphertext TEXT NOT NULL,
    manual_token_nonce      TEXT NOT NULL,
    key_version             INTEGER NOT NULL,
    token_version           INTEGER NOT NULL DEFAULT 1,
    created_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    rotated_at              TEXT,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_frp_credentials_user_id ON frp_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_frp_credentials_manual_token_hash ON frp_credentials(manual_token_hash);
