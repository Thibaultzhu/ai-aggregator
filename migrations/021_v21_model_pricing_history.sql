-- AI Aggregator Platform - v0.4/v0.5 Pricing History Baseline
-- Migration: 021_v21_model_pricing_history

CREATE TABLE IF NOT EXISTS model_pricing_history (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id            VARCHAR(128) NOT NULL REFERENCES models(model_id) ON DELETE CASCADE,
    old_input_price     DECIMAL(10,6),
    new_input_price     DECIMAL(10,6),
    old_output_price    DECIMAL(10,6),
    new_output_price    DECIMAL(10,6),
    old_price_unit      VARCHAR(32) NOT NULL DEFAULT '',
    new_price_unit      VARCHAR(32) NOT NULL DEFAULT '',
    change_type         VARCHAR(32) NOT NULL DEFAULT 'update'
        CHECK (change_type IN ('create', 'update')),
    changed_by          UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_model_pricing_history_model_created
    ON model_pricing_history(model_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_pricing_history_changed_by
    ON model_pricing_history(changed_by, created_at DESC)
    WHERE changed_by IS NOT NULL;
