-- 005_create_device_credentials.sql
-- Device HMAC credentials with versioning

CREATE TABLE IF NOT EXISTS device_credentials (
    id                     TEXT PRIMARY KEY,
    client_id              TEXT NOT NULL,
    token_version          INTEGER NOT NULL,
    device_token_hash      TEXT NOT NULL,
    signing_key_ciphertext TEXT NOT NULL,
    signing_key_nonce      TEXT NOT NULL,
    master_key_version     INTEGER NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'revoked')),
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    activated_at           TEXT,
    revoked_at             TEXT,

    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE ON UPDATE CASCADE,

    UNIQUE (client_id, token_version)
);

CREATE INDEX IF NOT EXISTS idx_device_credentials_client_id ON device_credentials(client_id);
CREATE INDEX IF NOT EXISTS idx_device_credentials_status ON device_credentials(status);
CREATE INDEX IF NOT EXISTS idx_device_credentials_device_token_hash ON device_credentials(device_token_hash);
