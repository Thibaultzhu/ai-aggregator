-- AI Aggregator Platform - v1.0 Private / Sovereign AI + Self-hosted Inference
-- Migration: 010_v10_private_sovereign_ai

CREATE TABLE IF NOT EXISTS inference_clusters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(256) NOT NULL,
    region          VARCHAR(64) NOT NULL DEFAULT 'local',
    network_mode    VARCHAR(32) NOT NULL DEFAULT 'private'
        CHECK (network_mode IN ('public', 'private', 'vpc')),
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'maintenance', 'disabled')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inference_nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      UUID NOT NULL REFERENCES inference_clusters(id) ON DELETE CASCADE,
    name            VARCHAR(256) NOT NULL,
    endpoint_url    TEXT NOT NULL DEFAULT '',
    gpu_type        VARCHAR(128) NOT NULL DEFAULT '',
    gpu_count       INTEGER NOT NULL DEFAULT 0,
    status          VARCHAR(16) NOT NULL DEFAULT 'healthy'
        CHECK (status IN ('healthy', 'degraded', 'down', 'maintenance')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inference_nodes_cluster ON inference_nodes(cluster_id, status);

CREATE TABLE IF NOT EXISTS model_deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id      UUID NOT NULL REFERENCES inference_clusters(id) ON DELETE CASCADE,
    provider_id     TEXT NOT NULL,
    model_id        TEXT NOT NULL,
    upstream_model  TEXT NOT NULL,
    runtime         VARCHAR(32) NOT NULL DEFAULT 'vllm'
        CHECK (runtime IN ('vllm', 'sglang', 'openai_compatible')),
    endpoint_url    TEXT NOT NULL,
    replicas        INTEGER NOT NULL DEFAULT 1,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'deploying', 'maintenance', 'disabled')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_model_deployments_model ON model_deployments(model_id, status);
CREATE INDEX IF NOT EXISTS idx_model_deployments_cluster ON model_deployments(cluster_id, status);

CREATE TABLE IF NOT EXISTS capacity_metrics (
    id              BIGSERIAL PRIMARY KEY,
    cluster_id      UUID NULL REFERENCES inference_clusters(id) ON DELETE CASCADE,
    node_id         UUID NULL REFERENCES inference_nodes(id) ON DELETE CASCADE,
    gpu_total       INTEGER NOT NULL DEFAULT 0,
    gpu_available   INTEGER NOT NULL DEFAULT 0,
    memory_total_gb NUMERIC(12,4) NOT NULL DEFAULT 0,
    memory_used_gb  NUMERIC(12,4) NOT NULL DEFAULT 0,
    qps             NUMERIC(12,4) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_capacity_metrics_cluster ON capacity_metrics(cluster_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_inference_clusters_updated') THEN
        CREATE TRIGGER trg_inference_clusters_updated
            BEFORE UPDATE ON inference_clusters
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_inference_nodes_updated') THEN
        CREATE TRIGGER trg_inference_nodes_updated
            BEFORE UPDATE ON inference_nodes
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_model_deployments_updated') THEN
        CREATE TRIGGER trg_model_deployments_updated
            BEFORE UPDATE ON model_deployments
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
