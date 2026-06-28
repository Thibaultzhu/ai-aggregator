-- Migration: 027_v27_secret_management_metadata
-- Adds production secret-management metadata for provider and tool credentials.

ALTER TABLE provider_keys
    ADD COLUMN IF NOT EXISTS seal_version VARCHAR(32) NOT NULL DEFAULT 'local:v1',
    ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_used_scope VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ NULL;

UPDATE provider_keys
SET seal_version = 'local:v1'
WHERE seal_version IS NULL OR seal_version = '';

UPDATE provider_keys
SET seal_version = split_part(key_ref, ':', 1) || ':' || split_part(key_ref, ':', 2)
WHERE key_ref LIKE '%:%:%' AND (seal_version IS NULL OR seal_version = 'local:v1');

CREATE INDEX IF NOT EXISTS idx_provider_keys_last_used
    ON provider_keys(provider_id, last_used_at DESC)
    WHERE last_used_at IS NOT NULL;

ALTER TABLE tool_credentials
    ADD COLUMN IF NOT EXISTS seal_version VARCHAR(32) NOT NULL DEFAULT 'local:v1',
    ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ NULL;

UPDATE tool_credentials
SET seal_version = 'local:v1'
WHERE seal_version IS NULL OR seal_version = '';
