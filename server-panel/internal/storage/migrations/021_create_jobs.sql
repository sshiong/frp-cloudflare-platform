-- 021_create_jobs.sql
-- Background job queue with lease/dedup

CREATE TABLE IF NOT EXISTS jobs (
    id                TEXT PRIMARY KEY,
    job_type          TEXT NOT NULL,
    payload           TEXT,
    state             TEXT NOT NULL DEFAULT 'pending' CHECK (state IN (
        'pending', 'running', 'succeeded', 'failed', 'cancelled'
    )),
    priority          INTEGER NOT NULL DEFAULT 0,
    attempts          INTEGER NOT NULL DEFAULT 0,
    max_attempts      INTEGER NOT NULL DEFAULT 5,
    lease_until       TEXT,
    locked_by         TEXT,
    error             TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    completed_at      TEXT,

    UNIQUE (job_type, payload)
);

CREATE INDEX IF NOT EXISTS idx_jobs_state ON jobs(state);
CREATE INDEX IF NOT EXISTS idx_jobs_lease_until ON jobs(lease_until) WHERE lease_until IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_locked_by ON jobs(locked_by) WHERE locked_by IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_job_type_state ON jobs(job_type, state);
