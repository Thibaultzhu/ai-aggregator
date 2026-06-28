-- AI Aggregator Platform - v0.2 Observability
-- Migration: 003_v02_observability
--
-- Rollback note:
--   DROP TABLE IF EXISTS fallback_logs;
--   DROP TABLE IF EXISTS provider_health_checks;
--   DROP TABLE IF EXISTS request_logs;
--   ALTER TABLE users DROP COLUMN IF EXISTS is_admin;

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ALTER COLUMN balance_usd TYPE NUMERIC(18,8);
ALTER TABLE billing_transactions ALTER COLUMN amount_usd TYPE NUMERIC(18,8);
ALTER TABLE billing_transactions ALTER COLUMN balance_after_usd TYPE NUMERIC(18,8);

CREATE TABLE IF NOT EXISTS request_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id          TEXT NOT NULL UNIQUE,
    user_id             UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    api_key_id          UUID NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    workspace_id        UUID NULL,
    model_id            VARCHAR(128) NULL REFERENCES models(model_id) ON DELETE SET NULL,
    provider_id         VARCHAR(64) NULL REFERENCES providers(id) ON DELETE SET NULL,
    final_provider_id   VARCHAR(64) NULL REFERENCES providers(id) ON DELETE SET NULL,
    method              TEXT NOT NULL,
    path                TEXT NOT NULL,
    status_code         INTEGER NOT NULL,
    error_code          TEXT NULL,
    error_message       TEXT NULL,
    latency_ms          INTEGER NOT NULL DEFAULT 0,
    input_tokens        INTEGER NOT NULL DEFAULT 0,
    output_tokens       INTEGER NOT NULL DEFAULT 0,
    total_tokens        INTEGER NOT NULL DEFAULT 0,
    charged_cost_usd    NUMERIC(18,8) NOT NULL DEFAULT 0,
    upstream_cost_usd   NUMERIC(18,8) NOT NULL DEFAULT 0,
    gross_margin_usd    NUMERIC(18,8) NOT NULL DEFAULT 0,
    fallback_count      INTEGER NOT NULL DEFAULT 0,
    request_preview     TEXT NULL,
    response_preview    TEXT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_request_logs_user_created_at ON request_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_model_created_at ON request_logs(model_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_created_at ON request_logs(final_provider_id, created_at DESC);

CREATE TABLE IF NOT EXISTS provider_health_checks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     VARCHAR(64) NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    status          TEXT NOT NULL CHECK (status IN ('healthy', 'unhealthy', 'degraded', 'unknown')),
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    error_code      TEXT NULL,
    error_message   TEXT NULL,
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_provider_health_provider_checked_at
    ON provider_health_checks(provider_id, checked_at DESC);

CREATE TABLE IF NOT EXISTS fallback_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id          TEXT NOT NULL,
    model_id            VARCHAR(128) NULL REFERENCES models(model_id) ON DELETE SET NULL,
    from_provider_id    VARCHAR(64) NULL REFERENCES providers(id) ON DELETE SET NULL,
    to_provider_id      VARCHAR(64) NULL REFERENCES providers(id) ON DELETE SET NULL,
    reason              TEXT NOT NULL,
    error_code          TEXT NULL,
    error_message       TEXT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_fallback_logs_request_id ON fallback_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_fallback_logs_created_at ON fallback_logs(created_at DESC);
