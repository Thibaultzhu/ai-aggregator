# Business Model to Engineering Module Mapping

| Business Model | What It Sells | Required Modules | MVP Priority |
|---|---|---|---|
| API Resale / Thin Gateway | Unified API and token resale | Gateway, Provider Adapter, Routing, Billing, Wallet | v0.1 |
| Model Marketplace | Model discovery and model distribution | Model Catalog, Tags, Capabilities, Pricing, Comparison | v0.4 |
| Self-hosted Inference Provider | Platform-hosted open models | SelfHostedProvider, Inference Cluster, GPU Node, Model Deployment | v1.0 |
| AI Gateway / Enterprise Governance | Enterprise AI traffic control | Workspace, RBAC, API Key Scope, Audit, Policy | v0.3 |
| AI FinOps | AI cost governance | Budget, Quota, Cost Attribution, Margin, Cost Dashboard | v0.3 |
| Smart Routing | Better model selection | Cost Router, Latency Router, Quality Router, A/B Router | v0.5 |
| Reliability / SLA Layer | Production stability | Health Check, Fallback, Retry, Circuit Breaker, Status Page | v0.2 |
| Evaluation / Benchmark | Model quality measurement | Benchmark Runner, Eval Dataset, LLM Judge, Regression Test | v0.5 |
| Guardrails / Compliance | Risk and policy control | PII, Moderation, Policy Engine, Audit Trail, Retention | v0.6 |
| Workflow / Agent API | Business-result APIs | Workflow Engine, Agent Runtime, Tool Registry, Task Billing | v0.7 |
| BYOK / BYOM | Customer-owned credentials/models | Provider Credentials, Key Vault, BYOK Routing, BYOM Registry | v0.3+ |
| Private / Sovereign AI | Local deployment and data residency | Helm, Local Logs, VPC, Self-hosted Model, Sovereign Package | v1.0 |
| Data Loop / Fine-tuning | Feedback and improvement | Feedback Store, Dataset Builder, Fine-tune Jobs | v1.1+ |
| White-label / OEM / Channel | Partner resale | Partner Console, Branding, Revenue Share | v1.1+ |
| Developer / Agent / Tool Marketplace | Ecosystem monetization | Tool Marketplace, Agent Marketplace, Prompt Marketplace | v0.7+ |
| Procurement / Unified Billing | Unified enterprise procurement | Invoice, PO, Cost Center, Enterprise Contract | v0.3+ |
| AI Assurance / Risk Management | Risk report and assurance | Eval, Human Review, Incident Report, Version Control | v0.6+ |

## Commercial Packaging

### Developer Plan

- Free credits
- Multi-model API
- Playground
- API Keys
- Basic logs

### Team Plan

- Workspace
- Team members
- Usage dashboard
- Budget
- API key limits

### Enterprise Gateway

- SSO
- RBAC
- Audit logs
- BYOK
- Region policy
- SLA

### Dedicated Model Endpoint

- Dedicated GPU / model endpoint
- Reserved capacity
- Private endpoint
- SLA

### Workflow / Agent SaaS

- Workflow templates
- Agent templates
- RAG connectors
- Tool integrations
- Task-level billing
