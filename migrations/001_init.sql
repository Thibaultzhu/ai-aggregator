-- AI Aggregator Platform - Initial Schema
-- Migration: 001_init

-- ===== Required Extensions =====
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

-- ===== Users =====
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(256) UNIQUE,
    username        VARCHAR(64) UNIQUE,
    password_hash   VARCHAR(256),
    role            VARCHAR(16) DEFAULT 'user' CHECK (role IN ('user', 'admin', 'super_admin')),
    status          VARCHAR(16) DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    
    -- Billing
    balance_usd     DECIMAL(12,4) DEFAULT 0,
    monthly_quota   DECIMAL(10,2) DEFAULT 100.00,
    billing_mode    VARCHAR(16) DEFAULT 'prepaid' CHECK (billing_mode IN ('prepaid', 'postpaid', 'unlimited')),
    
    -- Metadata
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);

-- ===== API Keys =====
CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash        VARCHAR(256) UNIQUE NOT NULL,
    key_prefix      VARCHAR(16) NOT NULL,
    name            VARCHAR(128),
    permissions     JSONB DEFAULT '{"models":"*"}',
    rate_limit_rpm  INTEGER,
    rate_limit_tpm  INTEGER,
    expires_at      TIMESTAMPTZ,
    is_active       BOOLEAN DEFAULT true,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_keys_active ON api_keys(is_active) WHERE is_active = true;

-- ===== Providers =====
CREATE TABLE IF NOT EXISTS providers (
    id              VARCHAR(64) PRIMARY KEY,
    display_name    VARCHAR(256) NOT NULL,
    adapter_type    VARCHAR(32) NOT NULL,
    base_url        VARCHAR(512),
    config          JSONB DEFAULT '{}',
    is_enabled      BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- ===== Provider Keys (encrypted references) =====
CREATE TABLE IF NOT EXISTS provider_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id     VARCHAR(64) NOT NULL REFERENCES providers(id),
    key_name        VARCHAR(64) NOT NULL,
    key_ref         VARCHAR(256) NOT NULL,
    region          VARCHAR(16),
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- ===== Models =====
CREATE TABLE IF NOT EXISTS models (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id        VARCHAR(128) UNIQUE NOT NULL,
    display_name    VARCHAR(256),
    modality        VARCHAR(32) NOT NULL CHECK (modality IN ('text', 'image', 'video', 'audio', 'embedding')),
    capabilities    JSONB DEFAULT '[]',
    input_price     DECIMAL(10,6),
    output_price    DECIMAL(10,6),
    price_unit      VARCHAR(32) DEFAULT 'per_1k_tokens',
    max_context     INTEGER,
    max_output      INTEGER,
    supports_stream BOOLEAN DEFAULT true,
    is_async        BOOLEAN DEFAULT false,
    status          VARCHAR(16) DEFAULT 'active' CHECK (status IN ('active', 'deprecated', 'maintenance')),
    tags            VARCHAR(64)[] DEFAULT '{}',
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_models_modality ON models(modality);
CREATE INDEX IF NOT EXISTS idx_models_status ON models(status) WHERE status = 'active';

-- ===== Model-Provider Bindings =====
CREATE TABLE IF NOT EXISTS model_providers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id        VARCHAR(128) NOT NULL REFERENCES models(model_id),
    provider_id     VARCHAR(64) NOT NULL REFERENCES providers(id),
    priority        INTEGER DEFAULT 1,
    upstream_model  VARCHAR(128),
    endpoint_url    VARCHAR(512),
    api_key_ref     VARCHAR(128),
    is_stream       BOOLEAN DEFAULT true,
    cost_multiplier DECIMAL(5,2) DEFAULT 1.00,
    timeout_ms      INTEGER DEFAULT 30000,
    max_retries     INTEGER DEFAULT 2,
    is_enabled      BOOLEAN DEFAULT true,
    health_status   VARCHAR(16) DEFAULT 'unknown',
    last_health_chk TIMESTAMPTZ,
    UNIQUE(model_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_mp_model ON model_providers(model_id);
CREATE INDEX IF NOT EXISTS idx_mp_provider ON model_providers(provider_id);

-- ===== Usage Logs =====
CREATE TABLE IF NOT EXISTS usage_logs (
    id              BIGSERIAL PRIMARY KEY,
    request_id      VARCHAR(64) UNIQUE NOT NULL,
    user_id         UUID NOT NULL REFERENCES users(id),
    api_key_id      UUID REFERENCES api_keys(id),
    model_id        VARCHAR(128) NOT NULL,
    provider_id     VARCHAR(64) NOT NULL,
    modality        VARCHAR(32) NOT NULL,
    
    -- Usage
    input_tokens    INTEGER DEFAULT 0,
    output_tokens   INTEGER DEFAULT 0,
    total_tokens    INTEGER DEFAULT 0,
    image_count     INTEGER DEFAULT 0,
    duration_sec    DECIMAL(8,2) DEFAULT 0,
    
    -- Performance
    latency_ms      INTEGER NOT NULL,
    ttft_ms         INTEGER,
    is_stream       BOOLEAN DEFAULT false,
    
    -- Cost
    upstream_cost_usd DECIMAL(10,6),
    charged_cost_usd  DECIMAL(10,6),
    
    -- Status
    status_code     INTEGER NOT NULL,
    error_type      VARCHAR(64),
    is_cached       BOOLEAN DEFAULT false,
    
    -- Context
    region          VARCHAR(16) DEFAULT 'jkt',
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Partition by month for archiving
CREATE INDEX IF NOT EXISTS idx_usage_user ON usage_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_model ON usage_logs(model_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_key ON usage_logs(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_created ON usage_logs(created_at DESC);

-- ===== Billing Transactions =====
CREATE TABLE IF NOT EXISTS billing_transactions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id),
    amount_usd          DECIMAL(12,6) NOT NULL,
    balance_after_usd   DECIMAL(12,6),
    tx_type             VARCHAR(32) NOT NULL CHECK (tx_type IN ('credit_grant', 'usage_charge', 'topup', 'refund', 'adjustment')),
    description         TEXT,
    metadata            JSONB DEFAULT '{}',
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_user ON billing_transactions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_type ON billing_transactions(tx_type);

-- ===== Async Tasks =====
CREATE TABLE IF NOT EXISTS async_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     VARCHAR(64) UNIQUE NOT NULL,
    user_id         UUID NOT NULL REFERENCES users(id),
    model_id        VARCHAR(128) NOT NULL,
    provider_id     VARCHAR(64) NOT NULL,
    upstream_task_id VARCHAR(256),
    status          VARCHAR(16) DEFAULT 'pending',
    request_params  JSONB NOT NULL,
    result_data     JSONB,
    cost_usd        DECIMAL(10,6),
    created_at      TIMESTAMPTZ DEFAULT now(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    retry_count     INTEGER DEFAULT 0,
    max_retries     INTEGER DEFAULT 3,
    callback_url    VARCHAR(512),
    region          VARCHAR(16) DEFAULT 'jkt'
);

CREATE INDEX IF NOT EXISTS idx_tasks_user_status ON async_tasks(user_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON async_tasks(status) WHERE status IN ('pending', 'submitted', 'processing');
CREATE INDEX IF NOT EXISTS idx_tasks_created ON async_tasks(created_at DESC);

-- ===== Semantic Cache =====
CREATE TABLE IF NOT EXISTS semantic_cache (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id        VARCHAR(128) NOT NULL,
    query_embedding vector(1024),
    query_text      TEXT NOT NULL,
    response_text   TEXT NOT NULL,
    token_count     INTEGER,
    hit_count       INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT now(),
    expires_at      TIMESTAMPTZ DEFAULT now() + INTERVAL '24 hours'
);

CREATE INDEX IF NOT EXISTS idx_cache_embedding ON semantic_cache 
    USING ivfflat (query_embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX IF NOT EXISTS idx_cache_expires ON semantic_cache(expires_at);

-- ===== Audit Log =====
CREATE TABLE IF NOT EXISTS audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID REFERENCES users(id),
    action          VARCHAR(64) NOT NULL,
    resource_type   VARCHAR(64),
    resource_id     VARCHAR(128),
    details         JSONB,
    ip_address      VARCHAR(45),
    user_agent      VARCHAR(512),
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action, created_at DESC);

-- ===== System Settings (key-value with types) =====
CREATE TABLE IF NOT EXISTS system_settings (
    key             VARCHAR(128) PRIMARY KEY,
    value           TEXT NOT NULL,
    value_type      VARCHAR(16) DEFAULT 'string',
    description     TEXT,
    updated_by      UUID REFERENCES users(id),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Insert default settings
INSERT INTO system_settings (key, value, value_type, description) VALUES
    ('global_rpm_limit', '10000', 'int', 'Global requests per minute limit'),
    ('default_user_balance', '5.00', 'decimal', 'New user welcome bonus (USD)'),
    ('default_markup', '1.50', 'decimal', 'Default cost markup multiplier'),
    ('registration_enabled', 'true', 'boolean', 'Allow new user registration'),
    ('free_trial_credits', '5.00', 'decimal', 'Free trial credits for new users'),
    ('max_file_size_mb', '50', 'int', 'Maximum upload file size in MB'),
    ('task_timeout_seconds', '300', 'int', 'Async task timeout in seconds'),
    ('cache_ttl_hours', '24', 'int', 'Semantic cache TTL in hours')
ON CONFLICT DO NOTHING;

-- ===== Functions =====

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_providers_updated BEFORE UPDATE ON providers FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_models_updated BEFORE UPDATE ON models FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Grant permissions for application user
-- (Run separately after creating the 'aggregator' role)
-- GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO aggregator;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO aggregator;
