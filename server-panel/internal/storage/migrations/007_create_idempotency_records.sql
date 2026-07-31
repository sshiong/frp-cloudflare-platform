-- 007_create_idempotency_records.sql
-- Request idempotency tracking

CREATE TABLE IF NOT EXISTS idempotency_records (
    id                 TEXT PRIMARY KEY,
    actor_type         TEXT NOT NULL CHECK (actor_type IN ('user', 'device', 'admin', 'system')),
    actor_id           TEXT NOT NULL,
    client_id          TEXT,
    http_method        TEXT NOT NULL,
    normalized_path    TEXT NOT NULL,
    idempotency_key    TEXT NOT NULL,
    request_body_hash  TEXT NOT NULL,
    response_status    INTEGER,
    response_body_json TEXT,
    operation_id       TEXT,
    expires_at         TEXT NOT NULL,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    UNIQUE (actor_type, actor_id, http_method, normalized_path, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_records_expires_at
    ON idempotency_records(expires_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_records_operation_id
    ON idempotency_records(operation_id) WHERE operation_id IS NOT NULL;
