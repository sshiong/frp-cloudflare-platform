-- 006_create_device_request_nonces.sql
-- Nonce dedup for HMAC replay prevention

CREATE TABLE IF NOT EXISTS device_request_nonces (
    client_id          TEXT NOT NULL,
    token_version      INTEGER NOT NULL,
    nonce_hash         TEXT NOT NULL,
    request_timestamp  INTEGER NOT NULL,
    expires_at         TEXT NOT NULL,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    UNIQUE (client_id, token_version, nonce_hash)
);

CREATE INDEX IF NOT EXISTS idx_device_request_nonces_expires_at
    ON device_request_nonces(expires_at);
