# Commercialization Gap Analysis and Targets - 2026-06-26

## Current Commercialization State

AI Aggregator now has a working foundation for:

- User registration/login and platform API keys.
- OpenAI-compatible `/v1` gateway for chat, embeddings, files, image/video async, and mock audio.
- Alibaba Cloud Bailian/DashScope provider integration for real Qwen chat and embeddings.
- Marketplace, pricing, benchmark score baseline, Smart Routing baseline, provider health, failover, request logs, billing, usage, workspace/project attribution, guardrails, workflows, files, admin management, and private/self-hosted foundation.
- Platform-level and scoped provider credentials baseline: Admin can store masked provider API keys, runtime routing uses DB credentials before env fallback, scoped BYOK selects workspace > user > platform credentials, and Admin can validate a specific provider key.

The project is not yet a fully commercial multi-provider AI aggregator because several enterprise monetization and production trust capabilities remain incomplete.

## P0 Commercial Gaps

| Gap | Business Impact | Current Status | Completion Target |
|---|---|---|---|
| Customer BYOK | Enterprise customers need to bring their own OpenAI/Grok/Anthropic/Bailian keys and isolate spend/security | Baseline completed: platform/user/workspace scoped provider credentials, masked list, workspace > user > platform runtime priority, Admin UI, user self-service BYOK page, provider credential validation API/UI, request log credential scope/key id annotation, regressions 19/0, 13/0, 9/0, and 17/0 | Future: real upstream smoke with valid keys |
| OpenAI/Grok/Anthropic provider onboarding | Aggregator value depends on multi-provider access, not only Bailian | Baseline completed: Admin provider template list/install API for OpenAI/Grok/Anthropic, model mappings, provider/model bindings, native Anthropic Messages adapter, provider credential validation, OpenAI/Grok provider-specific fake server validation, audit, regressions 19/0, 13/0, 9/0, and 12/0 | Future: real upstream smoke with valid keys, Anthropic tool-use/streaming refinements |
| Project cost center | Enterprise FinOps requires project/product/team chargeback below workspace | Baseline completed: `projects`, API key project binding, request/usage/billing `project_id`, Top Projects attribution, CSV export/filter, regression 16/0 | Future: project-level budget/quota, project RBAC, selector UX, invoice grouping |
| Smart Routing | Differentiated product value: optimize cost/latency/quality automatically | Baseline completed: routing policy schema, Admin Routing UI, priority/cost/latency/balanced ordering, request_logs latency/cost/error stats, regression 16/0 | Future: A/B experiment split, sticky bucketing, provider-level quality score, request_log routing_policy_id |
| Invoice / PO / Enterprise billing | B2B sales needs invoices, PO, postpaid terms, monthly statements | Baseline completed: postpaid organization terms, default PO, invoice draft generation from billing transactions, Admin Workspaces invoice panel, invoice CSV export, invoice PDF export, draft/issued/paid/void status workflow, audit, regressions 12/0 and 18/0 | Future: monthly statement automation, regional tax rules, project invoice grouping |

## P1 Production Trust Gaps

| Gap | Business Impact | Current Status | Completion Target |
|---|---|---|---|
| Webhook delivery worker | Workflow/agent customers need reliable callbacks | Baseline completed: async HTTP delivery, retrying/delivered/failed statuses, HMAC signature headers, response status/body, run detail metadata, regression 8/0 | Future: persistent queued worker, dead-letter/replay UI, per-workspace webhook secrets |
| Production secret management | Enterprise compliance requires KMS/Vault, rotation, audit | Baseline completed: provider/tool credential `seal_version`, provider key `last_used_at` / `last_used_scope`, `revoked_at` / `rotated_at` metadata, safe Admin list metadata, regression 19/0 | Future: KMS/Vault envelope provider, rotation workflow UI/API, credential usage audit stream |
| Real audio adapter | Multimodal marketplace completeness | Gateway/mock STT/TTS complete; DashScope audio adapter pending | DashScope ASR/TTS request mapping, model calibration, real smoke |
| Image/video Bailian calibration | Real media generation commercial readiness | Async gateway reaches upstream; model/request mapping needs calibration | Official model mapping, small real smoke, docs |
| Moderation and sensitive data routing | Compliance and regulated workloads | PII/prompt injection baseline complete; moderation/sensitive routing incomplete | Moderation provider, policy actions, safe route policy, regression |

## P2 Scale / Deployment Gaps

| Gap | Business Impact | Current Status | Completion Target |
|---|---|---|---|
| GPU replicas/resources | Private AI production deployment | Cluster/node/deployment registry exists | Replica scheduling, capacity metrics, autoscale policy |
| VPC/private endpoint | Enterprise deployment requirement | Helm foundation exists | Private endpoint config, network policy, deployment validation |
| Team invite email | Admin usability | Member role/status UI exists | Invite token/email flow, acceptance, audit |
| Prompt/workflow maturity | Agent product stickiness | Prompt templates, sessions, tools baseline exists | Versioning, workspace template library, memory summarization |

## Recommended Execution Order

1. Webhook worker and production secret management
   - Required for reliable workflow/agent usage and compliance.

## Definition of Done for Commercial BYOK

Commercial BYOK should not be considered complete until all items below are true:

- Users/admins can create provider credentials scoped to user, workspace, or platform.
- Raw secrets are never returned after creation.
- Secrets are masked in UI and sealed at rest.
- Runtime routing can select the correct credential based on workspace/user/provider/model policy.
- Admins can validate a selected provider key and see healthy/unhealthy status without exposing plaintext.
- OpenAI and Grok work through OpenAI-compatible adapters with configured base URLs.
- Anthropic has either a native adapter or a verified OpenAI-compatible proxy path.
- Request logs include credential scope/provider attribution without exposing secrets.
- Regression scripts cover create/list/revoke, route selection, no-secret leakage, and audit events.
- Documentation explains setup for OpenAI, Grok, Anthropic, and Bailian.

## Status After This Round

Completed in this round:

- Platform-level provider credential baseline.
- Admin Provider Credentials UI.
- DB-first provider key resolution for runtime router.
- Regression script: `scripts/regression/provider-credentials.sh`, 11 pass / 0 fail.
- Project Cost Center baseline.
- Admin Workspace Projects UI.
- API Key `project_id` binding and request/usage/billing project attribution.
- Workspace usage Top Projects and CSV `project_id` export/filter.
- Regression script: `scripts/regression/project-cost-centers.sh`, 16 pass / 0 fail.
- Smart Routing baseline.
- Routing policy schema and Admin Routing UI.
- Router policy strategies: `priority`, `cost`, `latency`, `balanced`.
- Regression script: `scripts/regression/smart-routing.sh`, 16 pass / 0 fail.
- Invoice/PO/postpaid baseline.
- Organization payment terms and default PO metadata.
- Invoice draft generation from billing transactions with subtotal/total/due date calculation.
- Admin Workspaces Invoices / PO panel and invoice create/list API.
- Regression script: `scripts/regression/invoices-po-postpaid.sh`, 12 pass / 0 fail.
- Scoped BYOK baseline.
- Platform/user/workspace provider credentials and workspace > user > platform runtime priority.
- Regression script: `scripts/regression/scoped-byok.sh`, 19 pass / 0 fail.
- Provider onboarding templates for OpenAI, Grok, and Anthropic.
- Regression script: `scripts/regression/provider-onboarding-templates.sh`, 19 pass / 0 fail.
- Anthropic native Messages adapter baseline.
- Regression script: `scripts/regression/anthropic-native-adapter.sh`, 13 pass / 0 fail.
- Secret management metadata baseline.
- Provider/tool credential seal metadata, last-used tracking, revoke metadata.
- Regression script: `scripts/regression/secret-management-metadata.sh`, 19 pass / 0 fail.
- Provider credential validation baseline.
- Admin validation API/UI for selected provider keys, with health history and audit persistence.
- Regression script: `scripts/regression/provider-credential-validation.sh`, 9 pass / 0 fail.
- Request log credential scope baseline.
- Non-secret `credential_scope` / `credential_key_id` persisted to request logs and exposed through API/CSV/UI.
- Regression script: `scripts/regression/request-log-credential-scope.sh`, 17 pass / 0 fail.
- OpenAI/Grok provider-specific fake server validation baseline.
- Regression script: `scripts/regression/openai-grok-provider-validation.sh`, 12 pass / 0 fail.
- User BYOK self-service baseline.
- Settings page provider key create/list/revoke for user-scoped credentials.
- Regression script: `scripts/regression/user-byok-self-service.sh`, 13 pass / 0 fail.
- Invoice status/export baseline.
- Invoice CSV export and draft/issued/paid/void workflow.
- Regression script: `scripts/regression/invoice-status-export.sh`, 15 pass / 0 fail.

Remaining after this round:

- Real upstream smoke for OpenAI/Grok/Anthropic/Bailian with valid provider keys.
- Anthropic tool-use mapping and native streaming refinement.
- Project-level budget/quota, project RBAC, and invoice grouping.
- Monthly statement automation and regional tax rules.
- Smart Routing A/B experiments, sticky bucketing, provider-level quality score, and request log policy annotation.
- KMS/Vault envelope provider, credential rotation UI/API, and credential usage audit stream.
