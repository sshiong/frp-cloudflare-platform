-- 012_create_domain_bindings.sql
-- Domain name bindings with UNIQUE(normalized_domain)

CREATE TABLE IF NOT EXISTS domain_bindings (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL,
    client_id         TEXT NOT NULL,
    mapping_id        TEXT NOT NULL,
    hostname          TEXT NOT NULL,
    normalized_domain TEXT NOT NULL UNIQUE,
    zone_id           TEXT,
    https_mode        TEXT NOT NULL DEFAULT 'http_only' CHECK (https_mode IN (
        'auto_certificate', 'cloudflare_proxy', 'http_only'
    )),
    http_redirect     INTEGER NOT NULL DEFAULT 0 CHECK (http_redirect IN (0, 1)),
    status            TEXT NOT NULL DEFAULT 'reserved' CHECK (status IN (
        'reserved', 'pending_dns', 'pending_certificate', 'active',
        'offline', 'dns_error', 'certificate_error', 'disabled', 'deleting'
    )),
    revision          INTEGER NOT NULL DEFAULT 1,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (user_id)    REFERENCES users(id)    ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (client_id)  REFERENCES clients(id)  ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (mapping_id) REFERENCES mappings(id) ON DELETE CASCADE   ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_domain_bindings_user_id ON domain_bindings(user_id);
CREATE INDEX IF NOT EXISTS idx_domain_bindings_client_id ON domain_bindings(client_id);
CREATE INDEX IF NOT EXISTS idx_domain_bindings_mapping_id ON domain_bindings(mapping_id);
CREATE INDEX IF NOT EXISTS idx_domain_bindings_zone_id ON domain_bindings(zone_id) WHERE zone_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_domain_bindings_status ON domain_bindings(status);
