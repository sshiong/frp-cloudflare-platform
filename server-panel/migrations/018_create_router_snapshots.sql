-- 018_create_router_snapshots.sql
-- Router config snapshots

CREATE TABLE IF NOT EXISTS router_snapshots (
    version          INTEGER PRIMARY KEY,
    schema_version   TEXT NOT NULL,
    snapshot_path    TEXT NOT NULL,
    snapshot_hash    TEXT NOT NULL,
    snapshot_hmac    TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'applied', 'failed', 'superseded')),
    generated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    applied_at       TEXT,
    last_error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_router_snapshots_status ON router_snapshots(status);
