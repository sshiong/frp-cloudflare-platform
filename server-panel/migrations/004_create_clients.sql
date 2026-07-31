-- 004_create_clients.sql
-- Client device registrations

CREATE TABLE IF NOT EXISTS clients (
    id                              TEXT PRIMARY KEY,
    server_instance_id              TEXT NOT NULL,
    owner_user_id                   TEXT NOT NULL,
    installation_instance_id        TEXT NOT NULL,
    name                            TEXT NOT NULL,
    status                          TEXT NOT NULL DEFAULT 'bound' CHECK (status IN ('bound', 'disabled', 'revoked', 'unbound')),
    binding_revision                INTEGER NOT NULL DEFAULT 1,
    session_generation              INTEGER NOT NULL DEFAULT 0,
    active_device_credential_version INTEGER NOT NULL DEFAULT 0,
    frp_device_token_hash           TEXT,
    frp_device_token_version        INTEGER NOT NULL DEFAULT 0,
    desired_config_version          INTEGER NOT NULL DEFAULT 0,
    applied_config_version          INTEGER NOT NULL DEFAULT 0,
    last_failed_config_version      INTEGER,
    last_error_code                 TEXT,
    last_error_message              TEXT,
    client_panel_version            TEXT,
    frpc_version                    TEXT,
    protocol_version                TEXT,
    config_schema_version           TEXT,
    last_seen_at                    TEXT,
    last_ip                         TEXT,
    registered_at                   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    unbound_at                      TEXT,
    created_at                      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at                      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE RESTRICT ON UPDATE CASCADE,

    UNIQUE (server_instance_id, installation_instance_id)
);

CREATE INDEX IF NOT EXISTS idx_clients_owner_user_id ON clients(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_clients_server_instance_id ON clients(server_instance_id);
CREATE INDEX IF NOT EXISTS idx_clients_status ON clients(status);
CREATE INDEX IF NOT EXISTS idx_clients_installation_instance_id ON clients(installation_instance_id);
