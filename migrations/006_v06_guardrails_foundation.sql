-- AI Aggregator Platform - v0.6 Guardrails / Compliance Foundation
-- Migration: 006_v06_guardrails_foundation

CREATE TABLE IF NOT EXISTS guardrail_policies (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(256) NOT NULL,
    scope               VARCHAR(32) NOT NULL DEFAULT 'global'
        CHECK (scope IN ('global', 'organization', 'workspace', 'api_key')),
    scope_id            TEXT NULL,
    is_enabled          BOOLEAN NOT NULL DEFAULT true,
    pii_action          VARCHAR(16) NOT NULL DEFAULT 'mask'
        CHECK (pii_action IN ('allow', 'mask', 'block')),
    injection_action    VARCHAR(16) NOT NULL DEFAULT 'block'
        CHECK (injection_action IN ('allow', 'mask', 'block')),
    moderation_action   VARCHAR(16) NOT NULL DEFAULT 'block'
        CHECK (moderation_action IN ('allow', 'mask', 'block')),
    config              JSONB NOT NULL DEFAULT '{}',
    created_by          UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_guardrail_policies_scope ON guardrail_policies(scope, scope_id, is_enabled);

CREATE TABLE IF NOT EXISTS guardrail_results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id          TEXT NOT NULL,
    user_id             UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    workspace_id        UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    api_key_id          UUID NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    model_id            TEXT NOT NULL DEFAULT '',
    policy_id           UUID NULL REFERENCES guardrail_policies(id) ON DELETE SET NULL,
    action              VARCHAR(16) NOT NULL DEFAULT 'allow'
        CHECK (action IN ('allow', 'mask', 'block')),
    status              VARCHAR(16) NOT NULL DEFAULT 'passed'
        CHECK (status IN ('passed', 'masked', 'blocked')),
    risk_score          NUMERIC(8,4) NOT NULL DEFAULT 0,
    categories          TEXT[] NOT NULL DEFAULT '{}',
    findings            JSONB NOT NULL DEFAULT '[]',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_guardrail_results_request ON guardrail_results(request_id);
CREATE INDEX IF NOT EXISTS idx_guardrail_results_created ON guardrail_results(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_guardrail_results_workspace ON guardrail_results(workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS pii_detections (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guardrail_result_id UUID NOT NULL REFERENCES guardrail_results(id) ON DELETE CASCADE,
    pii_type            VARCHAR(64) NOT NULL,
    count               INTEGER NOT NULL DEFAULT 1,
    action              VARCHAR(16) NOT NULL DEFAULT 'mask',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pii_detections_result ON pii_detections(guardrail_result_id);

CREATE TABLE IF NOT EXISTS policy_violations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guardrail_result_id UUID NOT NULL REFERENCES guardrail_results(id) ON DELETE CASCADE,
    policy_id           UUID NULL REFERENCES guardrail_policies(id) ON DELETE SET NULL,
    request_id          TEXT NOT NULL,
    violation_type      VARCHAR(64) NOT NULL,
    severity            VARCHAR(16) NOT NULL DEFAULT 'medium'
        CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    action              VARCHAR(16) NOT NULL DEFAULT 'block',
    details             JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_policy_violations_request ON policy_violations(request_id);
CREATE INDEX IF NOT EXISTS idx_policy_violations_created ON policy_violations(created_at DESC);

INSERT INTO guardrail_policies (name, scope, is_enabled, pii_action, injection_action, moderation_action, config)
SELECT 'Default Global Guardrail Policy', 'global', true, 'mask', 'block', 'block',
       '{"detect_pii": true, "detect_prompt_injection": true, "detect_jailbreak": true}'::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM guardrail_policies WHERE scope = 'global' AND scope_id IS NULL AND name = 'Default Global Guardrail Policy'
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_guardrail_policies_updated') THEN
        CREATE TRIGGER trg_guardrail_policies_updated
            BEFORE UPDATE ON guardrail_policies
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
