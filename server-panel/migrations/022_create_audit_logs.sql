-- 022_create_audit_logs.sql
-- Audit trail

CREATE TABLE IF NOT EXISTS audit_logs (
    id                        TEXT PRIMARY KEY,
    actor_type                TEXT NOT NULL CHECK (actor_type IN ('user', 'device', 'admin', 'system')),
    actor_id                  TEXT NOT NULL,
    server_session_id         TEXT,
    client_id                 TEXT,
    local_proxy_session_id    TEXT,
    browser_source_ip         TEXT,
    client_panel_source_ip    TEXT,
    user_agent                TEXT,
    request_id                TEXT,
    operation_id              TEXT,
    action                    TEXT NOT NULL,
    resource_type             TEXT,
    resource_id               TEXT,
    result                    TEXT NOT NULL CHECK (result IN ('success', 'failure', 'denied')),
    metadata_json             TEXT,
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_type, actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id) WHERE resource_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_operation_id ON audit_logs(operation_id) WHERE operation_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id ON audit_logs(request_id) WHERE request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_client_id ON audit_logs(client_id) WHERE client_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_logs_result ON audit_logs(result);
