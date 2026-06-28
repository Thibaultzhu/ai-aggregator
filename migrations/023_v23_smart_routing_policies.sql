-- v0.5/v1.0 Smart Routing policy baseline.

CREATE TABLE IF NOT EXISTS routing_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'global' CHECK (scope IN ('global', 'model', 'workspace')),
    scope_id TEXT NOT NULL DEFAULT '',
    strategy TEXT NOT NULL DEFAULT 'priority' CHECK (strategy IN ('priority', 'cost', 'latency', 'balanced')),
    latency_weight DOUBLE PRECISION NOT NULL DEFAULT 0.4,
    cost_weight DOUBLE PRECISION NOT NULL DEFAULT 0.3,
    error_weight DOUBLE PRECISION NOT NULL DEFAULT 0.3,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_routing_policies_scope_enabled
    ON routing_policies(scope, scope_id, is_enabled, updated_at DESC);

DROP TRIGGER IF EXISTS trg_routing_policies_updated ON routing_policies;
CREATE TRIGGER trg_routing_policies_updated
    BEFORE UPDATE ON routing_policies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
