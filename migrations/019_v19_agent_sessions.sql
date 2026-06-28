-- AI Aggregator Platform - v0.7 Agent Sessions Baseline
-- Migration: 019_v19_agent_sessions

CREATE TABLE IF NOT EXISTS agent_sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id         UUID NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    workflow_id          UUID NULL REFERENCES workflows(id) ON DELETE SET NULL,
    name                VARCHAR(256) NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed')),
    metadata            JSONB NOT NULL DEFAULT '{}',
    last_run_id         UUID NULL REFERENCES workflow_runs(id) ON DELETE SET NULL,
    last_activity_at    TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_user
    ON agent_sessions(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_workflow
    ON agent_sessions(workflow_id, created_at DESC)
    WHERE workflow_id IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'workflow_runs' AND column_name = 'agent_session_id'
    ) THEN
        ALTER TABLE workflow_runs
            ADD COLUMN agent_session_id UUID NULL REFERENCES agent_sessions(id) ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_workflow_runs_agent_session') THEN
        CREATE INDEX idx_workflow_runs_agent_session
            ON workflow_runs(agent_session_id, created_at DESC)
            WHERE agent_session_id IS NOT NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_agent_sessions_updated') THEN
        CREATE TRIGGER trg_agent_sessions_updated
            BEFORE UPDATE ON agent_sessions
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
