-- 015_create_certificates.sql
-- Certificate management

CREATE TABLE IF NOT EXISTS certificates (
    id                      TEXT PRIMARY KEY,
    domain_binding_id       TEXT NOT NULL,
    provider                TEXT NOT NULL DEFAULT 'lets_encrypt' CHECK (provider IN ('lets_encrypt', 'zerossl', 'manual')),
    status                  TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'valid', 'renewing', 'expired',
        'blocked_missing_token', 'blocked_invalid_token', 'error'
    )),
    not_before              TEXT,
    not_after               TEXT,
    renew_after             TEXT,
    cert_path               TEXT,
    private_key_ciphertext  TEXT,
    private_key_nonce       TEXT,
    wrapping_key_version    INTEGER,
    cert_hash               TEXT,
    last_error_code         TEXT,
    last_error_message      TEXT,
    created_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),

    FOREIGN KEY (domain_binding_id) REFERENCES domain_bindings(id) ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_certificates_domain_binding_id ON certificates(domain_binding_id);
CREATE INDEX IF NOT EXISTS idx_certificates_status ON certificates(status);
CREATE INDEX IF NOT EXISTS idx_certificates_renew_after ON certificates(renew_after) WHERE renew_after IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_certificates_not_after ON certificates(not_after);
