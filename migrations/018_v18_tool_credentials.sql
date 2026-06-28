-- AI Aggregator Platform - v0.7 Tool Credentials Baseline
-- Migration: 018_v18_tool_credentials

CREATE TABLE IF NOT EXISTS tool_credentials (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id         UUID NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    tool_id              TEXT NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    name                 VARCHAR(256) NOT NULL,
    secret_encrypted     TEXT NOT NULL,
    secret_mask          TEXT NOT NULL DEFAULT '',
    metadata             JSONB NOT NULL DEFAULT '{}',
    status               VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'revoked')),
    last_used_at         TIMESTAMPTZ NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tool_credentials_user
    ON tool_credentials(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_tool_credentials_workspace
    ON tool_credentials(workspace_id, created_at DESC)
    WHERE workspace_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tool_credentials_tool
    ON tool_credentials(tool_id, status);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_tool_credentials_updated') THEN
        CREATE TRIGGER trg_tool_credentials_updated
            BEFORE UPDATE ON tool_credentials
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
