# Codex Sprint Prompts

## Sprint 1 Prompt — Documentation and Scope Lock

```text
You are working on an existing runnable AI Model Aggregator / AI Gateway MVP. Do not implement new product features yet. First, inspect the repository and create or update the product and architecture documentation.

Read these first:
- README.md
- API_EXAMPLES.md
- MVP_IMPLEMENTATION_PLAN.md
- migrations/
- backend/internal/
- frontend/src/
- scripts/smoke-test.sh
- docker-compose.yml

Then create or update:
- docs/PDR.md
- docs/SCOPE.md
- docs/ARCHITECTURE.md
- docs/MODULE_BREAKDOWN.md
- docs/DATA_MODEL.md
- docs/API_DESIGN.md
- docs/ROADMAP.md

The platform scope must cover:
1. API Resale / Thin Gateway
2. Model Marketplace
3. Self-hosted Inference Provider
4. AI Gateway / Enterprise Governance
5. AI FinOps
6. Smart Routing
7. Reliability / SLA Layer
8. Evaluation / Benchmark
9. Guardrails / Compliance
10. Workflow / Agent API
11. BYOK / BYOM
12. Private / Sovereign AI
13. Data Loop / Fine-tuning / Synthetic Data
14. White-label / OEM / Channel
15. Developer / Agent / Tool Marketplace
16. Procurement / Unified Billing
17. AI Assurance / Risk Management

Important constraints:
- Preserve the existing v0.1 runnable path.
- Do not rewrite the architecture.
- Do not implement workflow, agent, GPU hosting, guardrails, or benchmark in this sprint.
- Explicitly define v0.2 scope as Request Logs, Provider Health Check, Failover, Provider Status Page, Admin Model/Provider Management, unified error format, request_id tracing, and failover smoke test.

After the documentation update, run existing tests and smoke test if available. Output changed files and any detected gaps.
```

## Sprint 2 Prompt — v0.1 Acceptance Freeze

```text
Validate and freeze the current v0.1 MVP behavior. Do not add new functionality unless required to fix a broken documented MVP path.

Acceptance path:
1. Start the stack with docker compose.
2. Register or login.
3. Create an API key.
4. Call GET /v1/models.
5. Call POST /v1/chat/completions with mock provider and configured real provider if available.
6. Confirm usage_logs are written.
7. Confirm billing_transactions are written.
8. Confirm balance deduction works.
9. Confirm Dashboard, Billing, and Playground still render.

Deliverables:
- docs/MVP_ACCEPTANCE_REPORT.md
- docs/DEMO_WALKTHROUGH.md
- updated API_EXAMPLES.md if needed
- updated smoke-test script only if it is currently broken

Do not refactor large code paths. The purpose is to establish a regression baseline before v0.2.
```

## Sprint 3 Prompt — Request Logs / Observability

```text
Implement v0.2 Request Logs and request_id tracing.

Requirements:
1. Add migration for request_logs.
2. Add storage/repository methods for creating and querying request logs.
3. Generate a request_id for every public API request, especially /v1/chat/completions.
4. Include request_id in responses and errors where appropriate.
5. Write request_logs for both success and failure paths.
6. Capture user_id, api_key_id, model_id, provider_id, final_provider_id, status_code, error_code, error_message, latency_ms, token usage, charged_cost_usd, upstream_cost_usd, gross_margin_usd, fallback_count, request_preview, response_preview.
7. Truncate request_preview and response_preview to a safe length.
8. Never log API keys or provider secrets.
9. Add user APIs:
   - GET /api/user/request-logs
   - GET /api/user/request-logs/:request_id
10. Add frontend Request Logs page with filters and detail view.

Acceptance:
- Successful chat completion writes a request log.
- Failed request writes a request log when user context is known.
- User can only see own request logs.
- Existing usage_logs and billing_transactions still work.
- go test and smoke test pass.
```

## Sprint 4 Prompt — Provider Health Check / Failover

```text
Implement v0.2 provider health check and fallback.

Requirements:
1. Add provider_health_checks migration.
2. Add fallback_logs migration.
3. Extend Provider interface with HealthCheck(ctx).
4. Implement HealthCheck for MockProvider and DashScopeProvider.
5. Add background health check job with configurable interval.
6. Add manual admin health check endpoint:
   - POST /api/admin/providers/:id/health-check
7. Add provider health list endpoint:
   - GET /api/admin/provider-health
8. Modify router to skip disabled or unhealthy providers when alternatives exist.
9. Implement fallback chain based on model_provider priority.
10. Add mock failure mode to force primary provider failure.
11. Write fallback_logs when fallback occurs.
12. Add Provider Status frontend page.
13. Add smoke-test case proving failover works.

Acceptance:
- If primary mock provider fails, request succeeds through fallback provider.
- fallback_logs contains the transition.
- provider_health_checks contains latest status.
- Provider Status UI shows health and errors.
- Existing v0.1 path still works.
```

## Sprint 5 Prompt — Admin Model / Provider Management

```text
Implement v0.2 Admin Model / Provider Management.

Requirements:
1. Add users.is_admin if not already added.
2. Add admin middleware that requires authenticated user and is_admin=true.
3. Implement CRUD APIs:
   - GET/POST/PATCH/DELETE /api/admin/models
   - GET/POST/PATCH/DELETE /api/admin/providers
   - GET/POST/PATCH/DELETE /api/admin/model-providers
4. Admin should be able to:
   - add model
   - edit model
   - enable/disable model
   - add provider
   - edit provider
   - enable/disable provider
   - configure provider base_url safely
   - map model to provider
   - set priority
   - set provider_model_name
   - configure pricing fields
5. Never return raw provider secret to frontend.
6. Add frontend pages:
   - Admin Models
   - Admin Providers
   - Admin Model Providers

Acceptance:
- Admin can create a model from UI.
- Admin can disable a provider and router stops using it.
- Non-admin users cannot access admin APIs.
- Existing v0.1 and v0.2 smoke tests pass.
```

## Sprint 6 Prompt — FinOps / Quota

```text
Implement first-level AI FinOps and quota controls.

Requirements:
1. Add API key quota rules.
2. Add daily/monthly token and spend limits.
3. Track cost by model and provider.
4. Track charged_cost_usd, upstream_cost_usd, and gross_margin_usd.
5. Add user and admin usage summary APIs.
6. Add frontend cost dashboard sections.
7. Ensure insufficient quota returns normalized billing_error or quota_error.

Acceptance:
- Requests are blocked when quota is exceeded.
- Cost by model/provider is visible.
- Gross margin can be calculated.
```

## Sprint 7 Prompt — OpenAI-compatible Provider

```text
Implement OpenAI-compatible Provider Adapter.

Requirements:
1. Add provider type openai_compatible.
2. Support configurable base_url.
3. Support provider API key stored securely.
4. Implement chat completion.
5. Implement streaming if gateway supports streaming.
6. Parse OpenAI-compatible usage.
7. Normalize errors.
8. Add admin config support.
9. Add smoke test using mock OpenAI-compatible endpoint if real key is unavailable.

Acceptance:
- A new OpenAI-compatible provider can be added without code changes.
- A model can route to this provider.
- Usage and billing still work.
```

## Sprint 8 Prompt — Marketplace Foundation

```text
Implement basic Model Marketplace functionality.

Requirements:
1. Add model tags and capabilities.
2. Add model detail page.
3. Add pricing display.
4. Add provider availability display.
5. Add model comparison page.
6. Add recommended use cases field.
7. Keep marketplace read-only for users in this sprint.

Acceptance:
- User can browse models by capability.
- User can compare models by price, context, provider availability, and recommended use case.
```
