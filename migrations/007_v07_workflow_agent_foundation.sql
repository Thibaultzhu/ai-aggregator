-- AI Aggregator Platform - v0.7 Workflow / Agent API Foundation
-- Migration: 007_v07_workflow_agent_foundation

CREATE TABLE IF NOT EXISTS workflows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    workspace_id    UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    name            VARCHAR(256) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workflows_user ON workflows(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflows_workspace ON workflows(workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS workflow_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    step_order      INTEGER NOT NULL,
    name            VARCHAR(256) NOT NULL,
    step_type       VARCHAR(32) NOT NULL DEFAULT 'prompt'
        CHECK (step_type IN ('prompt', 'tool', 'transform')),
    model_id        TEXT NOT NULL DEFAULT '',
    tool_id         TEXT NOT NULL DEFAULT '',
    prompt_template TEXT NOT NULL DEFAULT '',
    config          JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workflow_id, step_order)
);

CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow ON workflow_steps(workflow_id, step_order);

CREATE TABLE IF NOT EXISTS tools (
    id              TEXT PRIMARY KEY,
    display_name    VARCHAR(256) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    tool_type       VARCHAR(32) NOT NULL DEFAULT 'builtin'
        CHECK (tool_type IN ('builtin', 'http', 'function')),
    schema          JSONB NOT NULL DEFAULT '{}',
    is_enabled      BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflow_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    user_id         UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    workspace_id    UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    input           JSONB NOT NULL DEFAULT '{}',
    output          JSONB NOT NULL DEFAULT '{}',
    total_cost_usd  NUMERIC(18,8) NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ NULL,
    completed_at    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON workflow_runs(workflow_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflow_runs_user ON workflow_runs(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS workflow_run_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    workflow_step_id UUID NULL REFERENCES workflow_steps(id) ON DELETE SET NULL,
    step_order      INTEGER NOT NULL,
    name            VARCHAR(256) NOT NULL,
    step_type       VARCHAR(32) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'completed'
        CHECK (status IN ('running', 'completed', 'failed')),
    input           JSONB NOT NULL DEFAULT '{}',
    output          JSONB NOT NULL DEFAULT '{}',
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    cost_usd        NUMERIC(18,8) NOT NULL DEFAULT 0,
    error_message   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workflow_run_steps_run ON workflow_run_steps(run_id, step_order);

CREATE TABLE IF NOT EXISTS agent_traces (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    step_id         UUID NULL REFERENCES workflow_run_steps(id) ON DELETE SET NULL,
    trace_type      VARCHAR(64) NOT NULL,
    message         TEXT NOT NULL DEFAULT '',
    data            JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_traces_run ON agent_traces(run_id, created_at);

INSERT INTO tools (id, display_name, description, tool_type, schema, is_enabled)
VALUES ('echo', 'Echo Tool', 'Returns the supplied input for workflow smoke tests.', 'builtin', '{"input":"any"}'::jsonb, true)
ON CONFLICT (id) DO NOTHING;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_workflows_updated') THEN
        CREATE TRIGGER trg_workflows_updated
            BEFORE UPDATE ON workflows
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_workflow_steps_updated') THEN
        CREATE TRIGGER trg_workflow_steps_updated
            BEFORE UPDATE ON workflow_steps
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_tools_updated') THEN
        CREATE TRIGGER trg_tools_updated
            BEFORE UPDATE ON tools
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
