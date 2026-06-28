# Feature Implementation Tracker

文档名称：`FEATURE_IMPLEMENTATION_TRACKER.md`  
项目名称：AI Model Aggregator / AI Gateway / AI Aggregator Platform  
用途：跟踪每一个功能 / 页面 / API / 数据库表 / 技术实现方式 / 文件路径 / 验收方式。  
当前状态：配合 `PROJECT_STAGE_TRACKER.md` 使用。  
维护频率：每次 Codex / QoderWork / Manual 开发完成后必须更新。

---

## 1. 文档目标

本文件用于记录每个功能的：

```text
1. 功能目标
2. 所属阶段
3. 当前状态
4. 实现路径
5. 使用技术
6. 核心文件路径
7. 数据库表 / 字段
8. 后端 API
9. 前端页面
10. 关键业务流程
11. 验收方式
12. 后续优化点
```

此文档的目的不是只记录“做了什么”，而是记录：

```text
这个功能是怎么实现的
通过什么技术实现的
代码在哪里
数据存在哪里
接口怎么调用
前端在哪里展示
如何测试
后续怎么优化
```

---

## 2. 功能实现追踪总表

| 功能模块 | 所属阶段 | 当前状态 | 后端路径 | 前端路径 | 数据库表 | 主要技术 |
|---|---|---:|---|---|---|---|
| 用户注册 / 登录 | v0.1 | 已完成 | `backend/internal/gateway` / `auth` / `storage` | `frontend/src/pages/Login.tsx` | `users` | JWT, bcrypt, PostgreSQL |
| API Key 管理 | v0.1/v0.3 | 已完成 | `auth`, `storage`, `gateway` | `ApiKeys.tsx`, `lib/api.ts` | `api_keys` | crypto random, hash, prefix, optional workspace binding, PostgreSQL |
| `/v1/models` | v0.1 | 已完成 | `gateway`, `storage` | `Models.tsx` | `models`, `model_providers`, `providers` | OpenAI-compatible response |
| `/v1/chat/completions` | v0.1 | 已完成 | `gateway`, `router`, `provider` | `Playground.tsx` | `usage_logs`, `billing_transactions` | OpenAI-compatible API, Provider Adapter |
| `/v1/embeddings` | v0.3 | Verified | `gateway`, `router`, `provider` | `Playground.tsx` | `usage_logs`, `request_logs`, `billing_transactions` | OpenAI-compatible embeddings, provider fallback, Playground embedding vector preview |
| Image / Video Async API | v0.3 | Verified | `gateway`, `task`, `provider`, `storage` | `Playground.tsx` | `async_tasks` | async submit, task polling worker, DashScope native task API, Playground async task submit/status UI; real Bailian image/video model mapping待校准 |
| Mock Provider | v0.1 | 已完成 | `provider/mock` 或 `provider` | N/A | N/A | Mock response, local test |
| DashScope Provider | v0.1 | 已完成 | `provider/dashscope` | N/A | `providers`, `model_providers` | DashScope API, upstream model mapping |
| Billing / Balance | v0.1 | Verified | `billing`, `storage`, `gateway` | `Billing.tsx`, `Dashboard.tsx` | `billing_transactions`, `users.balance_usd` | prepaid balance, DB pricing, transaction CSV export, Add Credits admin-grant guidance |
| Usage Logs | v0.1/v0.2 | Verified | `storage`, `gateway` | `Dashboard.tsx` | `usage_logs` | token accounting, cost tracking, avg/p95/p99 latency, error rate |
| Rate Limit | v0.1 | 已完成 | `ratelimit` | N/A | Redis | fixed window RPM |
| Request Logs | v0.2 | Verified | `storage`, `gateway` | `RequestLogs.tsx` | `request_logs` | request_id trace, preview logging, pagination, filters, CSV export |
| Provider Health Check | v0.2 | Verified | `router`, `provider`, `storage`, `gateway` | `Admin.tsx` Provider Status | `provider_health_checks`, `model_providers`, `request_logs` | startup/background health loop, manual health API, health history, 24h request/error/fallback aggregation |
| Provider Failover | v0.2 | Verified | `router`, `gateway`, `provider` | Request Logs / Provider Status | `fallback_logs`, `request_logs` | fallback chain, mock failure mode |
| Fallback Smoke Test | v0.2 | Verified | `scripts/smoke-test.sh` | N/A | `request_logs`, `fallback_logs` | `RUN_FALLBACK_SMOKE=true` executed on 2026-06-26: primary `bailian_cn` failure falls back to `bailian_intl`, request_logs fallback_count=1, fallback_logs transition recorded |
| Enterprise Foundation Schema | v0.3 | Schema Foundation | `migrations/004_v03_enterprise_foundation.sql` | N/A | `organizations`, `workspaces`, `workspace_members`, `roles`, `workspace_budgets`, `workspace_quotas` | org/workspace/RBAC/FinOps base schema |
| Organization / Workspace Admin | v0.3 | Verified | `gateway`, `storage` | `Admin.tsx` Workspaces | `organizations`, `workspaces`, `workspace_members`, `request_logs.workspace_id` | Admin API, user dropdown member management with role/status upsert, usage summary and Cost Attribution panel |
| Workspace Cost Attribution | v0.3 | Verified | `auth`, `gateway`, `storage` | `ApiKeys.tsx`, `Admin.tsx` Workspaces | `api_keys.workspace_id`, `request_logs.workspace_id`, `usage_logs.workspace_id`, `billing_transactions.workspace_id` | API Key workspace binding, request/usage/billing attribution, workspace usage API returns Top model/provider/user cost attribution |
| Project Cost Centers | v0.3/v1.0 | Verified | `auth`, `gateway`, `storage`, `scripts/regression` | `ApiKeys.tsx`, `Admin.tsx` Workspaces | `projects`, `api_keys.project_id`, `request_logs.project_id`, `usage_logs.project_id`, `billing_transactions.project_id` | Workspace project create/list, API Key project binding, request/usage/billing project attribution, workspace usage Top Projects, CSV `project_id` export/filter; `scripts/regression/project-cost-centers.sh` 2026-06-26 result 16 pass / 0 fail |
| Invoice / PO / Postpaid Billing | v0.3/v1.0 | Verified | `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Workspaces | `organizations.payment_terms_days`, `organizations.default_po_number`, `invoices`, `billing_transactions`, `audit_logs` | Admin can create postpaid organizations with payment terms and default PO, generate workspace invoice drafts from billing transactions, auto-calculate due date/subtotal/total, list/export invoices, update invoice status draft/issued/paid/void, and write invoice create/status audit events; `scripts/regression/invoices-po-postpaid.sh` 2026-06-27 result 12 pass / 0 fail; `scripts/regression/invoice-status-export.sh` 2026-06-27 result 15 pass / 0 fail |
| Workspace Usage Export | v0.3 | Verified | `gateway`, `storage` | `Admin.tsx` Workspaces | `request_logs.workspace_id` | admin workspace usage/cost CSV export with request/cost/token/latency fields |
| RBAC Workspace Enforcement | v0.3/v1.0 | Verified | `gateway`, `storage` | `ApiKeys.tsx`, `Files.tsx`, `Workflows.tsx` | `workspace_members`, `roles`, `api_keys.workspace_id`, `uploaded_files.workspace_id`, `workflows.workspace_id`, `workspace_budgets`, `workspace_quotas` | workspace key creation requires api_keys:create; workspace file upload requires files:write; workspace workflow create/run requires workflows:write; workspace budget/quota user writes require workspace_budgets:write/workspace_quotas:write; viewer denied, member/owner paths verified by permission |
| Workspace Budget / Quota Enforcement | v0.3 | Verified | `gateway`, `storage` | `Admin.tsx` Workspaces / Admin API / User API | `workspace_budgets`, `workspace_quotas`, `request_logs`, `usage_logs` | runtime pre-check, monthly budget, request/token/spend quota, delegated owner/admin budget/quota management, Admin Workspaces budget/quota create/list UI |
| Admin Credit Grant / Audit Logs | v0.3/v1.0 | Verified | `gateway`, `storage` | `Admin.tsx` Audit Log / Admin API | `billing_transactions`, `audit_logs` | admin top-up, audit list, governance traceability, audit CSV export |
| Admin Model 管理 | v0.2/v0.4/v0.5 | Verified | `gateway`, `storage`, `router` | `Admin.tsx` Models | `models`, `model_provider_bindings`, `model_pricing_history` | CRUD, pricing update, pricing history / price-change audit baseline, provider binding edit/add/delete, upstream_model calibration UI, admin auth |
| Admin Provider 管理 | v0.2 | Verified | `gateway`, `storage`, `router` | Provider Status / Models binding view | `providers`, `model_providers` | CRUD, priority mapping, route refresh |
| Provider Credentials / Scoped BYOK | v0.3/v1.0 | Verified | `gateway`, `storage`, `router`, `scripts/regression` | `Admin.tsx` Models Provider Credentials | `provider_keys`, `audit_logs` | Admin provider API key create/list/revoke, masked response, sealed DB persistence, platform/user/workspace scoped BYOK, workspace > user > platform runtime credential priority, audit events; `scripts/regression/scoped-byok.sh` 2026-06-27 result 19 pass / 0 fail |
| User BYOK Self-Service | v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Settings.tsx` | `provider_keys`, `audit_logs` | Users can list enabled providers, create/list/revoke their own user-scoped provider keys from Settings, secrets are masked and sealed, and user create/revoke audit events are recorded; `scripts/regression/user-byok-self-service.sh` 2026-06-27 result 13 pass / 0 fail |
| Secret Management Metadata | v1.0 | Verified | `storage`, `router`, `gateway`, `scripts/regression` | `Admin.tsx` Models Provider Credentials | `provider_keys`, `tool_credentials` | `seal_version`, `last_used_at`, `last_used_scope`, `revoked_at`, `rotated_at` metadata for provider/tool credentials; provider key usage updates on routing; safe Admin list exposes metadata without plaintext; `scripts/regression/secret-management-metadata.sh` 2026-06-27 result 19 pass / 0 fail |
| Provider Credential Validation | v1.0 | Verified | `gateway`, `storage`, `provider`, `scripts/regression` | `Admin.tsx` Models Provider Credentials | `provider_keys`, `provider_health_checks`, `audit_logs` | Admin can validate a specific provider key via `POST /api/admin/providers/:id/keys/:key_id/validate`; validation builds a temporary adapter with the selected secret, returns healthy/unhealthy with latency/error, records provider health history and `provider_key.validate` audit event; `scripts/regression/provider-credential-validation.sh` 2026-06-27 result 9 pass / 0 fail |
| Request Log Credential Scope | v1.0 | Verified | `router`, `storage`, `gateway`, `scripts/regression` | `RequestLogs.tsx` | `request_logs`, `provider_keys` | Request logs record non-secret provider credential attribution via `credential_scope` and `credential_key_id`; API/CSV/UI expose metadata for customer BYOK traceability; `scripts/regression/request-log-credential-scope.sh` 2026-06-27 result 17 pass / 0 fail |
| Provider Onboarding Templates | v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `lib/api.ts` | `providers`, `models`, `model_providers`, `audit_logs` | Admin template list/install API for OpenAI, Grok, and Anthropic; installs provider rows, model catalog entries, and model-provider bindings; `scripts/regression/provider-onboarding-templates.sh` 2026-06-27 result 19 pass / 0 fail |
| Anthropic Native Adapter | v1.0 | Verified | `provider`, `router`, `gateway`, `scripts/regression` | `Admin.tsx` provider adapter type | `providers`, `model_providers`, `request_logs`, `usage_logs` | Native Anthropic Messages API adapter maps OpenAI-compatible chat requests to `/v1/messages`, sends `x-api-key` and `anthropic-version`, maps response/usage back to OpenAI-compatible shape, and supports `adapter_type=anthropic`; `scripts/regression/anthropic-native-adapter.sh` 2026-06-27 result 13 pass / 0 fail |
| OpenAI/Grok Provider Validation | v1.0 | Verified | `gateway`, `provider`, `scripts/regression` | `Admin.tsx` Provider Credentials | `providers`, `provider_keys`, `provider_health_checks`, `audit_logs` | OpenAI-compatible validation path verified for OpenAI and Grok provider base URLs using local fake upstream `/models`, Bearer authorization header, health persistence, and audit persistence; `scripts/regression/openai-grok-provider-validation.sh` 2026-06-27 result 12 pass / 0 fail |
| Model Marketplace | v0.4 | Verified | `gateway`, `storage` | `Models` | `models`, `model_providers` | public catalog API, search/filter, detail, compare, provider availability, deterministic marketplace score |
| Landing Featured Models | v1.0 | Verified | `gateway`, `storage` | `Landing.tsx`, `Playground.tsx` | `models`, `model_provider_bindings` | homepage featured models and model-count copy use live `/api/marketplace/models`; DemoCard opens query-prefilled Playground; sorted by marketplace_score |
| Docs Page API Reference | v1.0 | Verified | `gateway`, `storage` | `Docs.tsx` | `models`, `model_provider_bindings` | Docs page shows live catalog count, current local base URL `http://localhost:8081/v1`, and complete files endpoint list |
| Pricing Page Catalog | v1.0 | Verified | `gateway`, `storage` | `Pricing.tsx` | `models`, `model_provider_bindings` | Pricing page uses live `/api/marketplace/models` catalog instead of static mock model data; grouped by modality with price/unit/provider availability |
| Benchmark Foundation | v0.5 | Verified | `gateway`, `storage` | `Admin.tsx` Benchmarks / Marketplace score | `benchmark_tasks`, `benchmark_runs`, `benchmark_results` | task create/list UI, run benchmark UI, result detail UI, quality/cost/latency score, latest benchmark_score in Marketplace |
| Smart Routing Baseline | v0.5/v1.0 | Verified | `router`, `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Routing | `routing_policies`, `request_logs`, `model_providers` | Admin routing policy create/list, priority/cost/latency/balanced route ordering, provider stats from recent request_logs, router fallback chain reordered by active policy; `scripts/regression/smart-routing.sh` 2026-06-26 result 16 pass / 0 fail |
| Guardrails Foundation | v0.6 | Verified | `gateway`, `storage` | `Admin.tsx` Guardrails / Gateway pre-check | `guardrail_policies`, `guardrail_results`, `pii_detections`, `policy_violations` | default global policy, policy create/list UI, result list UI, PII masking, prompt injection block, audit write |
| Compliance Audit Export / Retention | v0.6/v1.0 | Verified | `gateway`, `storage` | `Admin.tsx` Settings / Audit Log | `audit_logs`, `system_settings` | audit log list filters, CSV export, audit_log_retention_days, dry-run cleanup, retention audit write |
| Workflow / Agent Foundation | v0.7 | Verified | `gateway`, `storage`, `router` | `Workflows.tsx` | `workflows`, `workflow_steps`, `workflow_runs`, `workflow_run_steps`, `tools`, `tool_credentials`, `prompt_templates`, `agent_sessions`, `agent_traces`, `webhook_deliveries` | workflow CRUD, sync run, prompt step, prompt templates baseline, echo tool, user workflow create/run UI, run history, traces, cost record, signed webhook delivery worker with retry status, tool credentials baseline, agent sessions baseline |
| Private / Self-hosted Inference Foundation | v1.0 | Verified | `gateway`, `storage`, `router` | `Admin.tsx` Inference / unified `/v1` | `inference_clusters`, `inference_nodes`, `model_deployments`, `capacity_metrics`, `providers`, `models`, `model_providers` | register cluster/node/deployment, Admin Inference UI, auto provider/model binding, self-hosted route via unified chat API |
| Private Deployment Package | v1.0 | Implemented | `deploy/helm` | N/A | N/A | Helm chart for backend/frontend private deployment with external Postgres/Redis secrets |
| File API Governance / Retention | v1.0 | Verified | `gateway`, `storage` | `Files.tsx` | `uploaded_files`, `system_settings`, `audit_logs` | MIME allowlist, checksum metadata, local storage, user file upload/list/detail/download/delete UI, admin retention cleanup, local EICAR malware-scan baseline |
| Completed Feature Test Cases | v1.0 | Verified | `scripts`, `backend`, `frontend` | N/A | N/A | `15_COMPLETED_FEATURE_TEST_CASES.md` documents completed features, test objectives, methods, expected results, current results, pass criteria, and evidence sources |
| Test Execution Results | v1.0 | Verified | `scripts`, `backend`, `frontend` | N/A | N/A | `16_TEST_EXECUTION_RESULTS_2026-06-25.md` records actual execution results: Go tests, frontend build, dev-check, mock smoke, real provider smoke, enterprise/admin/private inference API regression |
| Targeted Bailian Qwen Smoke | v1.0 | Verified | `scripts/smoke-test.sh` | N/A | `usage_logs`, `billing_transactions` | `SMOKE_MODEL_ID=qwen3.7-max` verifies real Bailian Qwen chat, balance deduction, usage log, billing transaction, and real `text-embedding-v3` embedding |
| UI Browser Smoke / Responsive Dashboard | v1.0 | Verified | `frontend/src/components/layout/DashboardLayout.tsx` | `DashboardLayout`, `Dashboard`, `RequestLogs`, `Files`, `Workflows`, `Playground`, `Admin` | N/A | in-app Browser smoke verifies register/login, logout, real profile display, dashboard rendering, workflow create interaction, request-log filter interaction, playground render, Admin Overview render, and 390px mobile Files layout; mobile sidebar clipping fixed |
| Admin Workspaces Browser E2E | v0.3/v1.0 | Verified | `gateway`, `storage` | `Admin.tsx` Workspaces | `organizations`, `workspaces`, `workspace_members`, `workspace_budgets`, `workspace_quotas`, `request_logs.workspace_id` | 2026-06-26 in-app Browser E2E verifies create organization/workspace, budget, quota, member role/status, page console health, and workspace usage CSV API export; stable `data-testid` hooks added |
| Admin Foundation Regression Script | v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | N/A | `guardrail_policies`, `guardrail_results`, `benchmark_tasks`, `benchmark_runs`, `benchmark_results`, `inference_clusters`, `inference_nodes`, `model_deployments`, `audit_logs`, `alert_rules`, `alert_events` | `scripts/regression/admin-foundation.sh` executable regression covers Guardrails, Benchmark, Private Inference, Audit export/retention, and Alert ack/resolve; 2026-06-26 result 24 pass / 0 fail |
| User Marketplace + Workflow Regression Script | v0.4/v0.7/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Models.tsx`, `Workflows.tsx` | `models`, `model_providers`, `benchmark_results`, `workflows`, `workflow_steps`, `workflow_runs`, `workflow_run_steps`, `agent_traces` | `scripts/regression/user-marketplace-workflow.sh` executable regression covers JWT missing-token rejection, Marketplace list/filter/detail/compare, and Workflow create/run/list/detail/traces; 2026-06-26 result 18 pass / 0 fail |
| Guardrails Policy Regression Script | v0.6/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Admin.tsx` Guardrails | `guardrail_policies`, `guardrail_results`, `pii_detections`, `policy_violations`, `audit_logs` | `scripts/regression/guardrails-policy.sh` executable regression covers PII detection/block, prompt injection block, guardrail result findings, audit logs, and DB persistence; temporary policy is disabled on exit; 2026-06-26 result 14 pass / 0 fail |
| Auth Conflict Regression Script | v0.1/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Login.tsx` | `users` | `scripts/regression/auth-conflict.sh` executable regression covers initial registration, duplicate registration 409 conflict, stable error envelope, and no database detail leakage; 2026-06-26 result 4 pass / 0 fail |
| Audio Mock Regression Script | v0.7/v1.0 | Verified | `gateway`, `router`, `storage`, `scripts/regression` | `Playground.tsx` | `providers`, `models`, `model_providers`, `request_logs`, `usage_logs` | `scripts/regression/audio-mock.sh` executable regression covers per-provider mock adapter routing, STT, TTS, request_logs, and usage_logs; 2026-06-26 result 7 pass / 0 fail |
| File Malware Scan Regression Script | v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Files.tsx` | `uploaded_files`, `audit_logs` | `scripts/regression/file-scan.sh` executable regression covers clean upload scan metadata, EICAR test signature block, blocked file not persisted, and `file.upload_blocked` audit event; 2026-06-26 result 6 pass / 0 fail |
| Provider Health Stats Regression Script | v0.2/v1.0 | Verified | `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Provider Status | `providers`, `provider_health_checks`, `request_logs` | `scripts/regression/provider-health-stats.sh` executable regression covers 24h request_count, error_count, error_rate, fallback_count, and average request latency aggregation; 2026-06-26 result 6 pass / 0 fail |
| Workflow Webhook Regression Script | v0.7/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Workflows.tsx` Run Detail | `workflows`, `workflow_runs`, `webhook_deliveries` | `scripts/regression/workflow-webhook.sh` executable regression covers callback URL validation, recorded webhook delivery in run response/detail, and DB persistence; 2026-06-26 result 8 pass / 0 fail |
| Workflow Webhook Worker Regression Script | v0.7/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Workflows.tsx` Run Detail | `webhook_deliveries` | `scripts/regression/workflow-webhook-worker.sh` executable regression covers real HTTP callback delivery to a local receiver, HMAC signature, delivery headers, DB `delivered` status, response status, attempt count, and run detail metadata; 2026-06-27 result 8 pass / 0 fail |
| Tool Credentials Regression Script | v0.7/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Workflows.tsx` Tool Credentials | `tools`, `tool_credentials` | `scripts/regression/tool-credentials.sh` executable regression covers create/list masked response, DB encrypted persistence, revoke, and invalid input; 2026-06-26 result 9 pass / 0 fail |
| Agent Sessions Regression Script | v0.7/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Workflows.tsx` Agent Sessions | `agent_sessions`, `workflow_runs` | `scripts/regression/agent-sessions.sh` executable regression covers session create/list/get/close, workflow run binding, last_run tracking, and DB persistence; 2026-06-26 result 11 pass / 0 fail |
| Prompt Templates Regression Script | v0.7/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Workflows.tsx` Prompt Templates | `prompt_templates`, `workflows`, `workflow_runs` | `scripts/regression/prompt-templates.sh` executable regression covers template create/list/get/archive, prompt workflow creation/run, and DB persistence; 2026-06-26 result 11 pass / 0 fail |
| Pricing History Regression Script | v0.4/v0.5/v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `Admin.tsx` Models | `models`, `model_pricing_history` | `scripts/regression/pricing-history.sh` executable regression covers model pricing create/update history, Admin detail/history API, and DB persistence; 2026-06-26 result 11 pass / 0 fail |
| Workspace Cost Attribution Regression Script | v0.3/v1.0 | Verified | `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Workspaces | `request_logs.workspace_id` | `scripts/regression/workspace-cost-attribution.sh` executable regression covers workspace total usage, Top model/provider/user cost attribution, and CSV compatibility; 2026-06-26 result 12 pass / 0 fail |
| Provider Credentials Regression Script | v0.3/v1.0 | Verified | `storage`, `gateway`, `router`, `scripts/regression` | `Admin.tsx` Models | `provider_keys`, `audit_logs` | `scripts/regression/provider-credentials.sh` executable regression covers provider create, masked credential create/list, sealed DB persistence, revoke, and audit events; 2026-06-26 result 11 pass / 0 fail |
| Scoped BYOK Regression Script | v0.3/v1.0 | Verified | `storage`, `gateway`, `router`, `scripts/regression` | `Admin.tsx` Models | `provider_keys`, `workspace_members`, `request_logs`, `audit_logs` | `scripts/regression/scoped-byok.sh` executable regression covers migration, platform/user/workspace provider keys, masked list, no plaintext leakage, workspace credential priority, workspace API key route, request attribution, and audit; 2026-06-27 result 19 pass / 0 fail |
| User BYOK Self-Service Regression Script | v1.0 | Verified | `storage`, `gateway`, `scripts/regression` | `Settings.tsx` | `provider_keys`, `audit_logs` | `scripts/regression/user-byok-self-service.sh` executable regression covers normal-user provider list, user-scoped provider key create/list/revoke, no plaintext response/DB leakage, ownership, inactive revoke state, and audit events; 2026-06-27 result 13 pass / 0 fail |
| Secret Management Metadata Regression Script | v1.0 | Verified | `storage`, `router`, `gateway`, `scripts/regression` | `Admin.tsx` Models | `provider_keys`, `tool_credentials` | `scripts/regression/secret-management-metadata.sh` executable regression covers seal metadata, last-used updates during route resolution, safe list metadata, revoked_at, and tool credential seal metadata; 2026-06-27 result 19 pass / 0 fail |
| Provider Credential Validation Regression Script | v1.0 | Verified | `storage`, `gateway`, `provider`, `scripts/regression` | `Admin.tsx` Models | `provider_keys`, `provider_health_checks`, `audit_logs` | `scripts/regression/provider-credential-validation.sh` executable regression covers provider-specific key validation for mock and Anthropic adapters, local fake Anthropic request shape, healthy result, provider health check persistence, and audit event persistence; 2026-06-27 result 9 pass / 0 fail |
| Request Log Credential Scope Regression Script | v1.0 | Verified | `storage`, `router`, `gateway`, `scripts/regression` | `RequestLogs.tsx` | `request_logs`, `provider_keys` | `scripts/regression/request-log-credential-scope.sh` executable regression covers v28 migration, workspace scoped provider key route selection, request log credential scope/key id persistence, and user request log API exposure; 2026-06-27 result 17 pass / 0 fail |
| OpenAI/Grok Provider Validation Regression Script | v1.0 | Verified | `gateway`, `provider`, `scripts/regression` | `Admin.tsx` | `providers`, `provider_keys`, `provider_health_checks`, `audit_logs` | `scripts/regression/openai-grok-provider-validation.sh` executable regression covers OpenAI-compatible provider validation for OpenAI and Grok fake upstreams, `/models` path, Bearer key header, health history, and audit events; 2026-06-27 result 12 pass / 0 fail |
| Provider Onboarding Templates Regression Script | v1.0 | Verified | `gateway`, `storage`, `scripts/regression` | `lib/api.ts` | `providers`, `models`, `model_providers`, `audit_logs` | `scripts/regression/provider-onboarding-templates.sh` executable regression covers template list, OpenAI/Grok/Anthropic install, provider/model/binding persistence, catalog availability, and audit; 2026-06-27 result 19 pass / 0 fail |
| Anthropic Native Adapter Regression Script | v1.0 | Verified | `provider`, `router`, `gateway`, `scripts/regression` | `Admin.tsx` | `providers`, `model_providers`, `request_logs`, `usage_logs` | `scripts/regression/anthropic-native-adapter.sh` executable regression uses a local fake Anthropic server to verify native `/messages` request shape, headers, system mapping, response mapping, request logging, and usage token logging; 2026-06-27 result 13 pass / 0 fail |
| Project Cost Centers Regression Script | v0.3/v1.0 | Verified | `storage`, `auth`, `gateway`, `scripts/regression` | `Admin.tsx` Workspaces, `ApiKeys.tsx` | `projects`, `api_keys.project_id`, `request_logs.project_id` | `scripts/regression/project-cost-centers.sh` executable regression covers project create/list, workspace owner membership, project-bound API key creation/storage/authentication, Top Projects attribution, and CSV `project_id`; 2026-06-26 result 16 pass / 0 fail |
| Smart Routing Regression Script | v0.5/v1.0 | Verified | `router`, `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Routing | `routing_policies`, `request_logs`, `model_providers` | `scripts/regression/smart-routing.sh` executable regression covers policy migration, Admin policy create/list, mock provider setup, latency stats seeding, chat completion, and faster-provider selection despite lower static priority; 2026-06-26 result 16 pass / 0 fail |
| Invoice / PO / Postpaid Regression Script | v0.3/v1.0 | Verified | `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Workspaces | `invoices`, `organizations`, `billing_transactions`, `audit_logs` | `scripts/regression/invoices-po-postpaid.sh` executable regression covers migration, postpaid org terms, workspace invoice draft, PO defaulting, subtotal calculation, due date, invoice list, DB persistence, and audit event; 2026-06-27 result 12 pass / 0 fail |
| Invoice Status / Export Regression Script | v0.3/v1.0 | Verified | `storage`, `gateway`, `scripts/regression` | `Admin.tsx` Workspaces | `invoices`, `audit_logs` | `scripts/regression/invoice-status-export.sh` executable regression covers status transitions draft -> issued -> paid, invalid status rejection, invoice CSV export, DB persistence, and audit events; 2026-06-27 result 15 pass / 0 fail |

---

## 3. 功能实现详情模板

每新增一个功能，必须使用以下模板记录。

```markdown
## 功能名称：<Feature Name>

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.x |
| 当前状态 | Not Started / Planned / In Progress / Partial / Implemented / Verified |
| 负责人 | Codex / QoderWork / Manual |
| 最后更新时间 | YYYY-MM-DD |
| 是否影响 v0.1 主链路 | Yes / No |
| 是否需要 migration | Yes / No |
| 是否需要前端页面 | Yes / No |
| 是否需要 smoke-test | Yes / No |

### 2. 功能目标

说明这个功能解决什么问题。

### 3. 实现方法论

说明通过什么设计和技术实现。

### 4. 后端实现路径

列出相关文件路径。

### 5. 前端实现路径

列出相关页面、组件、API client。

### 6. 数据库设计

列出涉及的数据表、字段、索引。

### 7. API 设计

列出接口、请求、响应、错误码。

### 8. 核心流程

用步骤或流程图说明运行链路。

### 9. 使用技术

列出技术、库、算法、协议。

### 10. 验收方式

列出 curl、smoke-test、前端验证、数据库验证。

### 11. 已知限制

列出当前未完成或风险点。

### 12. 后续优化

列出下一阶段增强项。
```

---

## 4. v0.1 功能实现记录

---

## 4.1 功能名称：用户注册 / 登录

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.1 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | Yes |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

实现基础用户体系，让用户可以注册、登录，并进入控制台管理 API Key、余额、调用记录和账单。

### 3. 实现方法论

通过邮箱 + 密码注册用户，密码不明文存储，后端使用 bcrypt 做 hash。登录成功后生成 JWT，前端将 JWT 存入 localStorage，用于访问用户控制台 API。

### 4. 后端实现路径

```text
backend/internal/auth/
backend/internal/gateway/
backend/internal/storage/
backend/cmd/server/
```

建议实际代码中追踪：

```text
auth.GenerateJWT()
auth.HashPassword()
auth.CheckPassword()
gateway.register()
gateway.login()
storage.CreateUser()
storage.GetUserByEmail()
```

### 5. 前端实现路径

```text
frontend/src/pages/Login.tsx
frontend/src/lib/api.ts
frontend/src/App.tsx
```

### 6. 数据库设计

涉及表：

```text
users
```

核心字段：

```text
id
email
password_hash
balance_usd
created_at
updated_at
```

### 7. API 设计

```text
POST /api/user/auth/register
POST /api/user/auth/login
```

注册响应包含：

```json
{
  "token": "jwt_xxx",
  "user": {
    "id": "uuid",
    "email": "user@example.com"
  }
}
```

### 8. 核心流程

```text
Frontend Register Form
→ POST /api/user/auth/register
→ validate email/password
→ bcrypt hash password
→ insert users
→ grant initial credit
→ create billing transaction
→ generate JWT
→ return token
→ frontend stores JWT
→ redirect Dashboard
```

### 9. 使用技术

```text
Go Fiber
JWT
bcrypt
PostgreSQL
localStorage
```

### 10. 验收方式

```bash
curl -X POST http://localhost:8080/api/user/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

前端验收：

```text
打开 Login 页面
注册用户
自动跳转 Dashboard
刷新页面后仍保持登录
```

### 11. 已知限制

```text
暂未实现 refresh token
暂未实现邮箱验证
暂未实现忘记密码
暂未实现 SSO
```

### 12. 后续优化

```text
v0.3 加 Organization / Workspace
v0.3 加 RBAC
v0.6 加审计和登录安全策略
```

---

## 4.2 功能名称：API Key 生成与管理

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.1 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | Yes |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

让用户可以创建用于模型调用的 API Key。模型调用接口不使用 JWT，而使用 API Key 鉴权。

### 3. 实现方法论

API Key 使用安全随机数生成。后端只保存 hash，不保存明文。明文 key 只在创建时返回一次。数据库保存 key prefix，用于展示和定位。调用模型时，用户在 Header 中传入：

```text
Authorization: Bearer sk-aag-xxxxx
```

后端对传入 key 做 hash 后匹配数据库。

### 4. 后端实现路径

```text
backend/internal/auth/
backend/internal/gateway/
backend/internal/storage/
```

建议追踪函数：

```text
auth.GenerateAPIKey()
auth.HashAPIKey()
auth.VerifyAPIKey()
storage.CreateAPIKey()
storage.ListAPIKeys()
storage.ValidateAPIKey()
storage.RevokeAPIKey()
```

### 5. 前端实现路径

```text
frontend/src/pages/ApiKeys.tsx
frontend/src/lib/api.ts
frontend/src/pages/Playground.tsx
```

### 6. 数据库设计

涉及表：

```text
api_keys
```

核心字段：

```text
id
user_id
name
key_hash
key_prefix
permissions
is_active
last_used_at
created_at
```

### 7. API 设计

```text
GET    /api/user/keys
POST   /api/user/keys
DELETE /api/user/keys/:id
```

创建响应：

```json
{
  "key": "sk-aag-plaintext-only-once",
  "api_key": {
    "id": "uuid",
    "name": "Default Key",
    "key_prefix": "sk-aag-abc123",
    "is_active": true
  }
}
```

### 8. 核心流程

```text
Frontend API Keys Page
→ User clicks Create Key
→ POST /api/user/keys with JWT
→ backend generates secure random key
→ backend hashes key
→ backend stores hash + prefix
→ backend returns plaintext once
→ frontend shows key once
→ frontend stores current API key
→ Playground can call model API
```

### 9. 使用技术

```text
crypto/rand
SHA-256 or secure hash function
PostgreSQL
JWT for management API
Bearer API Key for model API
localStorage / sessionStorage
```

### 10. 验收方式

```bash
curl -X POST http://localhost:8080/api/user/keys \
  -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Default Key"}'
```

模型调用验收：

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-aag-xxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen-plus","messages":[{"role":"user","content":"hello"}]}'
```

### 11. 已知限制

```text
已实现基础 key scope：permissions.models 支持 "*" 或指定 model_id 列表
已实现 per-key model access control
已实现 Admin per-key RPM / TPM / expiry 配置
已实现同步 token-producing 路径的 per-key TPM enforcement
已实现 streaming chat preflight TPM enforcement
暂未实现 key rotation
用户侧暂未实现自助 key expiration 编辑；Admin 可编辑 expires_at
```

### 12. 后续优化

```text
继续增强 API Key rotation
继续增强 streaming chat token 估算精度
继续增强用户侧 key expiry / permission self-service
继续增强 API Key 审计维度
```

---

## 4.3 功能名称：OpenAI-compatible Chat Completion API

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.1 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | No |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

提供兼容 OpenAI 格式的 `/v1/chat/completions` 接口，让用户可以用标准格式调用模型。

### 3. 实现方法论

后端暴露 OpenAI-compatible endpoint。请求进入后执行 API Key 鉴权、rate limit、余额检查、模型路由、Provider 调用、usage 记录、billing 扣费，最后返回 OpenAI-compatible response。

### 4. 后端实现路径

```text
backend/internal/gateway/
backend/internal/router/
backend/internal/provider/
backend/internal/billing/
backend/internal/storage/
backend/internal/ratelimit/
```

建议追踪函数：

```text
gateway.handleChatCompletions()
gateway.handleNonStreamChat()
gateway.handleStreamChat()
router.Route()
provider.ChatCompletion()
billing.CalculateCost()
storage.RecordUsage()
storage.DeductBalance()
```

### 5. 前端实现路径

```text
frontend/src/pages/Playground.tsx
frontend/src/lib/api.ts
```

### 6. 数据库设计

涉及表：

```text
models
model_providers
providers
usage_logs
billing_transactions
api_keys
users
```

### 7. API 设计

```text
POST /v1/chat/completions
```

请求：

```json
{
  "model": "qwen-plus",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "temperature": 0.7,
  "max_tokens": 512,
  "stream": false
}
```

响应：

```json
{
  "id": "chatcmpl_xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "qwen-plus",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello!"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

### 8. 核心流程

```text
Client / Playground
→ POST /v1/chat/completions
→ API Key auth
→ rate limit check
→ balance check
→ model existence check
→ router selects provider
→ map aggregator model to upstream model
→ call provider
→ normalize response
→ calculate tokens and cost
→ write usage_logs
→ deduct balance
→ write billing_transactions
→ return response
```

### 9. 使用技术

```text
Go Fiber
OpenAI-compatible API format
Provider Adapter Pattern
PostgreSQL
Redis Rate Limit
SSE for stream
```

### 10. 验收方式

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-plus",
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

验证数据库：

```sql
SELECT * FROM usage_logs ORDER BY created_at DESC LIMIT 5;
SELECT * FROM billing_transactions ORDER BY created_at DESC LIMIT 5;
SELECT balance_usd FROM users WHERE email = 'test@example.com';
```

### 11. 已知限制

```text
stream billing 仍需加强
OpenAI edge cases 未完全覆盖
tool calling 未实现
JSON mode 未实现
vision input 未实现
```

### 12. 后续优化

```text
v0.2 增加 request_logs
v0.2 增加 failover
v0.2 增加 provider health
v0.4 增加更多 provider
v0.7 增加 workflow / agent API
```

---

## 4.4 功能名称：Billing / Balance / Usage Charge

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.1 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | Yes |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

实现最小商业闭环：用户调用模型后产生用量，并根据模型价格扣减余额。

### 3. 实现方法论

用户注册后获得测试 credit。每次模型调用完成后，根据 prompt tokens、completion tokens 和数据库中的模型价格计算费用，扣减用户 balance，并写入 billing_transactions。

### 4. 后端实现路径

```text
backend/internal/billing/
backend/internal/storage/
backend/internal/gateway/
```

建议追踪函数：

```text
billing.CalculateCost()
storage.GetModelPricing()
storage.RecordUsage()
storage.DeductBalance()
storage.CreateBillingTransaction()
storage.GetUserBalance()
storage.ListBillingTransactions()
```

### 5. 前端实现路径

```text
frontend/src/pages/Billing.tsx
frontend/src/pages/Dashboard.tsx
frontend/src/lib/api.ts
```

### 6. 数据库设计

涉及表：

```text
users
models
usage_logs
billing_transactions
```

核心字段：

```text
users.balance_usd
models.input_price
models.output_price
models.price_unit
usage_logs.charged_cost_usd
billing_transactions.amount_usd
billing_transactions.balance_after_usd
billing_transactions.tx_type
```

### 7. API 设计

```text
GET /api/user/billing/balance
GET /api/user/billing/transactions
```

### 8. 核心流程

```text
Chat completion success
→ get provider usage tokens
→ if no usage, estimate tokens
→ get model pricing from DB
→ calculate charged_cost_usd
→ insert usage_logs
→ update users.balance_usd
→ insert billing_transactions with tx_type=usage_charge
→ return response
```

### 9. 使用技术

```text
PostgreSQL transaction
NUMERIC/DECIMAL for money
DB model pricing
prepaid balance model
```

### 10. 验收方式

```text
调用模型前记录 balance
调用 /v1/chat/completions
调用后 balance 应减少
billing_transactions 应新增 usage_charge
usage_logs 应新增 charged_cost_usd
```

### 11. 已知限制

```text
暂未实现充值支付
已实现 invoice / PO / postpaid baseline：organization 支持 payment terms 和 default PO，Admin 可按 workspace 生成 invoice draft 并从 billing_transactions 自动计算 subtotal/total/due date。
已实现 workspace cost attribution baseline：通过 API Key 的 workspace_id 传递到 request_logs、usage_logs、billing_transactions；workspace usage API 已返回总量和 Top model/provider/user 成本归因，Admin Workspaces 已展示。
暂未实现 gross margin dashboard
```

### 12. 后续优化

```text
v0.2 增加 upstream_cost_usd / gross_margin_usd
v0.3 增加 quota / budget
v0.3 继续增强 cost center / workspace billing：已完成 workspace/project 归因、预算、配额、报表汇总、CSV 导出、invoice/PO/postpaid baseline、invoice CSV export、invoice PDF export 和 status workflow；后续增强时间窗口筛选、monthly statement、regional tax rules 和 project invoice grouping。
v0.3 增加 admin grant credit
```

---

## 5. v0.2 功能实现记录

---

## 5.1 功能名称：Request Logs

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.2 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | Yes |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

让每一次请求都可以通过 request_id 追踪，解决生产环境中难以排查错误、延迟、provider、billing 和 fallback 的问题。

### 3. 实现方法论

新增 `request_logs` 表，在 `/v1/chat/completions` 成功和失败路径都写入 request log。记录请求的模型、provider、final provider、状态码、错误、延迟、tokens、cost、request preview 和 response preview。前端提供 Request Logs 页面用于查询、过滤、分页、CSV 导出和查看详情。

### 4. 后端实现路径

```text
backend/internal/storage/
backend/internal/gateway/
backend/internal/middleware/
migrations/003_v02_observability.sql
```

建议追踪函数：

```text
storage.RecordRequestLog()
storage.ListRequestLogs()
storage.GetRequestLogByRequestID()
gateway.recordRequestLog()
gateway.writeError()
```

### 5. 前端实现路径

```text
frontend/src/pages/RequestLogs.tsx
frontend/src/lib/api.ts
frontend/src/components/Layout.tsx
```

### 6. 数据库设计

涉及表：

```text
request_logs
```

核心字段：

```text
id
request_id
user_id
api_key_id
model_id
provider_id
final_provider_id
method
path
status_code
error_code
error_message
latency_ms
input_tokens
output_tokens
total_tokens
charged_cost_usd
upstream_cost_usd
gross_margin_usd
fallback_count
request_preview
response_preview
created_at
```

### 7. API 设计

```text
GET /api/user/request-logs
GET /api/user/request-logs/:request_id
```

`GET /api/user/request-logs` query params:

```text
limit
offset
model
provider
status=all|success|error
from
to
format=csv
```

查询参数建议：

```text
limit
offset
model
provider
status
from
to
```

### 8. 核心流程

```text
Request enters gateway
→ request_id generated
→ request processed
→ success or failure
→ build request log
→ insert request_logs
→ frontend fetches request logs
→ user opens detail by request_id
```

### 9. 使用技术

```text
PostgreSQL
request_id middleware
structured error
JSON preview
React table/detail panel
```

### 10. 验收方式

```bash
curl -X GET http://localhost:8080/api/user/request-logs \
  -H "Authorization: Bearer <JWT>"
```

验证：

```text
成功请求有 request_logs
失败请求有 request_logs
request_id 可查详情
前端 Request Logs 页面展示正常
服务端 limit/offset 分页正常
服务端 model/status/provider/date 过滤正常
CSV 导出正常，且不包含 request_preview / response_preview
```

### 11. 已知限制

```text
request_preview / response_preview 需要脱敏策略
```

### 12. 后续优化

```text
v0.2 和 fallback_logs 联动
v0.3 加 workspace 维度
v0.6 加 PII masking
```

---

## 5.2 功能名称：Provider Health Check

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.2 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | Yes |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

监测每个 Provider 的健康状态，让路由器可以避开不可用 Provider，并在前端展示 Provider 状态。

### 3. 实现方法论

Provider Adapter 实现 `HealthCheck(ctx)`。Router 定时调用 health check，并更新 provider 或 model_provider 健康状态。每次 health check 结果应写入 `provider_health_checks` 表，用于历史追踪和前端展示。

### 4. 后端实现路径

```text
backend/internal/provider/
backend/internal/router/
backend/internal/storage/
migrations/003_v02_observability.sql
```

建议追踪函数：

```text
provider.HealthCheck()
router.healthCheckLoop()
storage.UpdateProviderHealth()
storage.RecordProviderHealthCheck()
storage.ListLatestProviderHealth()
gateway.adminProviderHealth()
gateway.adminProviderHealthCheck()
```

### 5. 前端实现路径

```text
frontend/src/pages/Admin.tsx
frontend/src/lib/api.ts
```

### 6. 数据库设计

涉及表：

```text
provider_health_checks
model_providers
providers
```

核心字段：

```text
provider_health_checks.id
provider_health_checks.provider_id
provider_health_checks.status
provider_health_checks.latency_ms
provider_health_checks.error_message
provider_health_checks.checked_at

model_providers.health_status
model_providers.last_health_chk
```

### 7. API 设计

已实现接口：

```text
GET /api/admin/provider-health
GET /api/admin/providers/:id/health/history
POST /api/admin/providers/:id/health-check
```

### 8. 核心流程

```text
Background health loop
→ load enabled providers
→ call provider.HealthCheck()
→ calculate latency
→ update health_status
→ insert provider_health_checks
→ frontend fetches provider health
→ router filters unhealthy providers
```

### 9. 使用技术

```text
Go background goroutine
context timeout
Provider Adapter interface
PostgreSQL health history
React Provider Status page
```

### 10. 当前完成情况

```text
已实现 startup + background health loop
已更新 model_providers.health_status
已新增并写入 provider_health_checks 表
已实现 Provider Health API
已在 Admin Provider Status 页面接入真实 API
已支持手动触发 provider health check
已支持按 provider 查询最近 N 次 health check history
已在 Admin Provider Status 页面展示 health history panel
```

### 11. 验收方式

```text
手动让某 provider health check 失败
数据库 provider_health_checks 应新增失败记录
Provider Status 页面显示 unhealthy
Router 不再选择 unhealthy provider

已验证：
GET /api/admin/provider-health 返回 3 个 provider
POST /api/admin/providers/:id/health-check 返回 healthy 和 latency_ms
provider_health_checks 表持续写入历史记录
GET /api/admin/providers/:id/health/history?limit=5 返回最近历史，包含 status / latency_ms / checked_at
```

### 12. 后续优化

```text
provider 24h request/error/fallback 聚合已完成并由 `scripts/regression/provider-health-stats.sh` 覆盖
v0.3 接入 workspace / org 级可见性
v0.5 用 health status 参与 smart routing
```

---

## 5.3 功能名称：Provider Failover / Fallback Chain

### 1. 基本信息

| 字段 | 内容 |
|---|---|
| 所属阶段 | v0.2 |
| 当前状态 | Verified |
| 是否影响主链路 | Yes |
| 是否需要 migration | Yes |
| 是否需要前端页面 | Yes |
| 是否需要 smoke-test | Yes |

### 2. 功能目标

当主 Provider 调用失败时，自动尝试下一个可用 Provider，提升 Gateway 可用性。

### 3. 实现方法论

Router 不再只返回单个 route，而是返回按优先级排序的 provider route 列表。Gateway 调用时逐个尝试。如果当前 provider 失败，则记录失败原因并尝试下一个 provider。最终成功时记录 final_provider 和 fallback_count。失败过程应写入 fallback_logs。

### 4. 后端实现路径

```text
backend/internal/router/
backend/internal/gateway/
backend/internal/provider/
backend/internal/storage/
migrations/003_v02_observability.sql
```

建议追踪函数：

```text
router.RouteAll()
gateway.handleNonStreamChat()
gateway.handleStreamChat()
storage.RecordFallbackLog()
storage.RecordRequestLog()
```

### 5. 前端实现路径

```text
frontend/src/pages/RequestLogs.tsx
frontend/src/pages/Admin.tsx
```

### 6. 数据库设计

涉及表：

```text
fallback_logs
request_logs
```

核心字段：

```text
fallback_logs.id
fallback_logs.request_id
fallback_logs.model_id
fallback_logs.from_provider_id
fallback_logs.to_provider_id
fallback_logs.error_code
fallback_logs.error_message
fallback_logs.created_at

request_logs.primary_provider_id
request_logs.final_provider_id
request_logs.fallback_count
```

### 7. API 设计

可选新增：

```text
GET /api/user/request-logs/:request_id/fallbacks
```

或直接在 request detail 中返回 fallback chain。

### 8. 核心流程

```text
Request model=qwen-plus
→ Router returns provider routes [providerA, providerB]
→ call providerA
→ providerA fails
→ write fallback_logs
→ call providerB
→ providerB succeeds
→ write request_logs with final_provider=providerB, fallback_count=1
→ return success response
```

### 9. 使用技术

```text
Provider Adapter Pattern
Priority routing
Fallback chain
Structured provider error
PostgreSQL fallback logs
Mock failure mode for test
```

### 10. 当前完成情况

```text
RouteAll 已有
Gateway 已可循环尝试多个 provider
request_logs 可记录 fallback_count
fallback_logs 表已建
fallback_logs 写入已完成
Mock failure mode 已完成：
  MOCK_FAIL_PROVIDER_IDS=bailian_cn
Mock health failure 已独立配置：
  MOCK_FAIL_HEALTH_PROVIDER_IDS=bailian_cn
Fallback smoke-test 已脚本固化：
  RUN_FALLBACK_SMOKE=true
```

### 11. 验收方式

```text
配置 primary provider 强制失败
secondary provider 正常
调用 /v1/chat/completions 应成功
request_logs.final_provider_id 应为 secondary provider
request_logs.fallback_count 应为 1
fallback_logs 已记录 providerA → providerB
当前已通过手动 fallback 验证，并已固化到 `scripts/smoke-test.sh` 的可选回归项
验证结果示例：
fallback_logs=1
transition=bailian_cn->bailian_intl
final_provider=bailian_intl
```

### 12. 后续优化

```text
v0.2 后续把 fallback smoke-test 接入 CI 或 release checklist
v0.5 支持 latency / cost / quality based fallback
```

---

## 6. 界面实现追踪

### 6.1 Login 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/Login.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `POST /api/user/auth/register`, `POST /api/user/auth/login` |
| 使用技术 | React, TypeScript, localStorage, JWT |
| 当前状态 | 已完成 |
| 验收方式 | 注册/登录成功，刷新页面保持登录 |

### 6.2 API Keys 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/ApiKeys.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET /api/user/keys`, `POST /api/user/keys`, `DELETE /api/user/keys/:id` |
| 数据库表 | `api_keys` |
| 使用技术 | React, Clipboard API, localStorage |
| 当前状态 | 已完成 |
| 验收方式 | 创建 key，复制 key，revoke key，revoked key 不可调用 |

### 6.3 Playground 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/Playground.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET /v1/models`, `POST /v1/chat/completions` |
| 数据库表 | `models`, `model_providers`, `usage_logs`, `billing_transactions` |
| 使用技术 | React, OpenAI-compatible API, API Key Bearer Auth |
| 当前状态 | 已完成 |
| 验收方式 | 选择模型，输入 prompt，返回模型响应，usage/billing 增加 |

### 6.4 Dashboard 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/Dashboard.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET /api/user/dashboard`, `GET /api/user/usage` |
| 数据库表 | `usage_logs`, `billing_transactions` |
| 使用技术 | React, summary cards, usage table, avg/p95/p99 latency, error rate |
| 当前状态 | 已完成 |
| 验收方式 | 调用模型后 Dashboard 数字变化 |

### 6.5 Billing 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/Billing.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET /api/user/billing/balance`, `GET /api/user/billing/transactions` |
| 数据库表 | `users`, `billing_transactions` |
| 使用技术 | React, transaction table |
| 当前状态 | 已完成 |
| 验收方式 | 显示余额、credit_grant、usage_charge |

### 6.6 Request Logs 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/RequestLogs.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET /api/user/request-logs`, `GET /api/user/request-logs/:request_id` |
| 数据库表 | `request_logs` |
| 使用技术 | React, log table, detail drawer / modal |
| 当前状态 | 基本完成 |
| 验收方式 | 每次请求后可查看 request_id、provider、latency、tokens、cost、preview |

### 6.7 Provider Status 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/ProviderStatus.tsx` 或 `Admin.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET /api/admin/provider-health` |
| 数据库表 | `provider_health_checks`, `model_providers` |
| 使用技术 | React, provider health table, manual check action, health history panel |
| 当前状态 | 已完成 / 已接真实 API |
| 验收方式 | Provider health 变化后页面真实展示；可手动触发 health check；可查看 provider health history |

### 6.8 Admin Models 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/AdminModels.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET/POST/PATCH/DELETE /api/admin/models` |
| 数据库表 | `models`, `model_provider_bindings` |
| 使用技术 | React, CRUD form, provider binding inline editor |
| 当前状态 | 已完成 / 已接真实 API |
| 验收方式 | Admin 修改模型价格后下一次调用按新价格计费；Admin 修改 provider upstream_model 后 router registry 刷新并影响下一次路由 |

### 6.9 Admin Providers 页面

| 项目 | 内容 |
|---|---|
| 页面路径 | `frontend/src/pages/AdminProviders.tsx` |
| API Client | `frontend/src/lib/api.ts` |
| 后端接口 | `GET/POST/PATCH/DELETE /api/admin/providers` |
| 数据库表 | `providers`, `model_providers` |
| 使用技术 | React, CRUD form, priority config |
| 当前状态 | 未完成 |
| 验收方式 | Admin 禁用 provider 后 router 不再选择该 provider |

---

## 7. 技术实现方法论索引

### 7.1 API Key 生成方法论

| 项目 | 内容 |
|---|---|
| 目标 | 安全生成模型调用凭证 |
| 技术 | `crypto/rand`, hash, prefix |
| 后端模块 | `auth`, `storage`, `gateway` |
| 数据表 | `api_keys` |
| 安全原则 | 明文只返回一次，数据库只存 hash |
| 调用方式 | `Authorization: Bearer sk-aag-xxx` |
| 后续优化 | scope, expiration, rotation, workspace key |

### 7.2 JWT 登录方法论

| 项目 | 内容 |
|---|---|
| 目标 | 控制台用户鉴权 |
| 技术 | JWT, bcrypt |
| 后端模块 | `auth`, `gateway` |
| 前端模块 | `Login.tsx`, `api.ts` |
| 存储位置 | frontend localStorage |
| 后续优化 | refresh token, SSO, RBAC |

### 7.3 Provider Adapter 方法论

| 项目 | 内容 |
|---|---|
| 目标 | 统一不同模型供应商接口 |
| 技术 | Adapter Pattern |
| 后端模块 | `provider` |
| 当前实现 | Mock Provider, DashScope Provider |
| 标准方法 | `ChatCompletion`, `ChatCompletionStream`, `HealthCheck` |
| 后续优化 | OpenAI-compatible, BYOK, self-hosted provider |

### 7.4 Routing 方法论

| 项目 | 内容 |
|---|---|
| 目标 | 根据模型选择合适 Provider |
| 技术 | priority routing, fallback chain |
| 后端模块 | `router` |
| 数据表 | `model_providers` |
| 当前实现 | priority route, partial fallback |
| 后续优化 | health-based, cost-based, latency-based, quality-based routing |

### 7.5 Billing 方法论

| 项目 | 内容 |
|---|---|
| 目标 | 调用后计量计费 |
| 技术 | prepaid balance, DB pricing, transaction record |
| 后端模块 | `billing`, `storage` |
| 数据表 | `users`, `usage_logs`, `billing_transactions`, `models` |
| 当前实现 | balance deduction, credit_grant, usage_charge |
| 后续优化 | quota, budget, invoice, cost attribution, gross margin |

### 7.6 Observability 方法论

| 项目 | 内容 |
|---|---|
| 目标 | 每次请求可追踪 |
| 技术 | request_id, request_logs, structured error |
| 后端模块 | `observability`, `gateway`, `storage` |
| 前端页面 | `RequestLogs.tsx` |
| 当前实现 | request logs basic |
| 后续优化 | traces, p95/p99, provider health history, fallback chain |

---

## 8. 每次开发后必须填写的变更记录模板

```markdown
## YYYY-MM-DD — <Feature / Module Name>

### 1. 本次变更目标

### 2. 修改文件

#### 后端

- `backend/...`

#### 前端

- `frontend/...`

#### 数据库

- `migrations/...`

#### 文档

- `docs/...`

### 3. 新增 API

- `GET /...`
- `POST /...`

### 4. 新增数据库表 / 字段

- table.field

### 5. 使用的技术 / 方法论

- JWT / bcrypt / API Key hash / Adapter Pattern / etc.

### 6. 验收方式

- curl
- smoke-test
- frontend manual test
- SQL check

### 7. 是否影响 v0.1 主链路

Yes / No

### 8. 是否通过回归测试

- `go test ./...`: Pass / Fail / Not Run
- `docker compose up`: Pass / Fail / Not Run
- `smoke-test`: Pass / Fail / Not Run
- `frontend build`: Pass / Fail / Not Run

### 9. 已知限制

### 10. 下一步
```

---

## 9. 后续维护规则

每个功能开发完成后，必须同步更新：

```text
1. PROJECT_STAGE_TRACKER.md
2. FEATURE_IMPLEMENTATION_TRACKER.md
3. DATA_MODEL.md
4. API_DESIGN.md
5. ROADMAP.md
6. README.md 或 API_EXAMPLES.md
```

如果新增页面，必须更新：

```text
UI_PAGE_SPEC.md
FEATURE_IMPLEMENTATION_TRACKER.md
```

如果新增数据库表，必须更新：

```text
DATA_MODEL.md
migrations/
FEATURE_IMPLEMENTATION_TRACKER.md
```

如果新增 API，必须更新：

```text
API_DESIGN.md
API_EXAMPLES.md
FEATURE_IMPLEMENTATION_TRACKER.md
```

如果新增技术方法论，例如新的 Provider Adapter、BYOK、Guardrails、Workflow Engine，必须更新：

```text
ARCHITECTURE.md
MODULE_BREAKDOWN.md
FEATURE_IMPLEMENTATION_TRACKER.md
```

---

## 10. 当前下一步必须补充的追踪项

当前最需要补全的功能追踪卡片：

```text
1. v0.3 Organization / Workspace API 实现卡片
2. v0.3 RBAC Enforcement 实现卡片
3. Unified Error Format 完善卡片
4. OpenAI-compatible Provider 规划卡片
```

---

## 11. 功能卡片：User Settings / Profile UI

### 1. 功能名称

User Settings / Profile UI

### 2. 状态

Verified

### 3. 变更日期

2026-06-24

### 4. 改动文件

#### 前端

- `frontend/src/lib/api.ts`
- `frontend/src/pages/Settings.tsx`
- `frontend/src/App.tsx`
- `frontend/src/components/layout/DashboardLayout.tsx`

#### 文档

- `ai_aggregator_codex_pack/PROJECT_STAGE_TRACKER.md`
- `ai_aggregator_codex_pack/FEATURE_IMPLEMENTATION_TRACKER.md`
- `ai_aggregator_codex_pack/09_API_DESIGN.md`

### 5. 新增 / 补全 API Client

- `UserProfile`
- `getProfile()`
- `updateProfile()`

### 6. 功能说明

- Dashboard sidebar Settings 入口指向 `/dashboard/settings`
- Settings 页面加载当前登录用户 profile
- 支持编辑 `username`
- 支持编辑 `metadata` JSON，并在保存前进行 JSON validation
- 展示账户 email、role、balance
- 未登录用户自动跳转 `/login`

### 7. 验收方式

- `npm run build`: Pass
- `go test ./...`: Pass
- `bash -n scripts/smoke-test.sh`: Pass
- `curl http://localhost:5175/dashboard/settings`: Pass
- `scripts/dev-check.sh`: Pass
- Local API smoke:
  - register test user: 201
  - `GET /api/user/profile`: 200
  - `PUT /api/user/profile`: 200
  - returned username and metadata match update payload

### 8. 当前阶段判断

```text
v1.0 foundation = Verified / user-facing profile settings page completed
```

---

## 12. 功能卡片：Admin Control Plane 补全

### 1. 功能名称

Admin Users / Analytics / Settings / Alerts / Audit Log 基础闭环

### 2. 状态

Verified

### 3. 变更日期

2026-06-24

### 4. 改动文件

#### 后端

- `backend/internal/auth/middleware.go`
- `backend/internal/gateway/router.go`
- `backend/internal/storage/store.go`
- `backend/Dockerfile`

#### 前端

- `frontend/src/lib/api.ts`
- `frontend/src/pages/Admin.tsx`

#### 文档

- `ai_aggregator_codex_pack/PROJECT_STAGE_TRACKER.md`
- `ai_aggregator_codex_pack/FEATURE_IMPLEMENTATION_TRACKER.md`
- `ai_aggregator_codex_pack/09_API_DESIGN.md`
- `README.md`

### 5. 新增 / 补全 API

- `POST /api/user/auth/refresh`
- `GET /api/user/profile`
- `PUT /api/user/profile`
- `GET /api/admin/users`
- `POST /api/admin/users`
- `GET /api/admin/users/:id`
- `PUT /api/admin/users/:id`
- `GET /api/admin/users/:id/usage`
- `GET /api/admin/keys`
- `PUT /api/admin/keys/:id`
- `DELETE /api/admin/keys/:id`
- `GET /api/admin/analytics/overview`
- `GET /api/admin/analytics/usage`
- `GET /api/admin/analytics/cost`
- `GET /api/admin/analytics/latency`
- `GET /api/admin/analytics/errors`
- `GET /api/admin/settings`
- `PUT /api/admin/settings/:key`
- `GET /api/admin/alerts/rules`
- `POST /api/admin/alerts/rules`
- `PUT /api/admin/alerts/rules/:id`
- `GET /api/admin/alerts/history`
- `POST /api/admin/alerts/history/:id/ack`
- `POST /api/admin/alerts/history/:id/resolve`
- `GET /api/admin/audit-logs`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/speech`

### 6. 新增数据库表 / 字段

无新增表。复用：

- `users`
- `api_keys`
- `usage_logs`
- `request_logs`
- `system_settings`
- `provider_health_checks`
- `audit_logs`

### 7. 使用的技术 / 方法论

- JWT refresh by validated existing JWT claims
- PostgreSQL aggregate analytics over `request_logs`
- System setting KV read/write with Redis cache invalidation
- Provider health + request error-rate derived alert summaries
- Persistent Alert Rule CRUD in PostgreSQL
- Persistent Alert Event history with acknowledge / manual resolve lifecycle
- OpenAI-compatible audio gateway forwarding to provider adapter
- Optional smoke-test coverage for embeddings and image/video async APIs
- Admin API key metadata listing, control editing, and revoke without exposing secrets
- Per-key RPM override via Redis sliding-window limiter
- Per-key TPM enforcement via Redis minute counter on synchronous token-producing paths and streaming preflight
- API key `permissions.models` enforcement before single-model `/v1` provider dispatch
- Admin UI follows existing card/table layout
- Docker build uses committed `go.mod` / `go.sum` module graph

### 8. 验收方式

- `go test ./...`: Pass
- `npm run build`: Pass
- `docker compose build backend`: Pass
- `docker compose up -d backend`: Pass
- `scripts/dev-check.sh`: Pass
- Local API smoke:
  - `GET /health`: 200
  - `GET /api/user/profile`: 200
  - `POST /api/user/auth/refresh`: 200
  - `GET /api/admin/users`: 200
  - `GET /api/admin/analytics/overview`: 200
  - `GET /api/admin/settings`: 200
  - `PUT /api/admin/settings/default_markup`: 200
  - `GET /api/admin/alerts/history`: 200
  - `POST /api/admin/alerts/rules`: 201
  - `PUT /api/admin/alerts/rules/:id`: 200
  - `POST /api/admin/alerts/history/:id/ack`: 200
  - `POST /api/admin/alerts/history/:id/resolve`: 200
  - resolved alert remains resolved after refreshing history: Pass
  - `GET /api/admin/audit-logs`: 200
  - `GET /api/admin/keys`: 200
  - `PUT /api/admin/keys/:id`: 200
  - per-key `rate_limit_rpm=1` returns `/v1/models`: 200 then 429
  - per-key `rate_limit_tpm=1` returns `/v1/chat/completions`: 429 with `tpm_limit`
  - per-key `rate_limit_tpm=1` returns `/v1/chat/completions` with `stream=true`: 429 with `tpm_limit`
  - per-key `permissions.models=["qwen-turbo"]` rejects `/v1/embeddings` with `text-embedding-v2`: 403
  - `DELETE /api/admin/keys/:id`: 200
  - revoked key rejected by `/v1/models`: 401
  - `bash -n scripts/smoke-test.sh`: Pass
  - Embedding smoke branch: runs in mock mode by default; real provider execution requires `RUN_EMBEDDING_SMOKE=true`
  - Image/video async smoke branch: runs in mock mode by default; real provider execution requires `RUN_ASYNC_SMOKE=true`

### 9. 是否影响 v0.1 主链路

No. `/v1/models` 与 `/v1/chat/completions` 主链路未改。

### 10. 已知限制

- Alert Rule 与 alert event history 已持久化；ack / manual resolve 已完成；notification channel / silence policy / auto-resolve 尚未实现。
- Admin API Keys 页面支持真实 key 元数据列表、revoke、per-key rate limit / permissions 编辑；TPM 已在同步 token-producing 路径和 streaming preflight 接入。
- Analytics 当前使用 PostgreSQL 聚合，生产高流量后建议接 ClickHouse。
- 细粒度 RBAC enforcement 仍需按 workspace/role/permission 继续增强。
- Audio endpoint 网关层已完成并由 `scripts/regression/audio-mock.sh` 覆盖 mock STT/TTS、request_logs、usage_logs；DashScope audio adapter 仍需按百炼音频 API 映射。
- File endpoint 已新增基础治理 metadata、本地 EICAR malware-scan baseline 和 retention cleanup；生产 object storage adapter 仍需设计与实现。

### 11. 下一步

- 完成 File / Audio OpenAI-compatible endpoints。
- 增加 Admin API Key revoke / per-key details。
- 增加 workspace-level analytics and audit filters。
- 为 alert rules 增加表结构、CRUD 与触发历史。

---

## 13. 功能卡片：File Upload Foundation

### 1. 功能名称

OpenAI-compatible `/v1/files` 本地开发存储闭环

### 2. 状态

Verified

### 3. 变更日期

2026-06-24

### 4. 改动文件

#### 后端

- `backend/internal/config/config.go`
- `backend/internal/filestore/store.go`
- `backend/internal/gateway/router.go`
- `backend/internal/storage/store.go`
- `docker-compose.yml`

#### 数据库

- `migrations/011_v11_file_upload_foundation.sql`
- `migrations/014_v14_file_governance_settings.sql`
- `migrations/015_v15_file_retention_settings.sql`

#### 文档

- `README.md`
- `ai_aggregator_codex_pack/PROJECT_STAGE_TRACKER.md`
- `ai_aggregator_codex_pack/FEATURE_IMPLEMENTATION_TRACKER.md`
- `ai_aggregator_codex_pack/09_API_DESIGN.md`

### 5. 新增 API

- `POST /v1/files`
- `GET /v1/files`
- `GET /v1/files/:id`
- `GET /v1/files/:id/content`
- `DELETE /v1/files/:id`
- `POST /api/admin/files/retention/run`

### 6. 新增数据库表 / 字段

- `uploaded_files`
  - `id`
  - `user_id`
  - `api_key_id`
  - `workspace_id`
  - `filename`
  - `purpose`
  - `bytes`
  - `mime_type`
  - `storage_path`
  - `status`
  - `metadata`
  - `created_at`
  - `updated_at`
- `system_settings.file_retention_days`

### 7. 使用的技术 / 方法论

- OpenAI-compatible file metadata response
- Multipart upload via Fiber `FormFile`
- File listing with optional `purpose` filter
- Owner-scoped content download
- FileStore interface with local `Put/Get/Delete`
- Local development object storage under `FILE_STORAGE_DIR`
- PostgreSQL ownership enforcement by `user_id`
- Soft-delete metadata plus best-effort local file removal
- `system_settings.max_file_size_mb` upload limit
- `system_settings.allowed_upload_mime_types` MIME allowlist
- `system_settings.file_retention_days` retention cleanup window
- MIME sniffing via `http.DetectContentType`
- SHA-256 checksum metadata
- Audit log on upload/delete
- Admin retention cleanup with `dry_run` and `limit`
- Retention cleanup soft-deletes metadata and best-effort removes local bytes
- Audit log on retention cleanup

### 8. 验收方式

- `go test ./...`: Pass
- `docker compose build backend`: Pass
- `docker compose up -d backend`: Pass
- `npm run build`: Pass
- `scripts/dev-check.sh`: Pass
- `bash -n scripts/smoke-test.sh`: Pass
- File API smoke:
  - `POST /api/user/auth/register`: 201
  - `POST /api/user/keys`: 201
  - `POST /v1/files`: 201
  - `GET /v1/files`: 200
  - `GET /v1/files/:id`: 200 with `metadata.detected_mime` and `metadata.sha256`
  - `GET /v1/files/:id/content`: 200
  - `DELETE /v1/files/:id`: 200
  - disallowed MIME when `allowed_upload_mime_types=application/pdf`: 415
- File retention smoke:
  - `PUT /api/admin/settings/file_retention_days`: 200
  - `POST /api/admin/files/retention/run`: 200 with `deleted_count=1` and `storage_deleted_count=1`
  - `GET /v1/files/:id` after cleanup: 404
  - `file_retention_days` restored to `0`
- FileStore adapter smoke:
  - `POST /v1/files`: 201 with `metadata.source=local`
  - `metadata.sha256` length: 64
  - `GET /v1/files/:id/content`: content matched uploaded bytes
  - `DELETE /v1/files/:id`: deleted true
  - `GET /v1/files/:id` after delete: 404

### 9. 是否影响 v0.1 主链路

No. Chat completion and model listing paths are unchanged.

### 10. 已知限制

- 当前 FileStore adapter 已抽象，具体实现仍是 local；生产应补 Alibaba OSS/S3 adapter。
- 已加入本地 EICAR 签名扫描 baseline；生产建议继续接 ClamAV/云安全扫描。
- 生产对象存储 lifecycle policy 需要随 OSS/S3 adapter 一起接入。

### 11. 下一步

- 实现 Alibaba OSS FileStore adapter。
- 增加文件列表、下载、按 workspace 过滤。
- 将文件与后续 Assistants / Batch / Fine-tune 工作流打通。

---

## 10. 功能卡片：RBAC Workspace Permission Enforcement

### 1. 功能名称

RBAC Workspace Permission Enforcement

### 2. 状态

Verified

### 3. 变更日期

2026-06-25

### 4. 改动文件

#### 后端

- `backend/internal/storage/store.go`
- `backend/internal/gateway/router.go`

#### 文档

- `ai_aggregator_codex_pack/PROJECT_STAGE_TRACKER.md`
- `ai_aggregator_codex_pack/FEATURE_IMPLEMENTATION_TRACKER.md`
- `ai_aggregator_codex_pack/09_API_DESIGN.md`

### 5. 新增 / 补全能力

- `WorkspaceMemberHasPermission()`
- `workspaceRoleNameAllows()`
- `permissionSetAllows()`
- `POST /api/user/keys` with `workspace_id` requires `api_keys:create`
- `POST /v1/files` with workspace context requires `files:write`
- `POST /api/user/workflows` with `workspace_id` requires `workflows:write`
- `POST /api/user/workflows/:id/runs` for workspace workflow requires `workflows:write`
- `POST /api/user/workspaces/:id/budgets` requires `workspace_budgets:write`
- `POST /api/user/workspaces/:id/quotas` requires `workspace_quotas:write`
- `GET /api/user/workspaces/:id/budgets|quotas` requires `usage:read`
- `permission_denied` maps to `permission_error`

### 6. 使用的技术 / 方法论

- Role-name default permission mapping for owner/admin/member/viewer
- Optional custom `roles.permissions` JSON support
- Least-privilege enforcement at write boundary
- Workspace-scoped API key runtime permission check before file storage
- Workspace-scoped workflow create/run permission check before workflow persistence/execution
- OpenAI-style error envelope with `permission_error`

### 7. 验收方式

- `go test ./...`: Pass
- `npm run build`: Pass
- `bash -n scripts/smoke-test.sh`: Pass
- `docker compose build backend`: Pass
- `docker compose up -d backend`: Pass
- Local RBAC smoke:
  - admin creates organization/workspace
  - admin adds user as `viewer`
  - viewer `POST /api/user/keys` with `workspace_id`: 403
  - error code/type: `permission_denied` / `permission_error`
  - admin updates same user to `member`
  - member `POST /api/user/keys` with `workspace_id`: 201
  - controlled viewer workspace API key `POST /v1/files`: 403
  - member workspace API key `POST /v1/files`: 201
  - uploaded file `workspace_id` matches API key workspace
  - viewer `POST /api/user/workflows` with `workspace_id`: 403
  - member `POST /api/user/workflows` with `workspace_id`: 201
  - member `POST /api/user/workflows/:id/runs`: 201 completed
  - workflow and workflow_run `workspace_id` match target workspace
  - viewer `POST /api/user/workspaces/:id/budgets`: 403
  - owner `POST /api/user/workspaces/:id/budgets`: 201
  - owner `POST /api/user/workspaces/:id/quotas` with `tokens_per_month`: 201
  - invalid quota_type `monthly_tokens`: 400 `invalid_request`

### 8. 当前限制 / 下一步

- 当前已完成细粒度 permission 点：`api_keys:create`、`files:write`、`workflows:write`、`workspace_budgets:write`、`workspace_quotas:write`。
- 后续继续扩展 admin-delegated operations 和 custom role management UI。

### 9. 当前阶段判断

```text
v0.3/v1.0 foundation = Verified / workspace role permission enforcement covers api_keys:create, files:write, workflows:write, workspace_budgets:write, and workspace_quotas:write
```

---

## 10. 功能卡片：Admin Audit Log CSV Export

### 1. 功能名称

Admin Audit Log CSV Export

### 2. 状态

Verified

### 3. 变更日期

2026-06-25

### 4. 改动文件

#### 后端

- `backend/internal/gateway/router.go`
- `backend/internal/storage/store.go`

#### 前端

- `frontend/src/lib/api.ts`
- `frontend/src/pages/Admin.tsx`

#### 文档

- `ai_aggregator_codex_pack/PROJECT_STAGE_TRACKER.md`
- `ai_aggregator_codex_pack/FEATURE_IMPLEMENTATION_TRACKER.md`
- `ai_aggregator_codex_pack/09_API_DESIGN.md`

### 5. 新增 / 补全 API

- `GET /api/admin/audit-logs/export`
- `GET /api/admin/audit-logs?action=...&workspace_id=...&resource_type=...&resource_id=...&from=...&to=...`
- `GET /api/admin/audit-logs/export?action=...&workspace_id=...&resource_type=...&resource_id=...&from=...&to=...`
- `POST /api/admin/audit-logs/retention/run`
- `adminDownloadAuditLogsCsv()`
- `adminRunAuditLogRetention()`

### 6. 使用的技术 / 方法论

- Admin JWT authorization
- PostgreSQL `audit_logs` query reuse
- Shared audit filter parser for list and export
- `audit_log_retention_days` system setting
- Dry-run retention cleanup before destructive delete
- CSV response via Go `encoding/csv`
- Download response headers:
  - `Content-Type: text/csv; charset=utf-8`
  - `Content-Disposition: attachment; filename="audit-logs.csv"`
- Admin Audit Log UI download action with Blob URL
- Admin Audit Log UI filters for action, workspace, resource, and date range
- Admin Settings UI audit retention dry-run/run panel

### 7. 验收方式

- `go test ./...`: Pass
- `npm run build`: Pass
- `docker compose build backend`: Pass
- `docker compose up -d backend`: Pass
- `bash -n scripts/smoke-test.sh`: Pass
- `scripts/dev-check.sh`: Pass
- Local API smoke:
  - admin JWT created from local test admin user
  - `GET /api/admin/audit-logs/export?limit=1`: 200
  - `GET /api/admin/audit-logs?action=alert_rule.create&resource_type=alert_rule&resource_id=:id&from=2026-01-01`: 200 with one target event
  - `GET /api/admin/audit-logs/export?action=alert_rule.create&resource_type=alert_rule&resource_id=:id&from=2026-01-01`: 200 with one target event
  - `PUT /api/admin/settings/audit_log_retention_days`: 200
  - `POST /api/admin/audit-logs/retention/run` with `dry_run=true`: matched expired event, deleted_count=0
  - `POST /api/admin/audit-logs/retention/run` with `dry_run=false`: deleted expired event
  - expired audit event removed after cleanup
  - `audit_log_retention_days` restored to `0`
  - response `Content-Type`: `text/csv; charset=utf-8`
  - response `Content-Disposition`: `attachment; filename="audit-logs.csv"`
  - CSV header includes audit id, actor, workspace, action, resource, IP, user agent, details JSON

### 8. 当前限制 / 下一步

- 当前已支持 action / workspace_id / resource_type / resource_id / from / to filters。
- 当前已支持 `audit_log_retention_days` + manual cleanup；后续可增加 scheduler 自动执行。

### 9. 当前阶段判断

```text
v1.0 foundation = Verified / compliance audit export, filter and retention baseline completed
```

---

## 11. 当前最终状态

```text
v0.1：功能实现和方法论基本可追踪
v0.2：Request Logs、Provider Health、Provider 24h Stats、Failover、Fallback Smoke、Admin CRUD、Pricing Update 已完成并通过验证
v0.3：Enterprise Foundation Schema、Organization / Workspace Admin API、Admin Workspaces UI、user dropdown member role/status management、API Key workspace binding、workspace membership enforcement、api_keys:create/files:write/workflows:write/workspace_budgets:write/workspace_quotas:write role permission enforcement、platform/user/workspace scoped BYOK baseline、user BYOK self-service、provider credential validation、request log credential scope、workspace cost attribution Top model/provider/user breakdown、workspace usage/cost CSV export、Budget/Quota runtime enforcement 与 delegated owner/admin management、Admin Workspaces budget/quota create/list UI、Admin Users / Credit Grant、Admin Analytics、Admin Settings、Admin Alerts、Audit Logs 基础闭环已完成并通过 API smoke / regression 验证；真实 upstream smoke、更多 admin-delegated permission 点后续扩展
v0.4：Model Marketplace API、Models 页面升级与 pricing history / price-change audit baseline 已完成并通过 frontend build、Go test、Marketplace API smoke、pricing history regression 验证；benchmark_score 暂为 v0.5 接入项
v0.5：Benchmark Foundation 已完成并通过 Go test、Benchmark API smoke、标准 smoke、frontend build 验证；已补 Admin Benchmarks UI，当前为本地确定性评分，真实 LLM Judge、A/B Test、Smart Routing 权重后续增强
v0.6：Guardrails Foundation 已完成并通过 Go test、PII masking/API block smoke、Guardrails API smoke、标准 smoke 验证；已补 Admin Guardrails UI，当前为本地规则引擎，完整 moderation、retention、合规导出后续增强
v0.7：Workflow / Agent Foundation 已完成并通过 Go test、workflow API smoke、标准 smoke、frontend build 验证；已补用户侧 Workflows UI、webhook callback delivery audit baseline、tool credentials baseline、prompt templates baseline 与 agent sessions baseline，当前为同步 runner，后续增强真实 webhook delivery worker、生产 KMS/Vault、session memory/context summarization
v1.0：Private / Self-hosted Inference Foundation 已完成并通过 Go test、self-hosted registration/list smoke、标准 smoke 验证；已补 Admin Inference UI、用户 Files UI 和基础 Helm chart，当前支持 OpenAI-compatible/vLLM/SGLang 基础接入与本地文件治理闭环，GPU 调度/生产集群 Helm 实测/生产对象存储后续增强
v1.0 provider chain：DashScope audio adapter、Playground Audio TTS、`/v1/audio/speech`、`/v1/audio/transcriptions` 已完成并通过 `scripts/regression/dashscope-audio-chain.sh` 11 pass / 0 fail；真实百炼 ASR/TTS upstream smoke 仍需在确认模型权限和成本后执行，ASR 如要求公网文件 URL 则补对象存储中转
v0.4+：需要先建立功能卡片，再进入开发
```

当前最重要的要求：

```text
后续任何功能不能只说“完成了”。
必须同时记录：
做了什么
为什么这样做
通过什么技术做
改了哪些文件
新增哪些 API
新增哪些数据库表
如何验证
有什么限制
下一步怎么优化
```
