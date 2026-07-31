-- 011_create_port_leases.sql
-- Remote port allocations with UNIQUE(server_id, remote_port)

CREATE TABLE IF NOT EXISTS port_leases (
    id                  TEXT PRIMARY KEY,
    server_id           TEXT NOT NULL,
    mapping_id          TEXT NOT NULL,
    mapping_revision_id TEXT,
    remote_port         INTEGER NOT NULL CHECK (remote_port > 0 AND remote_port <= 65535),
    lease_role          TEXT NOT NULL CHECK (lease_role IN ('active', 'pending')),
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (mapping_id)          REFERENCES mappings(id)          ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (mapping_revision_id) REFERENCES mapping_revisions(id) ON DELETE SET NULL ON UPDATE CASCADE,

    UNIQUE (server_id, remote_port)
);

CREATE INDEX IF NOT EXISTS idx_port_leases_mapping_id ON port_leases(mapping_id);
CREATE INDEX IF NOT EXISTS idx_port_leases_lease_role ON port_leases(lease_role);
