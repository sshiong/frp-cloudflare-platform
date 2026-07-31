-- 010_create_mapping_revisions.sql
-- Mapping version history

CREATE TABLE IF NOT EXISTS mapping_revisions (
    id            TEXT PRIMARY KEY,
    mapping_id    TEXT NOT NULL,
    revision      INTEGER NOT NULL,
    remote_port   INTEGER CHECK (remote_port IS NULL OR (remote_port > 0 AND remote_port <= 65535)),
    config_json   TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'superseded', 'failed')),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    applied_at    TEXT,

    FOREIGN KEY (mapping_id) REFERENCES mappings(id) ON DELETE CASCADE ON UPDATE CASCADE,

    UNIQUE (mapping_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_mapping_revisions_mapping_id ON mapping_revisions(mapping_id);
CREATE INDEX IF NOT EXISTS idx_mapping_revisions_status ON mapping_revisions(status);
