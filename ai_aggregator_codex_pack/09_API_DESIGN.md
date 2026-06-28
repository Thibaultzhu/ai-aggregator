# API Design

## Existing v0.1 Public APIs

### GET /v1/models

OpenAI-compatible model list.

### POST /v1/chat/completions

OpenAI-compatible chat completion endpoint.

Required behavior:

- Bearer API key authentication.
- Model lookup.
- Provider routing.
- Provider response normalization.
- Usage parsing.
- Billing transaction.
- Usage log.
- Request log in v0.2.

## v0.2 User APIs

### User Billing APIs

Implemented endpoints:

```text
GET /api/user/billing/balance
GET /api/user/billing/transactions
```

Current billing mode:

```text
Prepaid balance.
New users receive a welcome credit_grant.
Model usage writes usage_charge billing transactions and deducts users.balance_usd.
Admin credit grants are available via POST /api/admin/users/:id/balance.
Self-service checkout/payment provider integration is not implemented yet; Billing.tsx Add Credits shows admin-grant guidance instead of a fake payment action.
```

### POST /api/user/keys

Creates a user API key. v0.3 adds optional workspace binding.

Request:

```json
{
  "name": "production-service",
  "workspace_id": "338527fb-6956-49e0-b1f4-a5d3d6c13f76"
}
```

`workspace_id` is optional. When present, the gateway attaches that workspace to authenticated `/v1/*` requests made with the key.

RBAC enforcement:

```text
If workspace_id is present, the current user must be an active workspace member with api_keys:create.
Default role_name behavior:
  owner/admin = all workspace permissions
  member = api_keys:create, files:write, workflows:write plus read/write operational defaults
  viewer = read-only defaults, no api_keys:create
Custom roles may allow "*" or specific permissions such as "api_keys:create", "files:write", "workflows:write", "workspace_budgets:write", or "workspace_quotas:write" through roles.permissions.
```

Response:

```json
{
  "id": "key_uuid",
  "name": "production-service",
  "key": "sk-aag-...",
  "prefix": "sk-aag-",
  "workspace_id": "338527fb-6956-49e0-b1f4-a5d3d6c13f76"
}
```

### GET /api/user/keys

Returns current user's API keys, including `workspace_id` when a key is bound to a workspace.

## v0.3 Multimodal Gateway APIs

### POST /v1/embeddings

OpenAI-compatible embeddings endpoint.

Current implementation status:

```text
Implemented.
Uses provider fallback through ModelRouter.
Writes usage_logs, request_logs, billing_transactions.
Playground Embedding tab calls this endpoint and displays vector dimensions, usage, and preview values.
```

Request:

```json
{
  "model": "text-embedding-v3",
  "input": "hello embedding"
}
```

### POST /v1/images/generations

Submits an async image generation task.

Current implementation status:

```text
Implemented.
Returns 202 with async task id.
Task worker polls provider and writes async_tasks.result_data.
Playground Image tab calls submit + task polling endpoints.
Current Singapore Bailian smoke returns upstream InvalidParameter for catalog model names; model mapping must be calibrated against live Bailian image API.
```

Request:

```json
{
  "model": "wan-image",
  "prompt": "a simple red cube",
  "n": 1,
  "size": "1024*1024"
}
```

### GET /v1/images/generations/:id

Returns image task status and result for the current authenticated user.

### POST /v1/video/generations

Submits an async video generation task.

Request:

```json
{
  "model": "wan2.7-t2v",
  "prompt": "a simple red cube rotating",
  "duration": 5,
  "resolution": "720p"
}
```

### GET /v1/video/generations/:id

Returns video task status and result for the current authenticated user.

Current implementation status:

```text
Implemented.
Playground Video tab calls submit + task polling endpoints.
Current Singapore Bailian smoke reaches upstream but returns URL parameter error for text-to-video request shape; DashScope video adapter mapping needs live API calibration.
```

### Audio endpoints

```text
POST /v1/audio/transcriptions
POST /v1/audio/speech
```

Current implementation status:

```text
Gateway handlers are implemented.
Mock adapter supports both endpoints.
DashScope adapter supports TTS and ASR gateway paths.
Playground Audio tab calls `/v1/audio/speech` and plays returned audio.
`/v1/audio/transcriptions` is available as a multipart API endpoint.
Regression coverage: `scripts/regression/dashscope-audio-chain.sh` validates fake DashScope `/compatible-mode/v1/models`, native TTS, native ASR, Bearer credential forwarding, request logs, and usage logs.
Production ASR may still need object-storage file URL handoff if the live Bailian account disallows direct multipart uploads.
```

### GET /api/user/request-logs

Returns request logs owned by current user.

Query parameters:

| Parameter | Type | Description |
|---|---|---|
| model | string | optional model filter |
| provider | string | optional provider filter |
| status | string | success / error |
| from | datetime/date | start time, RFC3339 or YYYY-MM-DD |
| to | datetime/date | end time, RFC3339 or YYYY-MM-DD |
| limit | number | page size, default 50, max 500 |
| offset | number | zero-based row offset |
| format | string | optional `csv` for CSV export |

Response:

```json
{
  "items": [
    {
      "request_id": "req_xxx",
      "model": "qwen-plus",
      "provider": "dashscope",
      "status_code": 200,
      "latency_ms": 1200,
      "input_tokens": 120,
      "output_tokens": 240,
      "charged_cost_usd": "0.00120000",
      "workspace_id": "338527fb-6956-49e0-b1f4-a5d3d6c13f76",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ],
  "total": 25,
  "limit": 50,
  "offset": 0
}
```

When `format=csv`, the endpoint returns `text/csv` with summary fields only. Request and response previews are intentionally excluded from CSV export to avoid bulk exporting prompt or response content.

### GET /api/user/request-logs/:request_id

Returns full request detail for current user.

Must enforce ownership.

### GET /api/user/dashboard

Returns current user's aggregate dashboard summary.

Response includes:

```json
{
  "total_requests": 10,
  "total_cost": 0.0123,
  "total_tokens": 1234,
  "average_latency_ms": 358,
  "p95_latency_ms": 396.9,
  "p99_latency_ms": 400.98,
  "error_requests": 1,
  "error_rate": 0.1,
  "balance": 9.9877
}
```

## v0.2 Admin APIs

### Admin Model / Provider CRUD

Current implementation status:

```text
Implemented and smoke-tested.
Protected by admin JWT middleware.
Admin writes refresh router registry so pricing/status/binding changes affect the next request.
Frontend Admin Models page supports inline provider binding edits for upstream_model, priority, timeout, retries, cost multiplier, and enabled state.
```

Implemented endpoints:

```text
GET    /api/admin/models
POST   /api/admin/models
GET    /api/admin/models/:id
PUT    /api/admin/models/:id
DELETE /api/admin/models/:id
POST   /api/admin/models/:id/providers
PUT    /api/admin/models/:id/providers/:pid
DELETE /api/admin/models/:id/providers/:pid

GET    /api/admin/providers
POST   /api/admin/providers
PUT    /api/admin/providers/:id
```

Verified behavior:

```text
Admin can create/list/get/update/delete model.
Admin can create/list/update provider.
Admin can bind/update/unbind model-provider mapping.
Admin can update existing model-provider upstream_model from UI/API and the value is persisted.
Pricing updates are used by the next chat completion.
Disabled model-provider binding is skipped by router on the next request.
```

### GET /api/admin/provider-health

Returns latest provider status.

Response:

```json
{
  "items": [
    {
      "provider_id": "uuid",
      "provider_name": "dashscope",
      "status": "healthy",
      "latency_ms": 200,
      "last_checked_at": "2026-01-01T00:00:00Z",
      "error_message": null
    }
  ]
}
```

Current implementation status:

```text
Implemented in backend/internal/gateway/router.go
Protected by admin JWT middleware.
Backed by provider_health_checks latest row per provider.
Used by Admin Provider Status page and Admin Overview provider summary.
```

Implemented response item fields:

```json
{
  "provider_id": "bailian_cn",
  "display_name": "百炼 DashScope CN",
  "adapter_type": "dashscope",
  "status": "healthy",
  "latency_ms": 0,
  "error_code": "",
  "error_message": "",
  "checked_at": "2026-06-22T00:00:00Z"
}
```

### GET /api/admin/providers/:id/health/history

Returns recent provider health-check history for one provider.

Query parameters:

| Parameter | Type | Description |
|---|---|---|
| limit | number | default 20, max 200 |

Response:

```json
{
  "provider_id": "bailian_cn",
  "items": [
    {
      "provider_id": "bailian_cn",
      "display_name": "百炼 DashScope CN",
      "adapter_type": "dashscope",
      "status": "healthy",
      "latency_ms": 12,
      "error_code": "",
      "error_message": "",
      "checked_at": "2026-06-24T00:00:00Z"
    }
  ]
}
```

### POST /api/admin/providers/:id/health-check

Triggers manual health check.

Current implementation status:

```text
Implemented.
Runs the in-memory provider adapter HealthCheck(ctx).
Writes provider_health_checks and updates model_providers.health_status.
```

Response:

```json
{
  "provider_id": "bailian_cn",
  "status": "healthy",
  "latency_ms": 0,
  "error_message": ""
}
```

## v0.2 Reliability Test Configuration

For local fallback testing in mock provider mode:

```text
MOCK_PROVIDER_MODE=true
MOCK_FAIL_PROVIDER_IDS=bailian_cn
```

This forces chat/stream calls through `bailian_cn` to fail while keeping provider health checks healthy. The expected result is fallback to the next provider and one row in `fallback_logs`.

The fallback regression is now scriptable:

```bash
MOCK_PROVIDER_MODE=true RUN_FALLBACK_SMOKE=true bash scripts/smoke-test.sh
```

The backend must already be running with:

```text
MOCK_FAIL_PROVIDER_IDS=bailian_cn
```

To force health checks to fail instead:

```text
MOCK_FAIL_HEALTH_PROVIDER_IDS=bailian_cn
```

## v0.3 Enterprise API Status

Current implementation:

```text
Organization / Workspace Admin API foundation implemented.
API Key workspace binding implemented.
Workspace request/usage/billing cost attribution implemented.
Admin Users / Analytics / Settings / Alerts / Audit Log foundation implemented.
```

Implemented:

```text
GET /api/admin/organizations
POST /api/admin/organizations
GET /api/admin/workspaces
POST /api/admin/workspaces
GET /api/admin/workspaces/:id/members
POST /api/admin/workspaces/:id/members
GET /api/admin/workspaces/:id/usage
GET /api/admin/workspaces/:id/usage/export
```

Still planned:

```text
Fine-grained RBAC permissions enforcement
Workspace-level request log and cost detail UI filters beyond CSV export
ClickHouse-backed high-volume analytics
```

Implemented governance endpoints:

```text
GET  /api/admin/workspaces/:id/budgets
POST /api/admin/workspaces/:id/budgets
GET  /api/admin/workspaces/:id/quotas
POST /api/admin/workspaces/:id/quotas
GET  /api/user/workspaces/:id/budgets
POST /api/user/workspaces/:id/budgets
GET  /api/user/workspaces/:id/quotas
POST /api/user/workspaces/:id/quotas
GET  /api/admin/users
POST /api/admin/users
GET  /api/admin/users/:id
PUT  /api/admin/users/:id
POST /api/admin/users/:id/balance
GET  /api/admin/users/:id/usage
GET  /api/admin/keys
PUT  /api/admin/keys/:id
DELETE /api/admin/keys/:id
GET  /api/admin/analytics/overview
GET  /api/admin/analytics/usage
GET  /api/admin/analytics/cost
GET  /api/admin/analytics/latency
GET  /api/admin/analytics/errors
GET  /api/admin/settings
PUT  /api/admin/settings/:key
GET  /api/admin/alerts/rules
POST /api/admin/alerts/rules
PUT  /api/admin/alerts/rules/:id
GET  /api/admin/alerts/history
POST /api/admin/alerts/history/:id/ack
POST /api/admin/alerts/history/:id/resolve
GET  /api/admin/audit-logs
GET  /api/admin/audit-logs/export
POST /api/admin/audit-logs/retention/run
```

Audit log CSV export:

```text
GET /api/admin/audit-logs/export?limit=10000&action=file.upload&workspace_id=...&resource_type=file&resource_id=...&from=2026-01-01&to=2026-06-25
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename="audit-logs.csv"
```

Supported audit query filters for both `GET /api/admin/audit-logs` and `GET /api/admin/audit-logs/export`:

```text
limit
action
workspace_id
resource_type
resource_id
from
to
```

CSV fields:

```text
id, created_at, user_id, organization_id, workspace_id, action, resource_type,
resource_id, ip_address, user_agent, details_json
```

Audit log retention cleanup:

```text
POST /api/admin/audit-logs/retention/run
Body:
{
  "dry_run": true,
  "limit": 100
}

Uses system_settings.audit_log_retention_days.
audit_log_retention_days = 0 disables cleanup.
```

Response:

```json
{
  "retention_days": 1,
  "dry_run": true,
  "matched_count": 3,
  "deleted_count": 0,
  "items": []
}
```

`GET /api/admin/analytics/overview` includes gateway-wide average, p95, and p99 latency:

```json
{
  "average_latency_ms": 358,
  "p95_latency_ms": 452.6,
  "p99_latency_ms": 2995.76
}
```

Frontend usage:

```text
frontend/src/pages/Admin.tsx AdminOverview uses /api/admin/analytics/overview for live control-plane metrics.
AdminOverview combines analytics overview with /api/admin/provider-health and /api/marketplace/models.
Admin Analytics page uses overview plus analytics series endpoints for detailed charts.
```

User portal profile endpoints:

```text
POST /api/user/auth/refresh
GET  /api/user/profile
PUT  /api/user/profile
```

Frontend usage:

```text
frontend/src/pages/Settings.tsx uses GET /api/user/profile to load the current account.
frontend/src/pages/Settings.tsx uses PUT /api/user/profile to update editable username and metadata JSON.
Dashboard Settings navigation points to /dashboard/settings.
```

Audio gateway endpoints:

```text
POST /v1/audio/transcriptions
  multipart/form-data:
    file
    model
    language optional
  response:
    {"text":"..."}

POST /v1/audio/speech
  JSON:
    model
    input
    voice optional
    response_format optional
    speed optional
  response:
    audio bytes with provider content-type
```

Current limitation:

```text
Gateway handlers are implemented and record usage/request logs.
Mock adapter supports both endpoints.
DashScope adapter supports both endpoints through native audio paths and records usage/request logs.
Playground supports TTS playback through `/v1/audio/speech`; ASR remains API-first through multipart upload.
Production ASR may still need object-storage file URL handoff if the live Bailian account disallows direct multipart uploads.
```

File gateway endpoints:

```text
User Dashboard includes /dashboard/files for upload/list/detail/download/delete.

POST /v1/files
  multipart/form-data:
    file
    purpose optional, default assistants
  response:
    {
      "id": "file-...",
      "object": "file",
      "bytes": 123,
      "created_at": 1782230000,
      "filename": "example.txt",
      "purpose": "assistants",
      "status": "uploaded",
      "mime_type": "text/plain; charset=utf-8",
      "metadata": {
        "source": "local",
        "declared_mime": "text/plain",
        "detected_mime": "text/plain; charset=utf-8",
        "sha256": "hex-encoded-sha256",
        "extension": ".txt"
      }
    }

GET /v1/files/:id
  Returns file metadata for the current authenticated owner.

GET /v1/files
  Query params:
    limit optional, default 100, max 200
    purpose optional
  response:
    {
      "object": "list",
      "data": [file metadata objects]
    }

GET /v1/files/:id/content
  Returns file bytes for the current authenticated owner.

DELETE /v1/files/:id
  Soft-deletes metadata and removes the local stored object.

POST /api/admin/files/retention/run
  Admin JWT required.
  Body:
    {
      "limit": 100,
      "dry_run": false
    }
  Uses system_settings.file_retention_days.
  file_retention_days = 0 disables cleanup.
  Response:
    {
      "retention_days": 1,
      "dry_run": false,
      "matched_count": 1,
      "deleted_count": 1,
      "storage_deleted_count": 1,
      "files": [
        {
          "id": "file-...",
          "filename": "example.txt",
          "bytes": 123,
          "purpose": "assistants",
          "user_id": "uuid",
          "workspace_id": "uuid",
          "created_at": 1782230000
        }
      ]
    }
```

Current file storage behavior:

```text
Metadata table: uploaded_files
Migration: migrations/011_v11_file_upload_foundation.sql
Governance setting: migrations/014_v14_file_governance_settings.sql
Retention setting: migrations/015_v15_file_retention_settings.sql
Bytes storage: FILE_STORAGE_DIR, default /tmp/ai-aggregator-files
FileStore backend: FILE_STORAGE_BACKEND=local uses backend/internal/filestore.LocalStore
Ownership: enforced by user_id in storage layer
MIME policy: http.DetectContentType result checked against allowed_upload_mime_types
Checksum: SHA-256 stored in file metadata
Retention: admin cleanup marks expired metadata as deleted and removes local bytes best-effort
Current backend: FileStore interface is in place with local implementation; OSS/S3 concrete implementation is a future replacement.
```

Validation status:

```text
2026-06-23 local API smoke passed for profile, refresh, users, analytics, settings, alerts and audit logs.
2026-06-23 go test / Docker build / dev-check passed after audio gateway handler implementation.
2026-06-23 file API smoke passed for register -> create key -> upload -> get -> delete.
2026-06-23 file API smoke passed for upload -> list -> get metadata -> download content -> delete.
2026-06-23 file governance smoke passed for detected_mime + sha256 metadata and disallowed MIME -> 415.
2026-06-23 admin API key smoke passed for list -> revoke -> revoked key rejected by /v1/models.
2026-06-23 alert rule smoke passed for list -> create -> update disabled -> history.
2026-06-23 alert event smoke passed for unhealthy provider -> persisted history -> acknowledge -> acknowledged history.
2026-06-23 alert resolve smoke passed for unhealthy provider -> resolve -> resolved history remains resolved.
2026-06-23 smoke-test script extended with optional embeddings/image/video coverage; bash -n passed.
2026-06-23 admin API key controls smoke passed for update RPM -> 429 and permissions.models -> 403.
2026-06-23 per-key TPM smoke passed for rate_limit_tpm=1 -> chat completion returns gateway tpm_limit 429.
2026-06-24 user Settings/Profile UI smoke passed for register -> GET /api/user/profile -> PUT /api/user/profile.
2026-06-25 admin audit log CSV export smoke passed for admin JWT -> GET /api/admin/audit-logs/export?limit=1 with text/csv and attachment filename.
2026-06-25 admin audit log filters smoke passed for action/resource_type/resource_id/from on JSON list and CSV export.
2026-06-25 admin audit log retention smoke passed for setting audit_log_retention_days=1 -> dry_run matched old events without delete -> run deleted expired event -> setting restored to 0.
2026-06-25 workspace RBAC permission smoke passed for viewer POST /api/user/keys with workspace_id = 403 permission_error and member = 201.
2026-06-25 workspace file RBAC smoke passed for viewer workspace key POST /v1/files = 403 permission_error and member workspace key = 201 with workspace attribution.
2026-06-25 workspace workflow RBAC smoke passed for viewer POST /api/user/workflows with workspace_id = 403 permission_error and member create/run = 201 with workspace attribution.
2026-06-25 workspace budget/quota RBAC smoke passed for viewer POST /api/user/workspaces/:id/budgets = 403 permission_error, owner budget create = 201, owner quota create = 201, invalid quota_type = 400.
2026-06-23 streaming TPM smoke passed for stream=true + rate_limit_tpm=1 -> preflight gateway tpm_limit 429.
```

Runtime enforcement:

When an API key is bound to workspace_id, /v1/* requests run workspace limit checks before provider dispatch.

API key controls:

```text
PUT /api/admin/keys/:id
  name
  workspace_id optional
  permissions JSON, currently supports models: "*" or ["model_id"]
  rate_limit_rpm optional, overrides DEFAULT_RPM
  rate_limit_tpm optional, overrides DEFAULT_TPM
  expires_at optional
  is_active

Runtime behavior:
  expired/inactive keys are rejected during API key auth
  per-key rate_limit_rpm overrides Redis sliding-window RPM limit
  per-key rate_limit_tpm uses Redis minute counters; stream requests are checked with a preflight estimate
  permissions.models restricts single-model /v1 requests before provider dispatch
```
Implemented checks:
- monthly workspace budget hard limit
- requests_per_minute quota
- tokens_per_month quota
- spend_per_month quota
```

Implemented marketplace endpoints:

```text
GET /api/marketplace/models
GET /api/marketplace/models?modality=text&q=qwen&capability=chat
GET /api/marketplace/models/:id
GET /api/marketplace/models/compare?ids=qwen-turbo,qwen-plus
```

Marketplace response includes model identity, modality, capabilities/tags, pricing, context/output limits,
stream/async support, provider availability, deterministic marketplace_score, recommended_use, and benchmark_score placeholder.

Frontend usage:

```text
frontend/src/pages/Models.tsx uses /api/marketplace/models for public catalog browsing, filtering, detail, and compare.
frontend/src/pages/Landing.tsx uses /api/marketplace/models for homepage Featured Models sorted by marketplace_score and dynamic model-count copy.
Landing DemoCard links to /playground with tab/model/prompt query parameters; Playground reads those parameters to prefill the selected modality, model, and prompt.
frontend/src/pages/Pricing.tsx uses /api/marketplace/models for live per-model pricing grouped by modality.
frontend/src/pages/Docs.tsx uses /api/marketplace/models for live catalog count in the API documentation page.
Pricing page no longer reads static mock model pricing; Admin model pricing changes propagate through the marketplace catalog response.
```

Implemented benchmark endpoints:

```text
GET  /api/admin/benchmarks/tasks
POST /api/admin/benchmarks/tasks
GET  /api/admin/benchmarks/runs
POST /api/admin/benchmarks/tasks/:id/runs
GET  /api/admin/benchmarks/runs/:id
```

Benchmark v0.5 behavior:

```text
Admin can create a benchmark task with an inline dataset.
Admin can run selected active models against the task.
Admin UI includes a Benchmarks page for task creation, run submission, and result review.
Current implementation generates deterministic local quality_score, latency_ms, cost_usd, and total_score for development/testing.
Latest benchmark result is exposed as model.benchmark_score in /api/marketplace/models/:id and compare responses.
```

Implemented guardrail endpoints:

```text
GET  /api/admin/guardrails/policies
POST /api/admin/guardrails/policies
GET  /api/admin/guardrails/results
```

Guardrail v0.6 behavior:

```text
A default global guardrail policy is seeded by migration.
Admin UI includes a Guardrails page for policy creation/listing and recent result review.
/v1/chat/completions evaluates guardrails before provider routing.
PII findings use local rules for email, phone/numeric identifiers, and payment-card-like patterns.
Default PII action is mask, so matching content is redacted before dispatch.
Default prompt-injection/jailbreak action is block, returning policy_violation and writing guardrail_results / policy_violations / audit_logs.
```

Implemented workflow / agent endpoints:

```text
GET  /api/user/tools
GET  /api/user/workflows
POST /api/user/workflows
GET  /api/user/workflows/:id
GET  /api/user/workflows/:id/runs
POST /api/user/workflows/:id/runs
GET  /api/user/workflow-runs/:id
```

Workflow v0.7 behavior:

```text
Workflow definitions contain ordered workflow_steps.
User Dashboard includes /dashboard/workflows for create/run/history/trace inspection.
The synchronous runner supports prompt steps and builtin tool steps.
Prompt steps route through the existing ModelRouter and provider adapters.
The builtin echo tool verifies tool registry / tool execution plumbing.
Each run writes workflow_runs, workflow_run_steps, agent_traces, output, status, and total_cost_usd.
```

Implemented private / self-hosted inference endpoints:

```text
GET  /api/admin/inference/clusters
POST /api/admin/inference/clusters
GET  /api/admin/inference/nodes
POST /api/admin/inference/nodes
GET  /api/admin/inference/deployments
POST /api/admin/inference/deployments
```

Private / self-hosted v1.0 behavior:

```text
Admin can register an inference cluster and GPU node.
Admin can register a model deployment with runtime vllm, sglang, or openai_compatible.
Admin UI includes an Inference page for cluster, node, and deployment registration.
Creating a model deployment automatically upserts provider, model, and model_provider binding records.
Router.Refresh now refreshes adapters and registry, so newly registered self-hosted deployments can be called through /v1/chat/completions.
In local MOCK_PROVIDER_MODE=true, self-hosted deployments use mock adapters for deterministic smoke testing.
```

Runtime attribution behavior:

```text
API Key workspace_id
→ auth middleware sets request context workspace_id
→ /v1/chat/completions writes request_logs.workspace_id
→ usage_logs.workspace_id is written with token/cost data
→ billing_transactions.workspace_id and organization_id are written for usage_charge
→ GET /api/admin/workspaces/:id/usage aggregates workspace request/cost/token totals
→ GET /api/admin/workspaces/:id/usage/export exports workspace request/cost/token/latency rows as CSV
```

### Model CRUD

```text
GET    /api/admin/models
POST   /api/admin/models
PATCH  /api/admin/models/:id
DELETE /api/admin/models/:id
```

Model fields:

```json
{
  "name": "qwen-plus",
  "display_name": "Qwen Plus",
  "description": "General purpose chat model",
  "context_window": 32768,
  "input_price_per_1m_tokens": "0.40000000",
  "output_price_per_1m_tokens": "1.20000000",
  "enabled": true
}
```

### Provider CRUD

```text
GET    /api/admin/providers
POST   /api/admin/providers
PATCH  /api/admin/providers/:id
DELETE /api/admin/providers/:id
```

Provider fields:

```json
{
  "name": "dashscope",
  "type": "dashscope",
  "base_url": "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
  "enabled": true,
  "config": {
    "timeout_ms": 30000
  }
}
```

Secret fields must not be returned.

### Model-Provider Mapping CRUD

```text
GET    /api/admin/model-providers
POST   /api/admin/model-providers
PATCH  /api/admin/model-providers/:id
DELETE /api/admin/model-providers/:id
```

Mapping fields:

```json
{
  "model_id": "uuid",
  "provider_id": "uuid",
  "provider_model_name": "qwen-plus",
  "priority": 10,
  "enabled": true,
  "supports_streaming": true
}
```

## Error Format

All API errors should follow:

```json
{
  "error": {
    "code": "insufficient_balance",
    "message": "Insufficient balance to complete this request.",
    "type": "billing_error",
    "request_id": "req_xxx"
  }
}
```

## Error Code Categories

`type` is derived from `error.code` by `backend/internal/gateway/router.go:errorTypeForCode`.

| Type | Codes |
|---|---|
| `auth_error` | `missing_auth`, `invalid_auth_format`, `authentication_error`, `invalid_api_key`, `expired_token` |
| `billing_error` | `insufficient_balance`, `quota_exceeded` |
| `rate_limit_error` | `rate_limit_exceeded` |
| `permission_error` | `permission_denied` |
| `provider_error` | `provider_timeout`, `provider_unavailable`, `upstream_error` |
| `routing_error` | `no_available_provider`, `model_not_found` |
| `validation_error` | `invalid_request`, `invalid_request_error`, `unsupported_parameter` |
| `internal_error` | `internal_error`, `internal_server_error`, uncategorized codes |

## Standard Error Codes

| HTTP | Code | Type | Retry | Meaning / Client Action |
|---:|---|---|---:|---|
| 400 | `invalid_request` | `validation_error` | No | Request body, path param, query param, file upload, or required field is invalid. Fix request and retry. |
| 400 | `invalid_request_error` | `validation_error` | No | OpenAI-compatible validation error alias. Fix request and retry. |
| 400 | `unsupported_parameter` | `validation_error` | No | Parameter is recognized but unsupported by current gateway/provider path. Remove or adjust parameter. |
| 401 | `missing_auth` | `auth_error` | No | Authorization header is missing. Send JWT or API key. |
| 401 | `invalid_auth_format` | `auth_error` | No | Authorization header is malformed. Use `Authorization: Bearer <token-or-key>`. |
| 401 | `authentication_error` | `auth_error` | No | JWT/login credentials are invalid or token validation failed. Re-authenticate. |
| 401 | `invalid_api_key` | `auth_error` | No | API key is revoked, missing, or invalid. Create/use a valid key. |
| 401 | `expired_token` | `auth_error` | No | JWT expired. Refresh or log in again. |
| 402 | `insufficient_balance` | `billing_error` | No | Prepaid balance is depleted. Top up balance or switch billing mode. |
| 403 | `permission_denied` | `permission_error` | No | Current role/key scope cannot access resource/model. Use an allowed key or admin account. |
| 404 | `not_found` | `internal_error` | No | Resource is missing or not owned by current user. Check ID and ownership. |
| 404 | `model_not_found` | `routing_error` | No | Model is unavailable or has no routable provider. Pick another model or configure provider binding. |
| 413 | `invalid_request` | `validation_error` | No | Uploaded file exceeds `max_file_size_mb`. Reduce file size. |
| 415 | `invalid_request` | `validation_error` | No | Uploaded MIME type is not allowed by `allowed_upload_mime_types`. |
| 429 | `rate_limit_exceeded` | `rate_limit_error` | Yes | RPM/TPM limit exceeded. Back off and retry after the rate window. |
| 429 | `quota_exceeded` | `billing_error` | Later | Workspace budget/quota exceeded. Wait for reset or increase quota. |
| 500 | `internal_error` | `internal_error` | Maybe | Unexpected internal failure. Retry once; inspect request_id in logs if persistent. |
| 500 | `internal_server_error` | `internal_error` | Maybe | Internal service/storage failure. Retry once; inspect request_id in logs if persistent. |
| 500 | `workflow_failed` | `internal_error` | Maybe | Workflow execution failed. Inspect workflow run detail. |
| 502 | `provider_unavailable` | `provider_error` | Yes | Upstream provider failed or all providers failed. Gateway may have attempted fallback. Retry or use another model/provider. |
| 502 | `provider_error` | `internal_error` | Yes | Provider-specific task submission failed. Retry or inspect provider config. |
| 503 | `service_unavailable` | `internal_error` | Yes | Internal subsystem, such as task engine, is unavailable. Retry after service recovery. |

Notes:

```text
request_id should be included in support/debug reports.
Request Logs store error_code and error_message for authenticated /v1 paths.
CSV exports intentionally omit request_preview and response_preview.
Some legacy handlers still emit code aliases such as provider_error or not_found; the envelope remains consistent.
```
