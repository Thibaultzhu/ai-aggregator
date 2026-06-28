-- AI Aggregator Platform - v1.2 Alert Rules
-- Persistent operational alert rules for the Admin control plane.

CREATE TABLE IF NOT EXISTS alert_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    metric          TEXT NOT NULL,
    operator        TEXT NOT NULL DEFAULT '>='
        CHECK (operator IN ('>=', '>', '<=', '<', '=', '!=')),
    threshold       NUMERIC(18,8) NULL,
    severity        TEXT NOT NULL DEFAULT 'warning'
        CHECK (severity IN ('info', 'warning', 'critical')),
    window_minutes  INTEGER NOT NULL DEFAULT 60,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_by      UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled);
CREATE INDEX IF NOT EXISTS idx_alert_rules_metric ON alert_rules(metric);

INSERT INTO alert_rules (name, metric, operator, threshold, severity, window_minutes, enabled, metadata)
VALUES
    ('Provider health degradation', 'provider_health_unhealthy', '>', 0, 'critical', 5, true, '{"builtin":true}'),
    ('1h gateway error rate', 'request_error_rate', '>=', 0.05, 'warning', 60, true, '{"builtin":true}')
ON CONFLICT DO NOTHING;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_alert_rules_updated') THEN
        CREATE TRIGGER trg_alert_rules_updated
            BEFORE UPDATE ON alert_rules
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
