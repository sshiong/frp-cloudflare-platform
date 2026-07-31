-- 016_create_config_snapshots.sql
-- FRPC config versions

CREATE TABLE IF NOT EXISTS config_snapshots (
    id                     TEXT PRIMARY KEY,
    client_id              TEXT NOT NULL,
    version                INTEGER NOT NULL,
    schema_version         TEXT NOT NULL,
    config_json            TEXT NOT NULL,
    config_hash            TEXT NOT NULL,
    config_signing_key_id  TEXT NOT NULL,
    config_signature       TEXT NOT NULL,
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE ON UPDATE CASCADE,

    UNIQUE (client_id, version)
);

CREATE INDEX IF NOT EXISTS idx_config_snapshots_client_id ON config_snapshots(client_id);
CREATE INDEX IF NOT EXISTS idx_config_snapshots_config_signing_key_id ON config_snapshots(config_signing_key_id);
