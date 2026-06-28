-- Migration: 028_v28_request_log_credential_scope
-- Adds non-secret provider credential attribution to request logs.

ALTER TABLE request_logs
    ADD COLUMN IF NOT EXISTS credential_scope TEXT NULL,
    ADD COLUMN IF NOT EXISTS credential_key_id UUID NULL REFERENCES provider_keys(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_request_logs_credential_scope_created
    ON request_logs(credential_scope, created_at DESC);

