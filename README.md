# AI Aggregator

**OpenAI-compatible API gateway for Alibaba Cloud Bailian (DashScope) models.**

AI Aggregator provides a unified, OpenAI-compatible API that routes requests to Alibaba Cloud Bailian (DashScope) models. It handles authentication, API key management, usage tracking, billing, and rate limiting, giving developers a drop-in replacement for OpenAI SDK clients while accessing Qwen, DeepSeek, GLM, and other models through Bailian.

---

## Features

- **OpenAI-compatible API** -- Drop-in replacement for existing OpenAI SDK clients (`/v1/chat/completions`, `/v1/models`, etc.)
- **Multi-model support** -- Text (Qwen, DeepSeek, GLM), image (Wan), video (Wan T2V/I2V), audio (Paraformer, CosyVoice), and embedding models
- **Streaming** -- Full SSE streaming support for chat completions with token-level relay
- **API key management** -- Create, list, and revoke API keys scoped per user; keys are SHA-256 hashed in storage
- **Usage tracking** -- Every request is logged with token counts, latency, provider, cost, and p95/p99 latency analytics
- **Billing** -- Prepaid balance system with automatic deduction, initial $10 credit for new users
- **Rate limiting** -- Sliding-window RPM and TPM limits via Redis sorted sets
- **Model routing** -- Priority-based provider selection with health checking and automatic failover
- **Prometheus metrics** -- Built-in `/metrics` endpoint with request counts, latency histograms, token counters, and cost gauges
- **React dashboard** -- Frontend scaffold with landing page, playground, model browser, pricing page, and user dashboard

---

## Architecture

```
Client (OpenAI SDK / curl)
       |
       v
  [Fiber HTTP Gateway]  :8080
       |
       +-- Auth Middleware (JWT or API Key)
       +-- Rate Limiter (Redis sliding window)
       +-- Model Router (priority + health-based selection)
       |       |
       |       +-- DashScope Adapter (CN / INTL)
       |               |
       |               v
       |        Alibaba Cloud Bailian API
       |
       +-- Billing (inline cost calculation + balance deduction)
       +-- Metrics Collector (Prometheus)
       +-- Stream Proxy (SSE relay)

  Storage:
    PostgreSQL 16 -- users, api_keys, models, usage_logs, billing
    Redis 7       -- rate limits, caching, concurrency counters
```

**Tech stack:**
- Backend: Go 1.22, Fiber v2, pgx/v5, go-redis/v9, Prometheus client
- Frontend: React 18, TypeScript, Vite 6, Tailwind CSS 3, Zustand, Recharts
- Database: PostgreSQL 16 (pgvector-enabled for future semantic cache), Redis 7
- Deployment: Docker Compose, multi-stage Dockerfiles

---

## Quick Start

### Local Codex/Desktop Environment

This workspace includes project-local tooling for machines where system packages are missing or restricted:

```bash
source scripts/dev-env.sh
scripts/dev-check.sh
```

Installed under `.local/`:

- Go `1.22.12`
- Helm `3.15.4`
- Go build/module caches inside `.local/go-cache` and `.local/go-mod-cache`

Project-local Docker config is under `.docker-config/` and `.docker-buildx/`. It avoids Docker Desktop Keychain credential errors during builds:

```bash
source scripts/dev-env.sh
docker compose build backend
docker compose up -d
```

`scripts/dev-check.sh` verifies Go, Node, Docker Compose, Helm rendering, app health, and the running frontend.

### Prerequisites

**Option A: Docker (recommended)**
- Docker 24+ and Docker Compose v2

**Option B: Local development**
- Go 1.22+
- Node.js 20+
- PostgreSQL 16
- Redis 7

### 1. Clone and configure

```bash
git clone <repo-url> ai-aggregator
cd ai-aggregator
cp .env.example .env
```

**Option 1: Mock provider mode (no API key needed)**

Add to `.env`:

```
MOCK_PROVIDER_MODE=true
```

This enables a built-in mock provider that returns canned responses for all chat completion requests. Ideal for local development, testing, and CI pipelines.

**Option 2: Real DashScope API key**

Add to `.env`:

```
DASHSCOPE_API_KEY=sk-your-dashscope-key-here
```

Or use region-specific keys (`DASHSCOPE_API_KEY_CN`, `DASHSCOPE_API_KEY_INTL`) if you need separate CN and INTL credentials.

Get a key from the [Alibaba Cloud DashScope console](https://dashscope.console.aliyun.com/).

### 2. Run with Docker Compose

```bash
docker compose up -d
```

This starts four services:

| Service   | Port  | Description                       |
|-----------|-------|-----------------------------------|
| postgres  | 5433  | PostgreSQL with schema + seed data |
| redis     | 6380  | Redis for rate limiting + cache   |
| backend   | 8081  | Go API gateway                    |
| frontend  | 5175  | React dev server (Vite)           |

The database is automatically initialized with schema (`001_init.sql`) and model seed data (`002_seed_models.sql`).

### 3. Run locally (without Docker)

**Start infrastructure:**

```bash
# PostgreSQL (must be running on :5432)
# Redis (must be running on :6379)
```

**Run database migrations:**

```bash
psql postgres://aag:aag_dev_pass@localhost:5432/aggregator \
  -f migrations/001_init.sql \
  -f migrations/002_seed_models.sql
```

**Start the backend:**

```bash
cd backend
go run ./cmd/server
# Server starts on :8081
```

**Start the frontend:**

```bash
cd frontend
npm install
npm run dev
# Dev server starts on :5173
```

### 4. Verify

```bash
curl http://localhost:8081/health
# {"status":"ok","version":"0.1.0-mvp"}
```

---

## API Endpoints

### Public

| Method | Path                    | Auth   | Description                |
|--------|-------------------------|--------|----------------------------|
| GET    | `/health`               | None   | Health check               |
| GET    | `/metrics`              | None   | Prometheus metrics         |

### User Authentication

| Method | Path                         | Auth   | Description                |
|--------|------------------------------|--------|----------------------------|
| POST   | `/api/user/auth/register`    | None   | Register new user          |
| POST   | `/api/user/auth/login`       | None   | Login (returns JWT)        |
| POST   | `/api/user/auth/refresh`     | JWT    | Refresh JWT                |

### User Portal (JWT required)

| Method | Path                              | Description                |
|--------|-----------------------------------|----------------------------|
| GET    | `/api/user/dashboard`             | Usage summary + balance    |
| GET    | `/api/user/usage`                 | Usage log (last 50)        |
| GET    | `/api/user/request-logs`          | Request logs with pagination, filters, and CSV export |
| GET    | `/api/user/request-logs/:request_id` | Request log detail      |
| GET    | `/api/user/keys`                  | List API keys              |
| POST   | `/api/user/keys`                  | Create API key             |
| DELETE | `/api/user/keys/:id`              | Revoke API key             |
| GET    | `/api/user/billing/balance`       | Get balance                |
| GET    | `/api/user/billing/transactions`  | Get billing transaction history |
| GET    | `/api/user/profile`               | Get current user profile   |
| PUT    | `/api/user/profile`               | Update current user profile |
| POST   | `/api/user/auth/refresh`          | Refresh JWT from valid JWT |

### OpenAI-compatible API (API Key or JWT)

| Method | Path                         | Description                        |
|--------|------------------------------|------------------------------------|
| GET    | `/v1/models`                 | List available models              |
| POST   | `/v1/chat/completions`       | Chat completion (stream & non-stream) |
| POST   | `/v1/images/generations`     | Submit async image generation task |
| GET    | `/v1/images/generations/:id` | Get image task status              |
| POST   | `/v1/video/generations`      | Submit async video generation task |
| GET    | `/v1/video/generations/:id`  | Get video task status              |
| POST   | `/v1/audio/transcriptions`   | Speech-to-text gateway via DashScope/mock adapter |
| POST   | `/v1/audio/speech`           | Text-to-speech gateway via DashScope/mock adapter |
| POST   | `/v1/embeddings`             | Text embeddings                    |
| POST   | `/v1/files`                  | Upload file with MIME sniffing + local dev storage |
| GET    | `/v1/files`                  | List current owner's files         |
| GET    | `/v1/files/:id`              | Get file metadata                  |
| GET    | `/v1/files/:id/content`      | Download file content              |
| DELETE | `/v1/files/:id`              | Delete file                        |

### Admin API (Admin JWT required)

| Method | Path                              | Description                   |
|--------|-----------------------------------|-------------------------------|
| GET    | `/api/admin/models`               | List all models               |
| POST   | `/api/admin/models`               | Create model                  |
| PUT    | `/api/admin/models/:id`           | Update model                  |
| DELETE | `/api/admin/models/:id`           | Delete model                  |
| POST   | `/api/admin/models/:id/providers` | Bind provider to model        |
| GET    | `/api/admin/providers`            | List providers                |
| POST   | `/api/admin/providers`            | Create provider               |
| PUT    | `/api/admin/providers/:id`        | Update provider               |
| GET    | `/api/admin/providers/:id/health/history` | Provider health-check history |
| GET    | `/api/admin/organizations`        | List organizations            |
| POST   | `/api/admin/organizations`        | Create organization           |
| GET    | `/api/admin/workspaces`           | List workspaces               |
| POST   | `/api/admin/workspaces`           | Create workspace              |
| GET    | `/api/admin/workspaces/:id/usage` | Workspace usage summary       |
| GET    | `/api/admin/workspaces/:id/usage/export` | Export workspace usage/cost CSV |
| GET    | `/api/admin/inference/clusters` | List private inference clusters |
| POST   | `/api/admin/inference/clusters` | Create private inference cluster |
| GET    | `/api/admin/inference/nodes` | List private inference nodes |
| POST   | `/api/admin/inference/nodes` | Register private inference node |
| GET    | `/api/admin/inference/deployments` | List self-hosted model deployments |
| POST   | `/api/admin/inference/deployments` | Register self-hosted model deployment |
| GET    | `/api/admin/users`                | List users                    |
| POST   | `/api/admin/users`                | Create user                   |
| GET    | `/api/admin/users/:id`            | Get user detail               |
| PUT    | `/api/admin/users/:id`            | Update user role/status       |
| POST   | `/api/admin/users/:id/balance`    | Top up user balance           |
| GET    | `/api/admin/users/:id/usage`      | User usage logs               |
| GET    | `/api/admin/keys`                 | List API key metadata         |
| PUT    | `/api/admin/keys/:id`             | Update API key limits/permissions |
| DELETE | `/api/admin/keys/:id`             | Revoke any API key            |
| GET    | `/api/admin/analytics/overview`   | Analytics overview            |
| GET    | `/api/admin/analytics/usage`      | Usage analytics               |
| GET    | `/api/admin/analytics/cost`       | Cost analytics                |
| GET    | `/api/admin/analytics/latency`    | Latency analytics             |
| GET    | `/api/admin/analytics/errors`     | Error analytics               |
| GET    | `/api/admin/benchmarks/tasks`     | List benchmark tasks          |
| POST   | `/api/admin/benchmarks/tasks`     | Create benchmark task         |
| GET    | `/api/admin/benchmarks/runs`      | List benchmark runs           |
| POST   | `/api/admin/benchmarks/tasks/:id/runs` | Run benchmark task       |
| GET    | `/api/admin/benchmarks/runs/:id`  | Get benchmark run results     |
| GET    | `/api/admin/settings`             | List system settings          |
| PUT    | `/api/admin/settings/:key`        | Update system setting         |
| POST   | `/api/admin/files/retention/run`  | Run file retention cleanup    |
| GET    | `/api/admin/guardrails/policies`  | List guardrail policies       |
| POST   | `/api/admin/guardrails/policies`  | Create guardrail policy       |
| GET    | `/api/admin/guardrails/results`   | List guardrail results        |
| GET    | `/api/admin/alerts/rules`         | List alert rules              |
| POST   | `/api/admin/alerts/rules`         | Create alert rule             |
| PUT    | `/api/admin/alerts/rules/:id`     | Update alert rule             |
| GET    | `/api/admin/alerts/history`       | Current alert summaries       |
| POST   | `/api/admin/alerts/history/:id/ack` | Acknowledge alert event     |
| POST   | `/api/admin/alerts/history/:id/resolve` | Resolve alert event      |
| GET    | `/api/admin/audit-logs`           | Audit log                     |

Admin endpoints are protected by admin JWT middleware. High-volume ClickHouse analytics remains a planned production enhancement.

Errors use a normalized envelope with `error.code`, `error.type`, `error.message`, and optional `error.request_id`. The full standard error code table is maintained in `ai_aggregator_codex_pack/09_API_DESIGN.md`.

---

## Testing

Run the automated smoke test:

```bash
# With mock provider (no API key needed):
MOCK_PROVIDER_MODE=true bash scripts/smoke-test.sh

# With real DashScope key:
DASHSCOPE_API_KEY=sk-your-key bash scripts/smoke-test.sh

# Without either (chat completion step is skipped):
bash scripts/smoke-test.sh
```

Or with a custom backend URL:

```bash
BASE_URL=http://localhost:8081 MOCK_PROVIDER_MODE=true bash scripts/smoke-test.sh
```

The smoke test runs 17 steps end-to-end:

1. Health check (GET /health) -- retries 10 times with 2s delay
2. Register test user (POST /api/user/auth/register) -- random email
3. Login (POST /api/user/auth/login) -- captures JWT
4. Get balance (GET /api/user/billing/balance) -- verifies ~$10 credit
5. Create API key (POST /api/user/keys) -- captures the key
6. List API keys (GET /api/user/keys) -- verifies key appears in list
7. List models (GET /v1/models) -- verifies model catalog
8. Chat completion non-stream (POST /v1/chat/completions) -- tests provider routing
9. Check balance decreased (GET /api/user/billing/balance) -- verifies billing deduction
10. Check usage logs (GET /api/user/usage) -- verifies recording
11. Check billing transactions (GET /api/user/billing/transactions) -- verifies credit_grant + usage_charge
12. Embeddings (POST /v1/embeddings) -- mock by default, real provider opt-in
13. Image async task (POST/GET /v1/images/generations) -- mock by default, real provider opt-in
14. Video async task (POST/GET /v1/video/generations) -- mock by default, real provider opt-in
15. Files API (POST/GET/DELETE /v1/files) -- verifies upload, metadata, content, delete
16. Revoke API key (DELETE /api/user/keys/:id) -- tests key lifecycle
17. Verify revoked key fails (POST /v1/chat/completions) -- expects 401

Each step prints PASS (green), FAIL (red), or SKIP (yellow) with HTTP status and response snippet.

**Requirements:** `curl`, `jq`, and a running backend instance.

---

## Project Structure

```
ai-aggregator/
|-- backend/
|   |-- cmd/server/main.go              # Entry point
|   |-- internal/
|   |   |-- config/config.go            # Environment config loader
|   |   |-- auth/middleware.go           # JWT + API key auth
|   |   |-- gateway/router.go           # HTTP route registration + handlers
|   |   |-- provider/
|   |   |   |-- adapter.go              # Provider adapter interface + types
|   |   |   |-- dashscope.go            # DashScope (Bailian) adapter
|   |   |   |-- mock.go                 # Mock adapter (MOCK_PROVIDER_MODE)
|   |   |-- router/model_router.go      # Priority-based model routing
|   |   |-- storage/store.go            # PostgreSQL + Redis data layer
|   |   |-- ratelimit/limiter.go        # Redis sliding-window rate limiter
|   |   |-- stream/proxy.go             # SSE stream relay
|   |   |-- billing/engine.go           # Cost calculation + balance mgmt
|   |   |-- task/engine.go              # Async task worker pool
|   |   |-- metrics/collector.go        # Prometheus metrics
|   |-- go.mod
|   |-- go.sum
|   |-- Dockerfile
|
|-- frontend/
|   |-- src/
|   |   |-- App.tsx                     # React Router setup
|   |   |-- pages/                      # Landing, Dashboard, Models, Pricing, etc.
|   |   |-- components/layout/          # Header, Footer, DashboardLayout
|   |   |-- lib/                        # Utilities, API client (api.ts)
|   |   |-- types/                      # TypeScript types
|   |-- package.json
|   |-- Dockerfile
|
|-- migrations/
|   |-- 001_init.sql                    # Database schema (all tables)
|   |-- 002_seed_models.sql             # Model + provider seed data
|
|-- docker-compose.yml                  # Full stack orchestration
|-- .env.example                        # Environment variable template
|-- scripts/
|   |-- smoke-test.sh                   # End-to-end smoke test
```

---

## Configuration

All configuration is via environment variables (see `.env.example`).

| Variable                  | Default                                          | Description                           |
|---------------------------|--------------------------------------------------|---------------------------------------|
| `APP_ENV`                 | `development`                                    | `development` or `production`         |
| `APP_HOST`                | `0.0.0.0`                                        | Bind address                          |
| `APP_PORT`                | `8080`                                           | HTTP listen port                      |
| `DATABASE_URL`            | *(required)*                                     | PostgreSQL connection string          |
| `REDIS_URL`               | `redis://localhost:6379/0`                       | Redis connection string               |
| `JWT_SECRET`              | `dev-secret-change-in-production-32chars!!`      | HMAC signing key for JWTs             |
| `API_KEY_PREFIX`          | `sk-aag-`                                        | Prefix for generated API keys         |
| `MOCK_PROVIDER_MODE`      | `false`                                          | Use mock provider (no real API calls) |
| `DASHSCOPE_API_KEY`       | *(optional)*                                     | Universal DashScope API key (works for CN & INTL) |
| `DASHSCOPE_API_KEY_CN`    | *(optional)*                                     | CN-specific override (takes priority over universal key) |
| `DASHSCOPE_API_KEY_INTL`  | *(optional)*                                     | INTL-specific override (takes priority over universal key) |
| `DASHSCOPE_ENDPOINT_CN`   | `https://dashscope.aliyuncs.com/compatible-mode/v1` | DashScope CN endpoint              |
| `DASHSCOPE_ENDPOINT_INTL` | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` | DashScope INTL endpoint        |
| `DEFAULT_RPM`             | `60`                                             | Requests per minute (per API key)     |
| `DEFAULT_TPM`             | `100000`                                         | Tokens per minute (per API key)       |
| `DEFAULT_DAILY_QUOTA_USD` | `100.00`                                         | Daily spending cap per user           |
| `TASK_WORKER_COUNT`       | `5`                                              | Async task polling workers            |
| `FILE_STORAGE_BACKEND`    | `local`                                          | FileStore backend selector; local is implemented |
| `FILE_STORAGE_DIR`        | `/tmp/ai-aggregator-files`                       | Local development file storage path   |

File upload governance is controlled at runtime by the `allowed_upload_mime_types` system setting. Uploaded files store `detected_mime`, `declared_mime`, `sha256`, and extension in file metadata. File lifecycle cleanup is controlled by `file_retention_days`; `0` disables retention cleanup.

---

## Roadmap

### MVP (current)

- [x] User registration and JWT authentication
- [x] API key creation, listing, and revocation
- [x] OpenAI-compatible `/v1/chat/completions` (streaming and non-streaming)
- [x] `/v1/models` listing
- [x] DashScope provider adapter (CN and INTL endpoints)
- [x] Priority-based model routing with health checks
- [x] Prepaid billing with balance deduction
- [x] Usage logging with token counts, latency, cost, and error rate
- [x] Sliding-window rate limiting (RPM/TPM) via Redis
- [x] Prometheus metrics endpoint
- [x] SSE stream relay with usage capture
- [x] React frontend scaffold (landing, dashboard, model browser, pricing)
- [x] Docker Compose deployment
- [x] Database schema with seed data for 30+ models
- [x] Per-model pricing from database (GetModelPricing with default fallback)
- [x] Provider fallback on failure (RouteAll with priority-based failover)
- [x] Mock provider mode for testing without API keys
- [x] Billing transaction history endpoint
- [x] JSON tags on all API response structs (snake_case consistency)

### Planned

- [x] Image generation (`/v1/images/generations`) async submit + task status
- [x] Video generation (`/v1/video/generations`) async submit + task status
- [x] Audio transcription and speech synthesis gateway handlers
- [x] Text embeddings (`/v1/embeddings`)
- [x] File upload and management (`/v1/files`) with FileStore adapter foundation, local development storage, MIME allowlist, checksum metadata, and admin retention cleanup
- [x] Admin panel foundation (model/provider/user CRUD, analytics, settings, alerts, audit)
- [ ] Semantic cache using pgvector cosine similarity
- [ ] OAuth 2.0 / SSO integration
- [ ] Multi-region deployment with Global Accelerator
- [ ] Webhook callbacks for async tasks
- [x] Token refresh endpoint
- [x] User profile management
- [ ] ClickHouse integration for high-volume analytics

---

## Troubleshooting

### Missing go.sum or Go module errors

If you see `missing go.sum entry` or module download errors:

```bash
cd backend
go mod tidy
go mod download
```

Ensure you're using Go 1.22+ (`go version`).

### Database migration failures

If migrations fail with "relation already exists":

```bash
# Drop and recreate the database (destructive!)
psql postgres://aag:aag_dev_pass@localhost:5432/aggregator -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Re-run migrations
psql postgres://aag:aag_dev_pass@localhost:5432/aggregator \
  -f migrations/001_init.sql \
  -f migrations/002_seed_models.sql
```

If migrations fail with "connection refused", verify PostgreSQL is running:

```bash
pg_isready -h localhost -p 5432
```

### Smoke test: chat completion returns 502

This is expected when no DashScope API key is configured and `MOCK_PROVIDER_MODE` is not enabled. To fix:

```bash
# Option 1: Use mock mode
MOCK_PROVIDER_MODE=true bash scripts/smoke-test.sh

# Option 2: Set a real key
DASHSCOPE_API_KEY=sk-your-key bash scripts/smoke-test.sh
```

### Redis connection refused

If the backend fails to start with Redis connection errors:

```bash
# Check Redis is running
redis-cli ping
# Should return: PONG

# If using Docker Compose, restart Redis
docker compose restart redis
```

### Port already in use

If port 8080 is already in use:

```bash
# Find what's using the port
lsof -i :8080

# Or run on a different port
APP_PORT=8081 go run ./cmd/server
```

### Docker Compose backend exits immediately

Check the logs:

```bash
docker compose logs backend
```

Common causes:
- PostgreSQL or Redis not ready yet (check health checks in `docker-compose.yml`)
- Missing or invalid environment variables in `.env`
- Database migration errors

### Frontend shows blank page or API errors

The frontend proxies API calls to `http://localhost:8080`. Ensure the backend is running:

```bash
curl http://localhost:8080/health
# Should return: {"status":"ok","version":"0.1.0-mvp"}
```

Check Vite proxy configuration in `frontend/vite.config.ts` if the backend runs on a non-default port.

---

## License

Proprietary. All rights reserved.
