-- AI Aggregator Platform - v0.7 Prompt Templates Baseline
-- Migration: 020_v20_prompt_templates

CREATE TABLE IF NOT EXISTS prompt_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id    UUID NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            VARCHAR(256) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    template        TEXT NOT NULL,
    variables       JSONB NOT NULL DEFAULT '[]',
    metadata        JSONB NOT NULL DEFAULT '{}',
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_user
    ON prompt_templates(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_workspace
    ON prompt_templates(workspace_id, created_at DESC)
    WHERE workspace_id IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_prompt_templates_updated') THEN
        CREATE TRIGGER trg_prompt_templates_updated
            BEFORE UPDATE ON prompt_templates
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
