-- v17: Workflow webhook delivery audit records

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    run_id          UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    callback_url    TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'recorded'
        CHECK (status IN ('recorded', 'delivered', 'failed', 'skipped')),
    response_status INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT NOT NULL DEFAULT '',
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_run_created
    ON webhook_deliveries(run_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_workflow_created
    ON webhook_deliveries(workflow_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_webhook_deliveries_updated') THEN
        CREATE TRIGGER trg_webhook_deliveries_updated
            BEFORE UPDATE ON webhook_deliveries
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
