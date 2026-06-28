# Module-level Codex Prompts

## Gateway Core Prompt

```text
Improve the Gateway Core without changing the public API contract.

Scope:
- Add request_id middleware.
- Add normalized error response helper.
- Ensure /v1/chat/completions returns OpenAI-compatible success response.
- Ensure errors include request_id.
- Ensure streaming path does not bypass billing or logging if streaming exists.
- Keep existing v0.1 behavior working.

Do not implement workflow or agent logic inside Gateway Core.

Output changed files, tests added, and validation steps.
```

## Provider Adapter Prompt

```text
Refactor or extend provider adapters behind a common Provider interface.

Required interface capabilities:
- ID()
- Type()
- HealthCheck(ctx)
- ChatCompletion(ctx, req)
- ChatCompletionStream(ctx, req) if streaming exists

Normalize:
- response
- usage
- errors
- streaming chunks
- latency metadata

Implement for existing Mock and DashScope providers. Do not break existing provider configuration.

Add tests for error normalization and usage parsing.
```

## Router Prompt

```text
Implement health-aware priority routing and fallback.

Requirements:
- Read model_provider mappings ordered by priority.
- Skip disabled mappings.
- Skip disabled providers.
- Prefer healthy providers when health data exists.
- If primary provider fails with retryable error, fallback to next provider.
- Record fallback_logs.
- Return normalized routing_error when no provider is available.

Do not add cost/latency/quality routing yet; leave extension points.
```

## Billing Prompt

```text
Improve billing to support AI FinOps fields.

Requirements:
- Preserve existing balance deduction and billing_transactions.
- Add or prepare fields for charged_cost_usd, upstream_cost_usd, gross_margin_usd.
- Ensure billing happens exactly once per completed request.
- Ensure failed provider attempts do not incorrectly charge users.
- Ensure fallback success charges based on final model usage.
- Add tests for insufficient balance, normal charge, and zero-token fallback failure.
```

## Observability Prompt

```text
Implement request-level observability.

Requirements:
- request_logs table.
- storage functions.
- write logs on success and failure.
- include request_id, user_id, api_key_id, model_id, provider_id, final_provider_id, status_code, error_code, latency, usage, cost, previews.
- truncate previews.
- no secret logging.
- APIs for user log list and detail.
- frontend Request Logs page.
```

## Admin Prompt

```text
Implement Admin Control Plane for models and providers.

Requirements:
- users.is_admin.
- admin middleware.
- model CRUD.
- provider CRUD.
- model_provider CRUD.
- provider secrets never returned.
- frontend pages for admin models, providers, and mappings.
- tests for non-admin rejection.
```

## Frontend Request Logs Prompt

```text
Create a Request Logs frontend page.

Requirements:
- Table columns: request_id, model, provider, status, latency, tokens, cost, created_at.
- Filters: model, provider, status, date range.
- Detail drawer shows request preview, response preview, error, fallback count, cost breakdown.
- Only use current user's APIs.
- Handle empty state and loading state.
```

## Frontend Provider Status Prompt

```text
Create a Provider Status frontend page.

Requirements:
- Show each provider status: healthy, degraded, unhealthy, unknown.
- Show latency, last checked time, error message.
- Admin button to trigger manual health check if user is admin.
- Show recent fallback events if API exists.
```

## Marketplace Prompt

```text
Implement read-only marketplace foundation.

Requirements:
- Add model tags and capabilities.
- Add model detail page.
- Show price, context window, provider availability, supported features.
- Add comparison page.
- No checkout or ranking algorithm yet.
```
