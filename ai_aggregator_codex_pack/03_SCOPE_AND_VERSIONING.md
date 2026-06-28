# Scope and Versioning

## Version Strategy

平台分为 v0.1 到 v1.0 八个阶段。每个阶段必须有明确范围、验收标准和不做事项。

## v0.1 — Runnable Thin Aggregator

### Positioning

最小可运行 AI Gateway / API Aggregator。

### Capabilities

- 用户注册 / 登录
- JWT auth
- API Key 创建与鉴权
- API Key hash 存储
- `/v1/models`
- `/v1/chat/completions`
- Mock Provider
- DashScope Provider
- Basic Model Routing
- Usage Logs
- Billing Transactions
- Balance Deduction
- Dashboard
- Billing Page
- Playground
- Docker Compose
- Smoke Test

### Acceptance

用户可以创建 API Key 并通过 OpenAI-compatible API 调用模型，系统可以记录用量并扣费。

## v0.2 — Reliable AI Gateway

### Positioning

从“能跑”升级为“稳定可运营”。

### Capabilities

- Request Logs
- Request ID 全链路追踪
- Provider Health Check
- Provider Failover
- Provider Status Page
- Fallback Logs
- Admin Model CRUD
- Admin Provider CRUD
- Admin Model-Provider Mapping CRUD
- Unified Error Format
- Fallback Smoke Test

### Explicitly Not Included

- Workflow / Agent Runtime
- Self-hosted GPU inference
- Full guardrails
- Full benchmark platform
- Full enterprise RBAC

## v0.3 — Enterprise Control Plane + FinOps

### Positioning

从开发者工具升级为企业 AI 控制面。

### Capabilities

- Organization
- Workspace
- Team Members
- RBAC
- API Key Scope
- Workspace Budget
- Quota
- Cost Attribution
- Audit Logs
- Provider Credentials / BYOK
- Usage Export
- Department / Project Cost

## v0.4 — Model Marketplace

### Positioning

从 Gateway 升级为模型市场。

### Capabilities

- Model Catalog
- Model Detail Page
- Model Tags
- Model Capabilities
- Model Comparison
- Pricing Page
- Provider Availability
- Model Ranking
- Benchmark Score
- Recommended Use Cases

## v0.5 — Evaluation / Benchmark / Optimization

### Positioning

从模型市场升级为模型选型和优化平台。

### Capabilities

- Benchmark Runner
- Private Evaluation Dataset
- A/B Testing
- Regression Test
- Quality Score
- Latency Score
- Cost Score
- LLM-as-Judge
- Prompt Evaluation
- Agent Evaluation
- Smart Routing Policy

## v0.6 — Guardrails / Compliance

### Positioning

企业安全、合规和 AI 风险控制。

### Capabilities

- PII Detection
- PII Masking
- Prompt Injection Detection
- Jailbreak Detection
- Content Moderation
- Policy Engine
- Output Validation
- Audit Trail
- Retention Policy
- Sensitive Data Routing

## v0.7 — Workflow / Agent API

### Positioning

从卖模型调用升级为卖业务结果。

### Capabilities

- Workflow Engine
- Workflow Steps
- Workflow Runs
- Tool Registry
- MCP Integration
- Prompt Template Registry
- Agent Runtime
- Agent Traces
- RAG Connector
- Webhook Callback
- Task-level Billing

## v1.0 — Private / Sovereign AI + Self-hosted Inference

### Positioning

支持政府、金融、KSA / UAE 本地化和私有化部署。

### Capabilities

- Private Deployment Package
- Helm / Kubernetes
- Self-hosted vLLM / SGLang Provider
- GPU Node Registry
- Model Deployment
- Dedicated Endpoint
- Local Logs
- Local Vector DB
- VPC / Private Network
- Sovereign AI Compliance Package
