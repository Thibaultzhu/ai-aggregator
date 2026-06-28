# Architecture — AI Aggregator Platform

## 1. Architecture Principle

平台必须采用“统一底座 + 多商业模式扩展”的架构：

```text
Unified AI Gateway Core
├── Third-party Provider Aggregation
├── Self-hosted Model Inference
└── Workflow / Agent Result API
```

底层共享：

- Auth
- API Key
- Model Registry
- Routing
- Billing
- Usage
- Request Logs
- Admin
- Observability
- Policy

上层分化：

- API Resale
- Model Marketplace
- Self-hosted Inference
- Workflow / Agent
- Enterprise Governance
- Private / Sovereign AI

## 2. Ten-layer Architecture

```text
┌─────────────────────────────────────────────┐
│ 10. Vertical / Workflow / Agent Layer        │
│ 行业 Agent、Workflow API、任务型 API          │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 9. Marketplace / Ecosystem Layer             │
│ 模型市场、Prompt、Agent、Tool、Workflow 市场  │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 8. Evaluation / Optimization Layer           │
│ Benchmark、A/B Test、Smart Routing、模型推荐  │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 7. Guardrails / Compliance Layer             │
│ PII、审计、策略、内容安全、合规日志            │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 6. Observability / SLA Layer                 │
│ Request Logs、Trace、Health、Fallback、SLA    │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 5. Billing / FinOps Layer                    │
│ 余额、计费、成本归因、预算、Quota、毛利分析    │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 4. Control Plane Layer                       │
│ Users、Workspace、API Keys、RBAC、Admin       │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 3. Routing Layer                             │
│ Priority、Fallback、Health、Cost、Quality     │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 2. Provider Adapter Layer                    │
│ DashScope、OpenAI-compatible、BYOK、自托管    │
└─────────────────────────────────────────────┘
┌─────────────────────────────────────────────┐
│ 1. API Gateway Layer                         │
│ OpenAI-compatible API、Auth、Rate Limit       │
└─────────────────────────────────────────────┘
```

## 3. Request Flow

```text
Client
→ API Gateway
→ API Key Auth
→ Rate Limit / Quota / Balance Check
→ Request ID generation
→ Model Registry lookup
→ Routing Policy evaluation
→ Provider selection
→ Provider Adapter request transform
→ Provider call
→ Response normalization
→ Usage parsing
→ Billing
→ Request Log
→ Response to client
```

## 4. Failover Flow

```text
Request model = qwen-plus
→ model_providers lookup
→ provider priority sort
→ filter disabled providers
→ filter unhealthy providers
→ try primary provider
→ if provider timeout / 5xx / normalized retryable error
→ record fallback_logs
→ try next provider
→ return successful response or final normalized error
```

Current v0.2 implementation notes:

```text
Router returns RouteAll(model) sorted by model_providers.priority.
Gateway attempts each route in order.
When a provider call fails and a next route exists, Gateway writes fallback_logs.
request_logs records final_provider_id and fallback_count.
Mock fallback testing uses MOCK_FAIL_PROVIDER_IDS without forcing health failure.
```

## 5. Billing Flow

```text
Provider response
→ parse usage
→ calculate user price
→ calculate upstream cost
→ gross margin = charged_cost_usd - upstream_cost_usd
→ write usage log
→ write billing transaction
→ update balance
→ write request log
```

## 6. Module Boundaries

### Gateway Core

Responsible for:

- API compatibility
- Auth
- Request validation
- Request ID
- Error format
- Streaming transport

Not responsible for:

- Workflow planning
- Agent memory
- Tool orchestration
- Model benchmarking

### Provider Adapter

Responsible for:

- Provider-specific request conversion
- Provider-specific response conversion
- Error normalization
- Usage parsing
- Stream chunk normalization

### Router

Responsible for:

- Model to provider selection
- Health-based routing
- Fallback
- Future cost / latency / quality routing

### Billing

Responsible for:

- Token pricing
- Wallet deduction
- Transaction writing
- Margin calculation

### Observability

Responsible for:

- Request logs
- Provider health
- Fallback logs
- Latency
- Error rates

### Workflow / Agent

Must remain a separate upper layer. It calls Gateway Core as a model execution backend, not the other way around.

## 7. Recommended Technology Stack

| Layer | Recommended Option |
|---|---|
| Backend | Go or FastAPI, keep existing stack if already chosen |
| Main DB | PostgreSQL |
| Cache / Rate Limit | Redis |
| Logs / Analytics | ClickHouse later; PostgreSQL for MVP |
| Frontend | Next.js / React |
| UI | Tailwind / shadcn |
| Queue | Redis Stream / RabbitMQ / Kafka later |
| Observability | OpenTelemetry, Prometheus, Grafana later |
| Self-hosted Inference | vLLM / SGLang |
| Workflow | Temporal / LangGraph / custom DAG later |
