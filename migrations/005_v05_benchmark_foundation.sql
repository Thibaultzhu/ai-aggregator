-- AI Aggregator Platform - v0.5 Evaluation / Benchmark Foundation
-- Migration: 005_v05_benchmark_foundation

CREATE TABLE IF NOT EXISTS benchmark_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(256) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    dataset         JSONB NOT NULL DEFAULT '[]',
    created_by      UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_benchmark_tasks_status ON benchmark_tasks(status, created_at DESC);

CREATE TABLE IF NOT EXISTS benchmark_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES benchmark_tasks(id) ON DELETE CASCADE,
    model_ids       TEXT[] NOT NULL DEFAULT '{}',
    status          VARCHAR(16) NOT NULL DEFAULT 'completed'
        CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    started_at      TIMESTAMPTZ NULL,
    completed_at    TIMESTAMPTZ NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_benchmark_runs_task ON benchmark_runs(task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_benchmark_runs_status ON benchmark_runs(status, created_at DESC);

CREATE TABLE IF NOT EXISTS benchmark_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID NOT NULL REFERENCES benchmark_runs(id) ON DELETE CASCADE,
    task_id         UUID NOT NULL REFERENCES benchmark_tasks(id) ON DELETE CASCADE,
    model_id        TEXT NOT NULL,
    quality_score   NUMERIC(8,4) NOT NULL DEFAULT 0,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    cost_usd        NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_score     NUMERIC(8,4) NOT NULL DEFAULT 0,
    details         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_benchmark_results_run ON benchmark_results(run_id, total_score DESC);
CREATE INDEX IF NOT EXISTS idx_benchmark_results_model ON benchmark_results(model_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_benchmark_tasks_updated') THEN
        CREATE TRIGGER trg_benchmark_tasks_updated
            BEFORE UPDATE ON benchmark_tasks
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
