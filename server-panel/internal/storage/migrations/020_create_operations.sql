-- 020_create_operations.sql
-- Operation tracking with state machine

CREATE TABLE IF NOT EXISTS operations (
    id                   TEXT PRIMARY KEY,
    user_id              TEXT NOT NULL,
    client_id            TEXT,
    resource_type        TEXT NOT NULL,
    resource_id          TEXT NOT NULL,
    operation_type       TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'running', 'waiting_client', 'waiting_external',
        'succeeded', 'failed', 'cancelled'
    )),
    phase                TEXT,
    step                 INTEGER NOT NULL DEFAULT 0,
    idempotency_key      TEXT,
    cancelable           INTEGER NOT NULL DEFAULT 1 CHECK (cancelable IN (0, 1)),
    compensation_status  TEXT CHECK (compensation_status IN ('none', 'pending', 'in_progress', 'completed', 'failed')),
    error_code           TEXT,
    error_message        TEXT,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    completed_at         TEXT,

    FOREIGN KEY (user_id)   REFERENCES users(id)   ON DELETE RESTRICT ON UPDATE CASCADE,
    FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE SET NULL  ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_operations_user_id ON operations(user_id);
CREATE INDEX IF NOT EXISTS idx_operations_client_id ON operations(client_id) WHERE client_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_operations_resource ON operations(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_operations_status ON operations(status);
CREATE INDEX IF NOT EXISTS idx_operations_idempotency_key ON operations(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_operations_operation_type ON operations(operation_type);
