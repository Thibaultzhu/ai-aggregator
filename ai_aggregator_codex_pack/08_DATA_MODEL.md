# Data Model

## Current v0.1 Tables

```text
users
api_keys
providers
models
model_providers
usage_logs
billing_transactions
system_settings
```

## v0.2 Required Additions

### users

Add:

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;
```

### request_logs

Purpose: request-level observability.

```sql
CREATE TABLE IF NOT EXISTS request_logs (
    id UUID PRIMARY KEY,
    request_id TEXT NOT NULL UNIQUE,
    user_id UUID NULL REFERENCES users(id),
    api_key_id UUID NULL REFERENCES api_keys(id),
    workspace_id UUID NULL,
    model_id UUID NULL REFERENCES models(id),
    provider_id UUID NULL REFERENCES providers(id),
    final_provider_id UUID NULL REFERENCES providers(id),
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    charged_cost_usd NUMERIC(18,8) NOT NULL DEFAULT 0,
    upstream_cost_usd NUMERIC(18,8) NOT NULL DEFAULT 0,
    gross_margin_usd NUMERIC(18,8) NOT NULL DEFAULT 0,
    fallback_count INTEGER NOT NULL DEFAULT 0,
    request_preview TEXT NULL,
    response_preview TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_request_logs_user_created_at ON request_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_model_created_at ON request_logs(model_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_created_at ON request_logs(final_provider_id, created_at DESC);
```

### provider_health_checks

```sql
CREATE TABLE IF NOT EXISTS provider_health_checks (
    id UUID PRIMARY KEY,
    provider_id UUID NOT NULL REFERENCES providers(id),
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NULL,
    error_message TEXT NULL,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_provider_health_provider_checked_at ON provider_health_checks(provider_id, checked_at DESC);
```

Recommended status values:

```text
healthy
unhealthy
degraded
unknown
```

### fallback_logs

```sql
CREATE TABLE IF NOT EXISTS fallback_logs (
    id UUID PRIMARY KEY,
    request_id TEXT NOT NULL,
    model_id UUID NULL REFERENCES models(id),
    from_provider_id UUID NULL REFERENCES providers(id),
    to_provider_id UUID NULL REFERENCES providers(id),
    reason TEXT NOT NULL,
    error_code TEXT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_fallback_logs_request_id ON fallback_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_fallback_logs_created_at ON fallback_logs(created_at DESC);
```

## v0.2 Current Implementation Notes

The existing MVP schema uses string business identifiers for models and providers:

```text
models.model_id VARCHAR(128)
providers.id VARCHAR(64)
model_providers.model_id VARCHAR(128)
model_providers.provider_id VARCHAR(64)
```

Therefore `migrations/003_v02_observability.sql` implements:

```text
request_logs.model_id VARCHAR(128) REFERENCES models(model_id)
request_logs.provider_id VARCHAR(64) REFERENCES providers(id)
request_logs.final_provider_id VARCHAR(64) REFERENCES providers(id)
provider_health_checks.provider_id VARCHAR(64) REFERENCES providers(id)
fallback_logs.model_id VARCHAR(128) REFERENCES models(model_id)
fallback_logs.from_provider_id VARCHAR(64) REFERENCES providers(id)
fallback_logs.to_provider_id VARCHAR(64) REFERENCES providers(id)
```

This preserves the runnable v0.1 path instead of forcing a UUID schema rewrite.

Money precision was also upgraded in v0.2:

```sql
ALTER TABLE users ALTER COLUMN balance_usd TYPE NUMERIC(18,8);
ALTER TABLE billing_transactions ALTER COLUMN amount_usd TYPE NUMERIC(18,8);
ALTER TABLE billing_transactions ALTER COLUMN balance_after_usd TYPE NUMERIC(18,8);
```

Reason: some low-cost model calls charge less than `0.0001`, and the old `DECIMAL(12,4)` balance could round away valid deductions.

## v0.3 Current Foundation Tables

Implemented in:

```text
migrations/004_v03_enterprise_foundation.sql
```

Current scope is schema foundation only. Runtime organization/workspace APIs, RBAC enforcement, budget blocking, and workspace usage UI are not wired yet.

```text
organizations
workspaces
workspace_members
roles
audit_logs
workspace_budgets
workspace_quotas
api_keys.workspace_id
usage_logs.workspace_id
request_logs.workspace_id FK
billing_transactions.organization_id
billing_transactions.workspace_id
audit_logs.organization_id
audit_logs.workspace_id
```

Still planned for later v0.3/v0.3+:

```text
permissions
api_key_scopes
provider_credentials
```

## v0.4 Future Tables

```text
model_tags
model_capabilities
model_benchmarks
model_scores
model_pricing_history
model_reviews
```

## v0.5 Future Tables

```text
benchmark_tasks
benchmark_runs
benchmark_results
eval_datasets
ab_tests
routing_experiments
quality_evaluations
```

## v0.6 Future Tables

```text
guardrail_policies
guardrail_results
pii_detections
policy_violations
retention_policies
```

## v0.7 Future Tables

```text
workflows
workflow_steps
workflow_runs
workflow_run_steps
tools
tool_credentials
prompt_templates
agent_sessions
agent_traces
webhook_deliveries
```

## v1.0 Future Tables

```text
inference_clusters
inference_nodes
model_deployments
deployment_replicas
gpu_resources
capacity_metrics
```

## Billing Field Rules

Every priced request should eventually produce:

| Field | Meaning |
|---|---|
| charged_cost_usd | Amount charged to user |
| upstream_cost_usd | Cost paid to upstream provider or estimated self-hosted cost |
| gross_margin_usd | charged_cost_usd - upstream_cost_usd |

## Log Truncation Rules

- request_preview max 2,000 chars by default.
- response_preview max 2,000 chars by default.
- Never store API key.
- Apply future PII masking before long-term retention.
