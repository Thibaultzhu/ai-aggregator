-- AI Aggregator Platform - v1.3 Alert Events
-- Persistent firing history for operational alerts.

CREATE TABLE IF NOT EXISTS alert_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dedupe_key      TEXT NOT NULL UNIQUE,
    rule_id         UUID NULL REFERENCES alert_rules(id) ON DELETE SET NULL,
    severity        TEXT NOT NULL DEFAULT 'warning'
        CHECK (severity IN ('info', 'warning', 'critical')),
    status          TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'acknowledged', 'resolved')),
    title           TEXT NOT NULL,
    description     TEXT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at TIMESTAMPTZ NULL,
    resolved_at     TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_events_status_last_seen
    ON alert_events(status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_alert_events_rule
    ON alert_events(rule_id, last_seen_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_alert_events_updated') THEN
        CREATE TRIGGER trg_alert_events_updated
            BEFORE UPDATE ON alert_events
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
