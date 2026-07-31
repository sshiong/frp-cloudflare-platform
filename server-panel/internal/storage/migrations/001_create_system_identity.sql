-- 001_create_system_identity.sql
-- Singleton table for server_instance_id

CREATE TABLE IF NOT EXISTS system_identity (
    singleton_id           INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    server_instance_id     TEXT    NOT NULL UNIQUE,
    created_at             TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    restored_from_backup_at TEXT
);
