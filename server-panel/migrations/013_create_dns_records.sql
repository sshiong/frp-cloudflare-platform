-- 013_create_dns_records.sql
-- DNS record management

CREATE TABLE IF NOT EXISTS dns_records (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL,
    domain_binding_id TEXT NOT NULL,
    type              TEXT NOT NULL CHECK (type IN ('A', 'AAAA', 'CNAME')),
    name              TEXT NOT NULL,
    normalized_name   TEXT NOT NULL,
    content           TEXT NOT NULL,
    ttl               INTEGER NOT NULL DEFAULT 1 CHECK (ttl = 1 OR (ttl >= 60 AND ttl <= 86400)),
    proxied           INTEGER NOT NULL DEFAULT 0 CHECK (proxied IN (0, 1)),
    zone_id           TEXT,
    record_id         TEXT,
    managed_by_panel  INTEGER NOT NULL DEFAULT 1 CHECK (managed_by_panel IN (0, 1)),
    adopted           INTEGER NOT NULL DEFAULT 0 CHECK (adopted IN (0, 1)),
    locked            INTEGER NOT NULL DEFAULT 0 CHECK (locked IN (0, 1)),
    sync_status       TEXT NOT NULL DEFAULT 'pending' CHECK (sync_status IN (
        'pending', 'synced', 'drift', 'error'
    )),
    last_synced_at    TEXT,
    last_error_code   TEXT,
    last_error_message TEXT,

    FOREIGN KEY (user_id)           REFERENCES users(id)           ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (domain_binding_id) REFERENCES domain_bindings(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_dns_records_user_id ON dns_records(user_id);
CREATE INDEX IF NOT EXISTS idx_dns_records_domain_binding_id ON dns_records(domain_binding_id);
CREATE INDEX IF NOT EXISTS idx_dns_records_zone_id ON dns_records(zone_id) WHERE zone_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_dns_records_record_id ON dns_records(record_id) WHERE record_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_dns_records_sync_status ON dns_records(sync_status);
CREATE INDEX IF NOT EXISTS idx_dns_records_normalized_name ON dns_records(normalized_name);
