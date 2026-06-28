# Roadmap and Sprint Plan

## Sprint 1 — Documentation and Scope Lock

### Goal

Codify the product direction before further development.

### Deliverables

- PDR.md
- SCOPE.md
- ARCHITECTURE.md
- MODULE_BREAKDOWN.md
- DATA_MODEL.md
- API_DESIGN.md
- ROADMAP.md

### Acceptance

- Documents clearly state v0.1 current state.
- Documents clearly state v0.2 implementation scope.
- v0.3 has started with schema foundation only; runtime APIs and enforcement stay behind v0.2 Admin CRUD stabilization.
- No production code changes unless required for doc generation.

## Sprint 2 — v0.1 Acceptance Freeze

### Goal

Make current MVP demonstrable and regression-safe.

### Deliverables

- MVP_ACCEPTANCE_REPORT.md
- Updated API examples
- Smoke test summary
- Known limitations
- Demo walkthrough

### Acceptance

- `docker compose up` works.
- User can create API Key.
- `/v1/models` works.
- `/v1/chat/completions` works.
- usage and billing records are written.

## Sprint 3 — Request Logs / Observability

### Goal

Every request can be traced from frontend to provider to billing.

### Backend Tasks

- Add `request_logs` migration.
- Add storage methods.
- Generate request_id for each request.
- Write logs for success and failure.
- Add request_preview and response_preview truncation.
- Add user API:
  - `GET /api/user/request-logs`
  - `GET /api/user/request-logs/:request_id`

### Frontend Tasks

- Add Request Logs page.
- Add filters: model, provider, status, date.
- Add detail drawer.
- Show request_id, latency, tokens, cost, error.

### Acceptance

- Every chat completion writes request_logs.
- Failed auth and insufficient balance errors are visible where appropriate.
- User can view own logs only.

## Sprint 4 — Provider Health Check / Failover

### Goal

Make provider reliability visible and actionable.

### Backend Tasks

- Add `provider_health_checks` migration.
- Add `fallback_logs` migration.
- Add Provider `HealthCheck(ctx)` interface.
- Implement mock and DashScope health checks.
- Add background health job.
- Router filters unhealthy providers.
- Implement fallback chain.
- Add mock failure mode.
- Add API:
  - `GET /api/admin/provider-health`
  - `POST /api/admin/providers/:id/health-check`

### Frontend Tasks

- Provider Status page.
- Show provider status, last check, latency, error.
- Show fallback count and recent incidents.

### Acceptance

- A failing mock provider triggers fallback.
- Fallback is logged.
- Provider status is visible in UI.

## Sprint 5 — Admin Model / Provider Management

### Goal

Move model/provider operations from seed SQL to Admin UI.

### Backend Tasks

- Admin auth / is_admin enforcement.
- CRUD models.
- CRUD providers.
- CRUD model-provider mappings.
- Update pricing.
- Enable / disable model and provider.
- Update priority.

### API

- `GET /api/admin/models`
- `POST /api/admin/models`
- `PATCH /api/admin/models/:id`
- `DELETE /api/admin/models/:id`
- `GET /api/admin/providers`
- `POST /api/admin/providers`
- `PATCH /api/admin/providers/:id`
- `DELETE /api/admin/providers/:id`
- `GET /api/admin/model-providers`
- `POST /api/admin/model-providers`
- `PATCH /api/admin/model-providers/:id`
- `DELETE /api/admin/model-providers/:id`

### Frontend Tasks

- Admin Models page.
- Admin Providers page.
- Admin Model-Provider Mapping page.

### Acceptance

- Admin can add a model without editing seed SQL.
- Admin can disable a provider and router stops using it.

## Sprint 6 — FinOps / Quota

### Goal

Introduce cost governance.

### Tasks

- API key quota rules.
- Monthly usage summary.
- Cost by model.
- Cost by provider.
- charged_cost_usd, upstream_cost_usd, gross_margin_usd reporting.
- Basic budget alert.

## Sprint 7 — OpenAI-compatible Provider

### Goal

Support any provider exposing OpenAI-compatible API.

### Tasks

- Add OpenAICompatibleProvider.
- Configurable base_url.
- Configurable API key.
- Chat completion.
- Streaming.
- Usage parsing.
- Error normalization.

## Sprint 8 — Marketplace Foundation

### Goal

Start model discovery and comparison.

### Tasks

- Model detail page.
- Tags.
- Capabilities.
- Comparison.
- Pricing page.
- Provider availability.
