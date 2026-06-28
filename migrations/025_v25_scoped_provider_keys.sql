-- Migration: 025_v25_scoped_provider_keys
-- Adds scoped BYOK support for platform, user, and workspace provider credentials.

ALTER TABLE provider_keys
    ADD COLUMN IF NOT EXISTS scope VARCHAR(16) NOT NULL DEFAULT 'platform'
        CHECK (scope IN ('platform', 'user', 'workspace')),
    ADD COLUMN IF NOT EXISTS user_id UUID NULL REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS key_mask VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

UPDATE provider_keys
SET scope = 'platform'
WHERE scope IS NULL OR scope = '';

UPDATE provider_keys
SET key_mask = 'stored-secret'
WHERE key_mask IS NULL OR key_mask = '';

CREATE INDEX IF NOT EXISTS idx_provider_keys_scope_lookup
    ON provider_keys(provider_id, scope, user_id, workspace_id, is_active, created_at DESC);

DROP TRIGGER IF EXISTS trg_provider_keys_updated ON provider_keys;
CREATE TRIGGER trg_provider_keys_updated
    BEFORE UPDATE ON provider_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
