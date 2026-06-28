# Module Breakdown

## Backend Target Structure

```text
backend/
├── cmd/
│   └── server/
├── internal/
│   ├── config/
│   ├── auth/
│   ├── gateway/
│   ├── provider/
│   │   ├── mock/
│   │   ├── dashscope/
│   │   ├── openai_compatible/
│   │   ├── self_hosted/
│   │   └── byok/
│   ├── router/
│   │   ├── priority/
│   │   ├── fallback/
│   │   ├── health/
│   │   ├── cost/
│   │   └── quality/
│   ├── billing/
│   ├── finops/
│   ├── ratelimit/
│   ├── observability/
│   ├── guardrails/
│   ├── evaluation/
│   ├── workflow/
│   ├── marketplace/
│   ├── admin/
│   ├── storage/
│   └── middleware/
```

## Backend Modules

### config

- Load environment config.
- Validate required env vars.
- Provide provider-specific config.
- Never print secret values.

### auth

- User registration / login.
- JWT verification.
- API key creation.
- API key hash verification.
- Admin role enforcement.
- Future RBAC.

### gateway

- OpenAI-compatible API.
- Request validation.
- Request ID generation.
- Error normalization.
- Streaming response.
- Calls router and billing.

### provider

Unified interface:

```go
type Provider interface {
    ID() string
    Type() string
    HealthCheck(ctx context.Context) (*HealthStatus, error)
    ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
}
```

Responsibilities:

- Request transformation.
- Response normalization.
- Usage parsing.
- Error normalization.
- Health check.

### router

- Priority routing.
- Health-based routing.
- Fallback routing.
- Future cost / latency / quality routing.

### billing

- Calculate token cost.
- Deduct balance.
- Record billing transactions.
- Track upstream cost.
- Track gross margin.

### finops

- Quota.
- Budget.
- Cost attribution.
- Cost center.
- Usage export.

### ratelimit

- API key limits.
- User limits.
- Workspace limits.
- RPM / TPM support.

### observability

- Request logs.
- Provider health checks.
- Fallback logs.
- Error rate.
- Latency metrics.

### admin

- Model CRUD.
- Provider CRUD.
- Model-provider mapping CRUD.
- Pricing management.
- Credit grants.

### marketplace

- Model tags.
- Capabilities.
- Model detail.
- Comparison.
- Pricing history.

### workflow

- Future workflow engine.
- Must not be coupled into Gateway Core.

## Frontend Target Pages

```text
frontend/src/
├── pages or app/
│   ├── login
│   ├── dashboard
│   ├── api-keys
│   ├── models
│   ├── playground
│   ├── billing
│   ├── request-logs
│   ├── provider-status
│   ├── admin/
│   │   ├── models
│   │   ├── providers
│   │   └── model-providers
│   ├── marketplace
│   ├── workflows
│   └── settings
```

## Frontend Module Responsibilities

### Dashboard

- Total requests.
- Total tokens.
- Total spend.
- Error rate.
- Average latency.

### API Keys

- Create API key.
- Show once.
- Revoke.
- Rotate later.

### Models

- List available models.
- Show price.
- Show provider availability.

### Playground

- Select model.
- Input messages.
- Stream response.
- Show usage and cost.

### Billing

- Balance.
- Transactions.
- Usage charges.
- Credit grants.

### Request Logs

- Request list.
- Filters.
- Detail drawer.
- Error detail.
- Token and cost detail.

### Provider Status

- Provider status.
- Last health check.
- Error rate.
- Latency.
- Recent fallbacks.

Current implementation:

```text
Admin route: /admin/provider-status
Frontend file: frontend/src/pages/Admin.tsx
API client: frontend/src/lib/api.ts
Backend handlers:
  gateway.adminProviderHealth()
  gateway.adminProviderHealthCheck()
Storage:
  storage.RecordProviderHealthCheck()
  storage.ListLatestProviderHealth()
```

### Admin Models

- Add model.
- Edit model.
- Disable model.
- Pricing fields.

### Admin Providers

- Add provider.
- Edit provider.
- Disable provider.
- Secret-safe config display.

### Admin Model Providers

- Map model to provider.
- Set priority.
- Set enabled status.
- Set provider-specific model name.
