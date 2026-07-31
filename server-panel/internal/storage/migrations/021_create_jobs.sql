-- 021_create_jobs.sql
-- Background job queue with lease/dedup

CREATE TABLE IF NOT EXISTS jobs (
    id                TEXT PRIMARY KEY,
    type              TEXT NOT NULL,
    resource_type     TEXT,
    resource_id       TEXT,
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'running', 'succeeded', 'failed', 'cancelled'
    )),
    run_after         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    attempts          INTEGER NOT NULL DEFAULT 0,
    max_attempts      INTEGER NOT NULL DEFAULT 5,
    lock_owner        TEXT,
    locked_at         TEXT,
    lock_expires_at   TEXT,
    heartbeat_at      TEXT,
    deduplication_key TEXT,
    token_version     INTEGER,
    last_error        TEXT,
    payload_json      TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    completed_at      TEXT,

    UNIQUE (type, deduplication_key)
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_run_after ON jobs(run_after) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_jobs_lock_owner ON jobs(lock_owner) WHERE lock_owner IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_lock_expires_at ON jobs(lock_expires_at) WHERE lock_expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_resource ON jobs(resource_type, resource_id) WHERE resource_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_type_status ON jobs(type, status);
