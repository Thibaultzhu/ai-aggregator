# API Examples

Complete curl examples for every implemented endpoint in the AI Aggregator MVP.

All examples use `BASE_URL=http://localhost:8080`. Adjust if your backend runs elsewhere.

---

## Setup

```bash
BASE_URL=http://localhost:8080
```

**Tip:** If the backend is running with `MOCK_PROVIDER_MODE=true`, chat completion requests will return canned responses without calling the real DashScope API. This is useful for testing the examples below without an API key.

---

## Health Check

No authentication required.

```bash
curl -s $BASE_URL/health | jq .
```

**Response:**
```json
{
  "status": "ok",
  "version": "0.1.0-mvp"
}
```

---

## Prometheus Metrics

No authentication required. Returns Prometheus-formatted metrics.

```bash
curl -s $BASE_URL/metrics
```

**Response (excerpt):**
```
# HELP aggregator_requests_total Total number of API requests
# TYPE aggregator_requests_total counter
aggregator_requests_total{model="qwen-max",provider="bailian_cn",status="200",modality="text",stream="false"} 42
# HELP aggregator_tokens_total Total tokens processed
# TYPE aggregator_tokens_total counter
aggregator_tokens_total{model="qwen-max",direction="input"} 12345
aggregator_tokens_total{model="qwen-max",direction="output"} 6789
```

---

## User Registration

Create a new account. Returns a JWT token and user object. New users receive $10 in credits.

```bash
curl -s -X POST $BASE_URL/api/user/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "dev@example.com",
    "username": "dev",
    "password": "securepass123"
  }' | jq .
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "email": "dev@example.com",
    "username": "dev",
    "role": "user"
  }
}
```

Save the token for subsequent requests:

```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

---

## User Login

Authenticate with email and password. Returns a JWT token valid for 24 hours.

```bash
curl -s -X POST $BASE_URL/api/user/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "dev@example.com",
    "password": "securepass123"
  }' | jq .
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "email": "dev@example.com",
    "username": "dev",
    "role": "user"
  }
}
```

---

## Create API Key

Requires JWT. The plaintext key is returned **only once** -- store it securely.

`workspace_id` is optional. When present, usage from this API key is attributed to that workspace in request logs, usage logs, billing transactions, and workspace usage summary.

```bash
curl -s -X POST $BASE_URL/api/user/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app"}' | jq .
```

**Response:**
```json
{
  "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
  "name": "my-app",
  "key": "sk-aag-a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
  "prefix": "sk-aag-",
  "workspace_id": ""
}
```

Workspace-bound example:

```bash
curl -s -X POST $BASE_URL/api/user/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "workspace-app",
    "workspace_id": "338527fb-6956-49e0-b1f4-a5d3d6c13f76"
  }' | jq .
```

Save the API key:

```bash
API_KEY="sk-aag-a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
```

---

## List API Keys

Requires JWT. Returns all API keys for the authenticated user (hashes are never exposed).

```bash
curl -s $BASE_URL/api/user/keys \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response:**
```json
{
  "data": [
    {
      "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
      "name": "my-app",
      "key_prefix": "sk-aag-",
      "workspace_id": "338527fb-6956-49e0-b1f4-a5d3d6c13f76",
      "permissions": null,
      "is_active": true,
      "last_used_at": "2025-01-15T10:30:00Z",
      "created_at": "2025-01-15T09:00:00Z"
    }
  ]
}
```

---

## Delete (Revoke) API Key

Requires JWT. Deactivates the key (soft delete).

```bash
curl -s -X DELETE $BASE_URL/api/user/keys/b2c3d4e5-f6a7-8901-bcde-f12345678901 \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response:**
```json
{
  "deleted": true,
  "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
}
```

---

## List Models

Requires API key (or JWT). Returns all active models in OpenAI-compatible format.

```bash
curl -s $BASE_URL/v1/models \
  -H "Authorization: Bearer $API_KEY" | jq .
```

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "qwen-max",
      "object": "model",
      "created": 1736900000,
      "owned_by": "alibaba",
      "display_name": "通义千问 Max",
      "modality": "text"
    },
    {
      "id": "qwen-plus",
      "object": "model",
      "created": 1736900000,
      "owned_by": "alibaba",
      "display_name": "通义千问 Plus",
      "modality": "text"
    },
    {
      "id": "deepseek-r1",
      "object": "model",
      "created": 1736900000,
      "owned_by": "alibaba",
      "display_name": "DeepSeek R1",
      "modality": "text"
    }
  ]
}
```

---

## Create Embeddings

Requires API key. Returns an OpenAI-compatible embeddings response.

```bash
curl -s -X POST $BASE_URL/v1/embeddings \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-v3",
    "input": "hello embedding"
  }' | jq .
```

---

## Image Generation

Requires API key. Image generation is asynchronous.

```bash
curl -s -X POST $BASE_URL/v1/images/generations \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "wan-image",
    "prompt": "a simple red cube",
    "n": 1,
    "size": "1024*1024"
  }' | jq .
```

Poll task status:

```bash
curl -s $BASE_URL/v1/images/generations/task_xxx \
  -H "Authorization: Bearer $API_KEY" | jq .
```

---

## Video Generation

Requires API key. Video generation is asynchronous.

```bash
curl -s -X POST $BASE_URL/v1/video/generations \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "wan2.7-t2v",
    "prompt": "a simple red cube rotating",
    "duration": 5,
    "resolution": "720p"
  }' | jq .
```

Poll task status:

```bash
curl -s $BASE_URL/v1/video/generations/task_xxx \
  -H "Authorization: Bearer $API_KEY" | jq .
```

---

## Chat Completion (Non-Streaming)

Requires API key (or JWT). Sends a synchronous request and returns the full response.

```bash
curl -s -X POST $BASE_URL/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-turbo",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "temperature": 0.7,
    "max_tokens": 256
  }' | jq .
```

**Response:**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1736900100,
  "model": "qwen-turbo",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 28,
    "completion_tokens": 8,
    "total_tokens": 36
  }
}
```

---

## Chat Completion (Streaming)

Requires API key (or JWT). Returns Server-Sent Events (SSE) with incremental content deltas.

```bash
curl -N -X POST $BASE_URL/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-turbo",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Write a haiku about programming."}
    ],
    "stream": true,
    "temperature": 0.8
  }'
```

**SSE Output:**
```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1736900200,"model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1736900200,"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"Code"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1736900200,"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":" flows"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1736900200,"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":" like"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1736900200,"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":" water"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1736900200,"model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":24,"completion_tokens":12,"total_tokens":36}}

data: [DONE]
```

Usage information is included in the final chunk before `[DONE]`. The gateway captures this to record billing metrics.

---

## Get Balance

Requires JWT. Returns the current USD balance.

```bash
curl -s $BASE_URL/api/user/billing/balance \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response:**
```json
{
  "balance_usd": 9.9978
}
```

The balance starts at $10.00 (new user credit) and decreases with each API call based on token usage and the configured markup.

---

## Get Billing Transactions

Requires JWT. Returns a paginated list of billing transactions (credit grants, usage charges, top-ups).

```bash
curl -s $BASE_URL/api/user/billing/transactions \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response:**
```json
{
  "data": [
    {
      "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
      "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "type": "credit_grant",
      "amount": 10.0,
      "description": "New user signup credit",
      "reference_id": null,
      "created_at": "2025-01-15T09:00:00Z"
    },
    {
      "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
      "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "type": "usage_charge",
      "amount": -0.000081,
      "description": "qwen-turbo: 28 input + 8 output tokens",
      "reference_id": "req-a1b2c3d4-e5f6-7890",
      "created_at": "2025-01-15T10:35:00Z"
    }
  ]
}
```

Transaction types:
- `credit_grant` -- initial signup credit or admin top-up (positive amount)
- `usage_charge` -- deduction for API usage (negative amount)
- `refund` -- reversal of a charge (positive amount)

---

## Get Usage Logs

Requires JWT. Returns the most recent 50 usage records with token counts, costs, and metadata.

```bash
curl -s $BASE_URL/api/user/usage \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response:**
```json
{
  "data": [
    {
      "request_id": "req-a1b2c3d4-e5f6-7890",
      "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "api_key_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
      "model_id": "qwen-turbo",
      "provider_id": "bailian_cn",
      "modality": "text",
      "input_tokens": 28,
      "output_tokens": 8,
      "latency_ms": 423,
      "ttft_ms": 0,
      "is_stream": false,
      "upstream_cost": 0.000062,
      "charged_cost": 0.000081,
      "status_code": 200,
      "error_type": "",
      "is_cached": false,
      "region": "",
      "created_at": "2025-01-15T10:35:00Z"
    }
  ]
}
```

---

## Dashboard

Requires JWT. Returns aggregate usage statistics.

```bash
curl -s $BASE_URL/api/user/dashboard \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Response:**
```json
{
  "total_requests": 15,
  "total_cost": 0.00234,
  "total_tokens": 4521,
  "balance": 9.9978
}
```

---

## Error Responses

All errors follow the OpenAI-compatible error format:

```json
{
  "error": {
    "message": "descriptive error message",
    "type": "error_type",
    "request_id": "req-abc123"
  }
}
```

### Common error types

| HTTP Status | Error Type              | Description                             |
|-------------|-------------------------|-----------------------------------------|
| 400         | `invalid_request_error` | Malformed request body or missing fields |
| 401         | `authentication_error`  | Invalid or missing auth credentials     |
| 402         | `insufficient_balance`  | Account balance is zero or negative      |
| 404         | `model_not_found`       | Requested model is not available        |
| 429         | `rate_limit_exceeded`   | Too many requests per minute            |
| 502         | `upstream_error`        | Provider (DashScope) returned an error  |

### Example: Rate limit exceeded

```json
{
  "error": {
    "message": "rate limit exceeded (requests per minute)",
    "type": "rate_limit_exceeded",
    "code": "rpm_limit",
    "retry_after": 5
  }
}
```

Rate limit headers are included in every response:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
Retry-After: 5
```

---

## Using with OpenAI SDK (Python)

Since the API is OpenAI-compatible, you can use the standard OpenAI Python SDK:

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-aag-your-api-key-here",
    base_url="http://localhost:8080/v1"
)

# Non-streaming
response = client.chat.completions.create(
    model="qwen-max",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Explain quantum computing in one sentence."}
    ]
)
print(response.choices[0].message.content)

# Streaming
stream = client.chat.completions.create(
    model="qwen-max",
    messages=[
        {"role": "user", "content": "Write a short poem about AI."}
    ],
    stream=True
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
print()
```

---

## Using with OpenAI SDK (Node.js / TypeScript)

```typescript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-aag-your-api-key-here",
  baseURL: "http://localhost:8080/v1",
});

const response = await client.chat.completions.create({
  model: "qwen-plus",
  messages: [
    { role: "user", content: "What are the benefits of TypeScript?" },
  ],
});

console.log(response.choices[0].message.content);
```

---

## v0.2 Observability and Provider Health

### Admin Model / Provider Management

Admin JWT is required.

```bash
curl -s "$BASE_URL/api/admin/models" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

```bash
curl -s "$BASE_URL/api/admin/models/qwen-plus" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Update pricing or status:

```bash
curl -s -X PUT "$BASE_URL/api/admin/models/qwen-plus" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "通义千问 Plus",
    "modality": "text",
    "capabilities": ["chat", "completion", "function_call"],
    "input_price": 0.0008,
    "output_price": 0.002,
    "price_unit": "per_1k_tokens",
    "supports_stream": true,
    "is_async": false,
    "status": "active"
  }' | jq .
```

Model-provider binding:

```bash
curl -s -X PUT "$BASE_URL/api/admin/models/qwen-plus/providers/bailian_cn" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "priority": 1,
    "upstream_model": "qwen-plus",
    "cost_multiplier": 1,
    "timeout_ms": 30000,
    "max_retries": 2,
    "is_enabled": true
  }' | jq .
```

Admin writes refresh the router registry, so price/status/binding changes are used by the next request.

### Request Logs

```bash
curl -s "$BASE_URL/api/user/request-logs?limit=20" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

```bash
REQUEST_ID="req-or-fiber-request-id"
curl -s "$BASE_URL/api/user/request-logs/$REQUEST_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Provider Health (Admin JWT required)

```bash
curl -s "$BASE_URL/api/admin/provider-health" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Trigger a manual provider health check:

```bash
curl -s -X POST "$BASE_URL/api/admin/providers/bailian_cn/health-check" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

### Local fallback test

In mock mode, force one provider to fail chat/stream calls while leaving health checks healthy:

```bash
MOCK_FAIL_PROVIDER_IDS=bailian_cn docker compose up -d --build backend
```

Then call `/v1/chat/completions` for a model with a secondary provider. Expected result:

```text
request_logs.final_provider_id = secondary provider
request_logs.fallback_count > 0
fallback_logs contains bailian_cn -> secondary provider
```

Or run the optional fallback smoke-test against a backend that is already started with `MOCK_FAIL_PROVIDER_IDS=bailian_cn`:

```bash
BASE_URL=http://localhost:8081 MOCK_PROVIDER_MODE=true RUN_FALLBACK_SMOKE=true bash scripts/smoke-test.sh
```

Restore normal behavior:

```bash
MOCK_FAIL_PROVIDER_IDS= docker compose up -d --build backend
```

---

## v0.3 Enterprise Foundation

The v0.3 schema foundation is in:

```text
migrations/004_v03_enterprise_foundation.sql
```

It adds organization/workspace/RBAC/FinOps base tables and nullable workspace attribution fields. Current scope is schema only; organization/workspace APIs, RBAC enforcement, budget blocking, and workspace usage UI are still planned.

### Organization / Workspace Admin APIs

Admin JWT is required.

Create an organization:

```bash
curl -s -X POST "$BASE_URL/api/admin/organizations" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme AI",
    "slug": "acme-ai",
    "billing_mode": "prepaid"
  }' | jq .
```

Create a workspace:

```bash
curl -s -X POST "$BASE_URL/api/admin/workspaces" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "organization_id": "org-uuid",
    "name": "Production",
    "slug": "production",
    "monthly_budget_usd": 1000
  }' | jq .
```

Add a workspace member:

```bash
curl -s -X POST "$BASE_URL/api/admin/workspaces/$WORKSPACE_ID/members" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-uuid",
    "role_name": "member",
    "status": "active"
  }' | jq .
```

Workspace usage summary:

```bash
curl -s "$BASE_URL/api/admin/workspaces/$WORKSPACE_ID/usage" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Current v0.3 limitations:

```text
API keys can be bound to workspace_id, but automatic default workspace assignment is not implemented yet.
Budget / quota tables exist, but runtime enforcement is not implemented yet.
RBAC roles exist at schema level, but request authorization still uses admin JWT only.
Audio endpoints are wired through the gateway and DashScope adapter:
`POST /v1/audio/speech` returns provider audio bytes, and
`POST /v1/audio/transcriptions` accepts multipart uploads and returns transcription text.
The local DashScope audio chain is covered by `scripts/regression/dashscope-audio-chain.sh`.
```
