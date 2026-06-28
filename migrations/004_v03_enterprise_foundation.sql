-- AI Aggregator Platform - v0.3 Enterprise Foundation
-- Migration: 004_v03_enterprise_foundation
--
-- Scope:
--   Add organization/workspace/RBAC/FinOps foundations without changing the
--   current single-user v0.1/v0.2 request path. Runtime enforcement will be
--   introduced incrementally after the schema is present.

CREATE TABLE IF NOT EXISTS organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(256) NOT NULL,
    slug            VARCHAR(128) UNIQUE NOT NULL,
    owner_user_id   UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'deleted')),
    billing_mode    VARCHAR(16) NOT NULL DEFAULT 'prepaid'
        CHECK (billing_mode IN ('prepaid', 'postpaid', 'unlimited')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_organizations_owner ON organizations(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);

CREATE TABLE IF NOT EXISTS workspaces (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(256) NOT NULL,
    slug            VARCHAR(128) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'deleted')),
    monthly_budget_usd NUMERIC(18,8) NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(organization_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_workspaces_org ON workspaces(organization_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_status ON workspaces(status);

CREATE TABLE IF NOT EXISTS roles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(64) NOT NULL,
    description     TEXT NULL,
    permissions     JSONB NOT NULL DEFAULT '[]',
    is_system       BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(organization_id, name)
);

CREATE TABLE IF NOT EXISTS workspace_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id         UUID NULL REFERENCES roles(id) ON DELETE SET NULL,
    role_name       VARCHAR(64) NOT NULL DEFAULT 'member',
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('invited', 'active', 'suspended', 'removed')),
    invited_by      UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    joined_at       TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_workspace_members_user ON workspace_members(user_id);
CREATE INDEX IF NOT EXISTS idx_workspace_members_workspace ON workspace_members(workspace_id);

CREATE TABLE IF NOT EXISTS workspace_budgets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    period          VARCHAR(16) NOT NULL DEFAULT 'monthly'
        CHECK (period IN ('daily', 'weekly', 'monthly')),
    amount_usd      NUMERIC(18,8) NOT NULL,
    soft_limit_pct  INTEGER NOT NULL DEFAULT 80,
    hard_limit_pct  INTEGER NOT NULL DEFAULT 100,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspace_budgets_workspace ON workspace_budgets(workspace_id, is_active);

CREATE TABLE IF NOT EXISTS workspace_quotas (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    quota_type      VARCHAR(32) NOT NULL
        CHECK (quota_type IN ('requests_per_minute', 'tokens_per_minute', 'tokens_per_month', 'spend_per_month')),
    limit_value     NUMERIC(18,8) NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspace_quotas_workspace ON workspace_quotas(workspace_id, is_active);

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL;
ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS organization_id UUID NULL REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS organization_id UUID NULL REFERENCES organizations(id) ON DELETE SET NULL;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_request_logs_workspace'
          AND table_name = 'request_logs'
    ) THEN
        ALTER TABLE request_logs
            ADD CONSTRAINT fk_request_logs_workspace
            FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_api_keys_workspace ON api_keys(workspace_id);
CREATE INDEX IF NOT EXISTS idx_usage_workspace_created ON usage_logs(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_workspace_created ON billing_transactions(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_workspace_created ON request_logs(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_workspace_created ON audit_logs(workspace_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_organizations_updated') THEN
        CREATE TRIGGER trg_organizations_updated
            BEFORE UPDATE ON organizations
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_workspaces_updated') THEN
        CREATE TRIGGER trg_workspaces_updated
            BEFORE UPDATE ON workspaces
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_roles_updated') THEN
        CREATE TRIGGER trg_roles_updated
            BEFORE UPDATE ON roles
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_workspace_members_updated') THEN
        CREATE TRIGGER trg_workspace_members_updated
            BEFORE UPDATE ON workspace_members
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_workspace_budgets_updated') THEN
        CREATE TRIGGER trg_workspace_budgets_updated
            BEFORE UPDATE ON workspace_budgets
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_workspace_quotas_updated') THEN
        CREATE TRIGGER trg_workspace_quotas_updated
            BEFORE UPDATE ON workspace_quotas
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
