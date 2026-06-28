-- Migration: 026_v26_webhook_delivery_worker
-- Adds delivery worker metadata for workflow webhooks.

ALTER TABLE webhook_deliveries
    DROP CONSTRAINT IF EXISTS webhook_deliveries_status_check;

ALTER TABLE webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_status_check
        CHECK (status IN ('recorded', 'retrying', 'delivered', 'failed', 'skipped'));

ALTER TABLE webhook_deliveries
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_attempts INTEGER NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS response_body TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS signature TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status_created
    ON webhook_deliveries(status, created_at DESC);
