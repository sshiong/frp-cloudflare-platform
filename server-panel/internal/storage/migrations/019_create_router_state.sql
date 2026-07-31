-- 019_create_router_state.sql
-- Router state singleton

CREATE TABLE IF NOT EXISTS router_state (
    singleton_id                  INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    router_config_version         INTEGER NOT NULL DEFAULT 0,
    router_applied_version        INTEGER NOT NULL DEFAULT 0,
    last_good_snapshot_version    INTEGER,
    last_good_snapshot_path       TEXT,
    last_good_snapshot_hash       TEXT,
    last_router_apply_error       TEXT,
    updated_at                    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
