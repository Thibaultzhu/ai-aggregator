# MVP Implementation Plan

Status of all planned features for the AI Aggregator platform.

---

## Completed

### Storage Layer (`internal/storage/store.go`)

- PostgreSQL connection pool (pgx/v5, 25 max connections, health checks)
- Redis client (go-redis/v9, 20 pool size, LRU eviction)
- Full CRUD for users, API keys, models, providers, usage logs, async tasks, billing transactions, system settings
- Usage summary aggregation (total requests, cost, tokens per user)
- Daily usage calculation for quota enforcement
- Conditional balance deduction (fails if insufficient funds)
- API key validation by SHA-256 hash with async last_used_at update
- System settings with Redis cache layer (30s TTL)

### Authentication (`internal/auth/middleware.go`)

- JWT generation and validation (HMAC-SHA256, 24-hour expiry)
- API key generation (crypto/rand, 32 random bytes, hex-encoded with configurable prefix)
- API key hashing (SHA-256) for secure storage
- Dual authentication middleware: `RequireAuth` (accepts JWT or API key), `RequireJWT` (JWT only), `RequireAdmin` (JWT + admin role)
- User context extraction (user_id, api_key_id, auth_type, permissions)

### Gateway & HTTP Handlers (`internal/gateway/router.go`)

- Fiber v2 HTTP server with graceful shutdown
- Request ID middleware (auto-generated per request)
- CORS middleware (allows all origins in MVP)
- Recovery middleware (panic protection)
- Structured error responses in OpenAI-compatible format
- OpenAI-compatible chat completion handler (both streaming and non-streaming)
- Model listing endpoint (`/v1/models`)
- User registration with bcrypt password hashing and $10 initial credit
- User login with password verification
- API key CRUD (create, list, revoke)
- Balance retrieval
- Usage log retrieval (last 50 records)
- Dashboard with aggregate statistics
- Health check and Prometheus metrics endpoints

### DashScope Provider Adapter (`internal/provider/`)

- `Adapter` interface defining all modality methods (chat, image, video, audio, embedding, health check)
- `DashScopeAdapter` implementation supporting:
  - Non-streaming chat completions via OpenAI-compatible endpoint
  - Streaming chat completions with SSE parsing (bufio scanner, 1MB buffer)
  - Image generation via native DashScope API (async task submission)
  - Video generation via native DashScope API (async task submission)
  - Async task polling (status check, result retrieval)
  - Text embeddings via OpenAI-compatible endpoint
  - Health check (GET /models endpoint)
- Error parsing for both OpenAI-compatible and native DashScope error formats
- Status mapping (PENDING -> pending, RUNNING -> processing, SUCCEEDED -> completed, FAILED -> failed)
- UnsupportedModality error type for graceful degradation

### Model Router (`internal/router/model_router.go`)

- Priority-based provider selection (ascending priority order)
- Health-aware routing (skips providers marked as "down")
- Provider adapter initialization from database configuration
- API key resolution from environment variables (DASHSCOPE_API_KEY_CN, DASHSCOPE_API_KEY_INTL)
- In-memory model registry with mutex-protected reads
- Background health check loop (30-second interval, calls each adapter's HealthCheck)
- Registry refresh on notification (channel-based trigger)
- Support for model-to-provider bindings with upstream model name mapping

### Rate Limiting (`internal/ratelimit/limiter.go`)

- Sliding-window RPM (requests per minute) limiting using Redis sorted sets
- Algorithm: ZRemRangeByScore (remove expired) -> ZCard (count) -> ZAdd (add new) -> Expire (TTL)
- Per-key or per-user limiting (uses api_key_id if present, falls back to user_id)
- Retry-After header calculation based on oldest entry in window
- TPM (tokens per minute) counter using Redis INCRBY
- Concurrency tracking (increment/decrement counters)
- Fail-open on Redis errors (allows request through, logs warning)
- Response headers: X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After

### Stream Proxy (`internal/stream/proxy.go`)

- SSE relay from upstream to client via Fiber's SetBodyStreamWriter
- Proper SSE headers (Content-Type: text/event-stream, Cache-Control: no-cache, X-Accel-Buffering: no)
- JSON serialization of each StreamChunk with `data: ` prefix
- Final `[DONE]` marker sent after stream completion
- Usage accumulation from final chunk (DashScope sends usage in last chunk)
- StreamResult channel for post-stream usage capture

### Billing Engine (`internal/billing/engine.go`)

- Pre-check middleware for balance and daily quota enforcement
- Balance check (rejects if balance <= 0, returns 402)
- Daily quota check (rejects if daily spend >= quota, returns 429)
- Fail-open on database errors (allows request, logs warning)
- Cost calculation framework (per-modality pricing: tokens, images, seconds, characters)
- Async usage recording with balance deduction

### Task Engine (`internal/task/engine.go`)

- Worker pool for async task polling (configurable worker count, default 5)
- Task submission flow: route to provider -> submit to upstream -> store in database
- Task polling loop: fetch pending tasks -> poll upstream -> update status
- Support for image and video task types
- Graceful shutdown with context cancellation and WaitGroup

### Metrics Collector (`internal/metrics/collector.go`)

- Prometheus integration via prometheus/client_golang
- Metrics exposed:
  - `aggregator_requests_total` (counter, labels: model, provider, status, modality, stream)
  - `aggregator_request_duration_seconds` (histogram, exponential buckets 50ms to 819s)
  - `aggregator_ttft_seconds` (histogram, time-to-first-token for streams)
  - `aggregator_tokens_total` (counter, labels: model, direction: input/output)
  - `aggregator_cost_usd_total` (counter, labels: model, provider, type: upstream/charged)
  - `aggregator_async_tasks_inflight` (gauge, label: model)
  - `aggregator_rate_limit_rejections_total` (counter, labels: key_id, type)
  - `aggregator_active_streams` (gauge, label: model)
- HTTP handler for `/metrics` endpoint via promhttp + fasthttp adaptor

### Configuration (`internal/config/config.go`)

- Environment-based configuration with sensible defaults
- Required field validation (DATABASE_URL always, JWT_SECRET in production)
- All settings configurable via env vars

### Database Schema (`migrations/`)

- 12 tables: users, api_keys, providers, provider_keys, models, model_providers, usage_logs, async_tasks, semantic_cache, audit_logs, system_settings, billing_transactions
- Indexes on all foreign keys and common query patterns
- Auto-update triggers for updated_at timestamps
- Default system settings seeded
- pgvector extension support (for semantic cache)
- Seed data for 3 providers and 30+ models across all modalities

### Frontend Scaffold (`frontend/`)

- React 18 + TypeScript + Vite 6
- Tailwind CSS 3 for styling
- React Router v6 with layout-based routing
- Zustand for state management
- Recharts for data visualization
- Lucide React for icons
- Pages implemented:
  - Landing page
  - Model browser
  - Pricing page
  - Playground (API testing interface)
  - Documentation page
  - Login page
  - User Dashboard
  - API Keys management
  - Billing page
  - Admin panel (scaffold)
- Layout components: MainLayout (header + footer), DashboardLayout (sidebar)
- Docker-ready with Vite dev server

### Deployment (`docker-compose.yml`, Dockerfiles)

- Docker Compose with 4 services (postgres, redis, backend, frontend)
- Health checks on postgres and redis (backend waits for both)
- Multi-stage Dockerfile for backend (golang:1.22-alpine builder, alpine:3.20 runtime)
- Frontend Dockerfile with Node.js 20
- Database auto-initialization via initdb.d volume mount
- Environment variable passthrough via .env file

### Mock Provider Mode (`MOCK_PROVIDER_MODE`)

- **COMPLETED** -- When `MOCK_PROVIDER_MODE=true`, the backend registers a mock provider adapter that returns canned responses for all chat completion requests
- No real DashScope API key needed for development, testing, or CI
- Mock responses include realistic token counts and proper OpenAI-compatible response format
- Smoke test supports mock mode via `MOCK_PROVIDER_MODE` env var

### Requested Model Fix (`requestedModel`)

- **COMPLETED** -- Fixed the issue where the `requestedModel` variable was not correctly resolved from the incoming request body
- Model ID is now properly extracted from the request and used for routing, usage recording, and billing
- Ensures correct model-to-provider mapping for all modalities

### Synchronous Usage Recording

- **COMPLETED** -- Usage records and balance deductions are now recorded synchronously within the request lifecycle (for non-streaming requests)
- Eliminates race conditions where near-zero balance users could send multiple requests before async deductions applied
- Streaming requests still use async recording via the stream proxy's post-completion channel to avoid blocking the SSE relay
- Balance check + deduction happens atomically to prevent overdraw

### Billing Transactions

- **COMPLETED** -- `GET /api/user/billing/transactions` endpoint implemented
- Returns paginated list of billing transactions for the authenticated user
- Transaction types: `credit_grant` (signup credit, admin top-up), `usage_charge` (API usage deductions), `refund` (charge reversals)
- `credit_grant` transaction recorded during user registration alongside the $10 balance
- `usage_charge` transaction recorded for each chat completion alongside usage log and balance deduction
- Storage layer `CreateBillingTransaction()` wired to the gateway handler

---

## Partially Implemented

### Streaming Chat Completions

- **Status:** Working end-to-end
- **What works:** SSE relay from DashScope to client, proper headers, chunk-by-chunk forwarding, [DONE] marker, usage capture from final chunk
- **What could improve:** The request-level Prometheus metric (`RecordRequest`) is recorded at stream initiation time, not completion time, so the latency value is inaccurate for streaming requests. Token-level and cost metrics are correctly recorded after stream completion via the wrapped channel goroutine. TTFT (time-to-first-token) measurement is defined in the metrics collector but not yet wired into the stream relay path.

### Cost Calculation

- **Status:** Working with hardcoded defaults
- **What works:** Cost formula `(input_tokens * input_price + output_tokens * output_price) / 1000 * markup` is applied to every request, and charged cost is deducted from balance
- **What could improve:** Currently uses hardcoded defaults ($0.002/1K input, $0.006/1K output, 1.3x markup) for all models instead of looking up per-model pricing from the database. The `calculateCost` function has a TODO to integrate with `modelRouter.GetModel()` for model-specific pricing.

### Billing Engine

- **Status:** Mostly complete -- PreCheck middleware still commented out
- **What works:** Balance check is done inline in the chat completion handler. Usage recording and balance deduction happen synchronously for non-streaming requests. Billing transactions (`credit_grant`, `usage_charge`) are recorded for every operation. The `billing.Engine` has full PreCheck middleware (balance + daily quota) but is commented out in the gateway constructor (`// TODO: re-enable after MVP`)
- **What could improve:** Re-enable the billing PreCheck middleware on the /v1 route group for consistent enforcement across all API endpoints (not just chat completions). Add transaction pagination support.

### Task Engine

- **Status:** Code complete but not wired into HTTP handlers
- **What works:** Worker pool, task submission, polling, and status updates are all implemented. The engine starts and runs in the background.
- **What could improve:** The image/video HTTP handlers return 501 instead of delegating to the task engine. The `// Tasks: taskEngine` dependency is commented out in the gateway constructor. The `generateTaskID` function uses timestamp-based IDs instead of random hex.

---

## Not Yet Implemented

### Image Generation (`/v1/images/generations`)

- DashScope adapter has `CreateImage()` implementation
- Task engine has image submission and polling logic
- HTTP handler returns 501
- **Effort:** Wire handler to task engine, add result formatting

### Video Generation (`/v1/video/generations`)

- DashScope adapter has `CreateVideo()` implementation
- Task engine has video submission logic
- HTTP handler returns 501
- **Effort:** Wire handler to task engine, add result formatting

### Audio Transcription (`/v1/audio/transcriptions`)

- DashScope adapter returns `ErrUnsupportedModality`
- DashScope uses native API format (not OpenAI-compatible) for Paraformer
- **Effort:** Implement multipart upload parsing, DashScope native API mapping

### Audio Speech (`/v1/audio/speech`)

- DashScope adapter returns `ErrUnsupportedModality`
- DashScope CosyVoice uses WebSocket-based streaming API
- **Effort:** Implement WebSocket client, streaming audio relay

### Text Embeddings (`/v1/embeddings`)

- DashScope adapter has `CreateEmbedding()` implementation
- HTTP handler returns 501
- **Effort:** Wire handler to adapter, add usage recording

### File Upload (`/v1/files`)

- No implementation exists
- Requires OSS (Object Storage Service) integration
- **Effort:** Implement multipart upload, OSS storage, file metadata CRUD

### Admin Panel (Backend Handlers)

- All 25+ admin routes are registered with auth middleware
- All handlers return 501
- **Effort:** Implement CRUD for models, providers, users; analytics queries; settings management

### Admin Panel (Frontend)

- Admin page scaffold exists (`/admin/*` route)
- No functional components
- **Effort:** Build dashboards, tables, forms for admin operations

### Semantic Cache

- Database table exists with pgvector column and IVFFlat index
- No application logic implemented
- **Effort:** Implement embedding generation, cosine similarity search, cache hit/miss logic

### OAuth 2.0 / SSO

- Only email/password authentication exists
- **Effort:** Integrate with Alibaba Cloud RAM, Google OAuth, or similar

### Multi-Region Deployment

- Global Accelerator provider entry exists in seed data (bailian_ga)
- No region-aware routing logic
- **Effort:** Implement geo-routing, latency-based provider selection

### Token Refresh

- `/api/user/auth/refresh` returns 501
- **Effort:** Implement refresh token rotation, store refresh tokens

### User Profile

- `/api/user/profile` GET/PUT return 501
- **Effort:** Implement profile read/update with validation

### ClickHouse Analytics

- Config field exists (`CLICKHOUSE_URL`)
- No integration code
- **Effort:** Implement dual-write to ClickHouse, build analytics queries

---

## Known Risks

### DashScope API Key Required for Real Testing

The chat completion endpoints proxy to Alibaba Cloud DashScope. Without a valid `DASHSCOPE_API_KEY_CN` environment variable, the upstream provider will reject requests (typically 401). The smoke test accounts for this by treating 502 responses as expected when no key is configured. For end-to-end testing with real model responses, a DashScope API key must be provisioned.

### Hardcoded Pricing

All cost calculations use hardcoded defaults ($0.002/1K input tokens, $0.006/1K output tokens, 1.3x markup) regardless of the model being used. In production, models like `qwen-turbo` ($0.0003/$0.0006) and `qwen-max` ($0.004/$0.012) have very different actual costs. This means the charged amounts and balance deductions are inaccurate. Model-specific pricing should be read from the `models` table (which already stores `input_price` and `output_price` per model).

### No OAuth / SSO

Authentication is email/password only with bcrypt hashing. There is no support for OAuth 2.0, SAML, or any third-party identity provider. This limits enterprise adoption.

### No Rate Limit Per-Key Overrides

The `api_keys` table has `rate_limit_rpm` and `rate_limit_tpm` columns, but the rate limiter always uses the global defaults (`DEFAULT_RPM`, `DEFAULT_TPM`). Per-key rate limit customization is not functional.

### Async Usage Recording (Streaming Only)

Non-streaming requests now record usage synchronously. Streaming requests still use goroutines (`go gw.recordUsage(...)`) for post-stream recording. While this keeps SSE response times low, it means:
- If the server crashes immediately after a stream completes, the usage record may be lost
- Streaming-specific race conditions are mitigated but not fully eliminated

### No Input Validation on Chat Messages

The chat completion handler validates that `model` and `messages` are present, but does not validate message roles, content structure, or token count limits. Malformed messages are passed through to the upstream provider.

### Single-Region Deployment

The application runs as a single instance with no horizontal scaling, load balancing, or multi-region support. The `region` column in usage_logs defaults to 'jkt' but is never dynamically set.

### JWT Secret in .env.example

The example `.env` file contains a plaintext JWT secret (`dev-secret-change-in-production-32chars!!`). While the config loader requires a different secret in production mode, there is no enforcement of secret strength or rotation.

### No Request Logging Persistence

HTTP access logs go to stdout (via Fiber logger middleware in development mode). There is no persistent request log, no correlation with usage_logs, and no centralized log aggregation.

---

## Known Limitations

### Pricing

- All cost calculations use hardcoded defaults ($0.002/1K input tokens, $0.006/1K output tokens, 1.3x markup) regardless of the actual model
- Model-specific pricing from the `models` table (`input_price`, `output_price` columns) is not yet wired into the billing engine
- Balance deductions may not reflect actual upstream costs

### Authentication & Security

- Email/password only -- no OAuth 2.0, SAML, or SSO support
- JWT secret is static and has no rotation mechanism
- No email verification for new user registration
- Password reset flow not implemented
- API key permissions field exists but is not enforced

### Rate Limiting

- Per-key rate limit overrides (`rate_limit_rpm`, `rate_limit_tpm` in `api_keys` table) are not functional
- All keys use global defaults from `DEFAULT_RPM` and `DEFAULT_TPM`
- No rate limiting on user portal endpoints (only on `/v1/*` API routes)

### Streaming

- Latency metrics for streaming requests are inaccurate (recorded at stream start, not completion)
- TTFT (time-to-first-token) measurement is defined but not wired into stream relay
- Stream error recovery is not implemented -- upstream errors are relayed directly to the client

### Deployment

- Single-instance deployment only -- no horizontal scaling or load balancing
- No health check for the backend in Docker Compose
- No SSL/TLS termination -- must be handled by a reverse proxy in production
- `region` column in usage_logs defaults to 'jkt' and is never dynamically set

### Data & Observability

- No persistent request logging (stdout only in development mode)
- No centralized log aggregation
- No alerting or anomaly detection
- Admin analytics endpoints all return 501

### Incomplete Modalities

- Image generation: adapter code exists but HTTP handler returns 501
- Video generation: adapter code exists but HTTP handler returns 501
- Audio (transcription/speech): adapter returns `ErrUnsupportedModality`
- Embeddings: adapter code exists but HTTP handler returns 501
- File upload: no implementation

---

## Next Steps

### Priority 1: Correctness & Reliability

1. **Wire per-model pricing** -- Read `input_price` and `output_price` from the model registry in `calculateCost()` instead of using hardcoded defaults
2. **Re-enable billing PreCheck middleware** -- Uncomment `billing.Engine.PreCheck` in the gateway route chain for consistent enforcement
3. **Wire image/video handlers to task engine** -- Replace 501 stubs with actual task submission via `taskEngine.Submit()`
4. **Wire embeddings handler** -- Connect to the existing `DashScopeAdapter.CreateEmbedding()` implementation
5. **Fix streaming latency metric** -- Record `RecordRequest` after stream completion, not at initiation

### Priority 2: Security & Production Readiness

6. **Add request body validation** -- Validate message roles, content types, and token limits before forwarding to upstream
7. **Implement refresh tokens** -- Complete the `/api/user/auth/refresh` endpoint with token rotation
8. **Add per-key rate limit overrides** -- Read `rate_limit_rpm` from the `api_keys` table in the rate limiter
9. **Add request logging middleware** -- Persist structured access logs with request_id correlation
10. **Enforce JWT secret strength** -- Validate minimum length and entropy in production config

### Priority 3: Features

11. **Admin panel backend** -- Implement model, provider, and user CRUD handlers
12. **Admin panel frontend** -- Build React components for admin operations
13. **Billing transaction pagination** -- Add offset/limit parameters and total count to the existing billing transactions endpoint
14. **User profile management** -- Implement GET/PUT for user profile
15. **File upload with OSS** -- Implement multipart upload and Alibaba Cloud OSS integration

### Priority 4: Scale & Advanced

16. **Semantic cache** -- Implement embedding-based cache with pgvector similarity search
17. **ClickHouse integration** -- Dual-write usage logs for high-volume analytics
18. **OAuth 2.0** -- Add support for Alibaba Cloud RAM and Google OAuth
19. **Multi-region routing** -- Implement geo-aware provider selection
20. **WebSocket audio streaming** -- Implement CosyVoice TTS and Paraformer ASR relay
