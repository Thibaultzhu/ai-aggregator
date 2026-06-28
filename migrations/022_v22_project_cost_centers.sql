-- AI Aggregator Platform - v1.0 Project Cost Center Foundation
-- Migration: 022_v22_project_cost_centers

CREATE TABLE IF NOT EXISTS projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            VARCHAR(256) NOT NULL,
    slug            VARCHAR(128) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_projects_workspace ON projects(workspace_id, status, created_at DESC);

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE request_logs ADD COLUMN IF NOT EXISTS project_id UUID NULL REFERENCES projects(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_project ON api_keys(project_id);
CREATE INDEX IF NOT EXISTS idx_usage_project_created ON usage_logs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_project_created ON billing_transactions(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_project_created ON request_logs(project_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_projects_updated') THEN
        CREATE TRIGGER trg_projects_updated
            BEFORE UPDATE ON projects
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
