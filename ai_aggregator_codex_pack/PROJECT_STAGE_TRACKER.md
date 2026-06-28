# AI Aggregator Platform 项目阶段跟踪文档

文档名称：`PROJECT_STAGE_TRACKER.md`  
项目名称：AI Model Aggregator / AI Gateway / AI Aggregator Platform  
当前版本状态：`v0.1-v1.0 foundation 已完成基础闭环；当前持续补齐生产增强、provider/model 对接、前端管理面、回归脚本和文档`  
最后更新日期：2026-06-27  
维护人：Project Owner / Codex / QoderWork

---

## 1. 项目总览

### 1.1 项目定位

本项目最初是一个 Thin AI Aggregator MVP，核心目标是实现：

```text
用户注册/登录
→ 创建 API Key
→ 查看模型列表
→ 调用 /v1/chat/completions
→ 路由到 Provider
→ 返回模型结果
→ 记录 usage
→ 扣减 balance
→ 写入 billing transaction
→ 在 Dashboard / Billing 中展示
```

当前项目方向已经升级为完整的：

```text
AI Aggregator Platform
= AI Gateway
+ Model Marketplace
+ Enterprise Control Plane
+ AI FinOps
+ Smart Routing
+ Reliability / SLA Layer
+ Evaluation / Benchmark
+ Guardrails / Compliance
+ BYOK / BYOM
+ Workflow / Agent API
+ Private / Sovereign AI
+ White-label / OEM
```

---

## 2. 当前整体阶段状态

| 阶段 | 名称 | 当前状态 | 完成度 | 说明 |
|---|---|---:|---:|---|
| v0.1 | Runnable Thin Aggregator MVP | 已完成 | 90%+ | 已经可以运行，完成最小调用与计费闭环 |
| v0.2 | Reliable AI Gateway | 已完成 | 92%+ | Request Logs、Provider Health、Provider Status Page、provider 24h request/error/fallback stats、fallback_logs、fallback smoke、Admin CRUD、Pricing Update 已接入并通过 smoke/API 验证 |
| v0.3 | Enterprise Control Plane + FinOps | 已完成 | 96%+ | Organization / Workspace、API Key workspace/project binding、workspace usage/cost attribution Top model/provider/user/project breakdown、Project Cost Centers、Invoice/PO/postpaid baseline、Invoice CSV export/status workflow、Budget/Quota runtime enforcement、Admin Credit Grant、Audit Logs、Playground embeddings/async task UI 基础闭环已完成；Admin Workspaces 浏览器 E2E 已验证组织/工作区/budget/quota/member/export，Project Cost Centers regression 16/0，Invoice regressions 12/0 + 15/0 |
| v0.4 | Model Marketplace | 已完成 | 89%+ | Marketplace API、模型搜索/筛选、模型详情、模型比较、provider 可用性、推荐场景、pricing history / price-change audit baseline 已完成；Marketplace 与 pricing history 已固化为 regression script |
| v0.5 | Evaluation / Benchmark / Optimization | 已完成 | 82%+ | Benchmark task/run/result schema、Admin Benchmark API、本地确定性评分、Marketplace benchmark_score 回填已完成；Smart Routing baseline 已完成 priority/cost/latency/balanced policy 和 Admin Routing UI，回归 16/0；A/B Test、provider-level quality score 与真实 LLM Judge 后续增强 |
| v0.6 | Guardrails / Compliance | 已完成基础闭环 | 73%+ | Guardrail policy、PII detection/masking/block、prompt injection block、guardrail result、policy violation audit 已完成；PII/injection 已固化为 regression script；retention/export 后续增强 |
| v0.7 | Workflow / Agent API | 已完成基础闭环 | 94%+ | Workflow/steps/run/run steps、Tool Registry、Tool Credentials baseline、Prompt Templates baseline、Agent Sessions baseline、Agent Trace、同步 workflow runner、用户侧 Workflows UI、workflow run 成本记录、signed webhook delivery worker 已完成；Workflow、webhook、webhook worker、tool credentials、prompt templates、agent sessions 已固化为 regression script |
| v1.0 | Private / Sovereign AI + Self-hosted Inference | 已完成基础闭环 | 87%+ | Inference Cluster/Node/Model Deployment、自托管 provider 注册、统一路由调用、自托管模型 health/capacity 基础表、用户 Files UI、Scoped BYOK、User BYOK Self-Service、OpenAI/Grok/Anthropic provider onboarding templates、Anthropic native adapter、Secret Management Metadata、Provider Credential Validation、Request Log Credential Scope、OpenAI/Grok Provider Validation 已完成；Admin foundation/BYOK/provider onboarding/Anthropic/secret metadata/provider credential validation/request log credential scope/OpenAI-Grok validation/user BYOK self-service regression 已固化 |

---

## 3. v0.1 — Runnable Thin Aggregator MVP

### 3.1 阶段目标

v0.1 的目标是证明 AI Aggregator 的最小商业闭环：

```text
用户可以注册、拿 API Key、调用模型、产生用量、扣费，并在控制台看到结果。
```

### 3.2 已实现功能

| 模块 | 功能 | 状态 | 说明 |
|---|---|---:|---|
| 用户系统 | 用户注册 | 已完成 | 支持创建用户；重复注册返回稳定 409 conflict，不暴露数据库 constraint 细节，已由 `scripts/regression/auth-conflict.sh` 覆盖 |
| 用户系统 | 用户登录 | 已完成 | 支持 JWT 登录 |
| 用户系统 | 密码 bcrypt hash | 已完成 | 密码不明文存储 |
| 用户系统 | JWT localStorage | 已完成 | 前端可保持登录状态 |
| API Key | 创建 API Key | 已完成 | 生成 `sk-aag-xxx` |
| API Key | hash 存储 | 已完成 | 明文只展示一次 |
| API Key | list keys | 已完成 | 控制台可查看 key 列表 |
| API Key | revoke key | 已完成 | 支持删除/禁用 key |
| API Key | current key | 已完成 | Playground 可使用当前 key |
| Gateway API | `GET /v1/models` | 已完成 | 返回 OpenAI-compatible model list |
| Gateway API | `POST /v1/chat/completions` | 已完成 | 支持非流式调用 |
| Gateway API | stream 基础支持 | 已完成 | 支持基础 SSE |
| Provider | Mock Provider | 已完成 | 用于本地测试 |
| Provider | DashScope Provider | 已完成 | 支持百炼/DashScope 基础调用 |
| Routing | priority routing | 已完成 | 按模型 provider priority 选择 |
| Billing | 用户余额 | 已完成 | 注册后有测试 credit |
| Billing | usage charge | 已完成 | 调用后扣费 |
| Billing | billing transaction | 已完成 | 写入交易记录 |
| Billing | DB pricing | 已完成 | 优先读取模型价格 |
| Usage | usage logs | 已完成 | 记录 tokens、cost、latency |
| Rate Limit | Redis RPM 限流 | 已完成 | 基础 fixed window |
| Frontend | Login 页面 | 已完成 | 注册/登录 |
| Frontend | Dashboard 页面 | 已完成 | 展示 usage summary |
| Frontend | API Keys 页面 | 已完成 | 创建/复制/revoke key |
| Frontend | Models 页面 | 已完成 | 展示模型 |
| Frontend | Playground 页面 | 已完成 | 调用 chat completion |
| Frontend | Billing 页面 | 已完成 | 展示余额和交易 |
| DevOps | Docker Compose | 已完成 | postgres/redis/backend/frontend |
| DevOps | `.env.example` | 已完成 | 支持本地配置 |
| Testing | smoke-test | 已完成 | 验证主链路 |

### 3.3 v0.1 验收标准

| 验收项 | 状态 |
|---|---:|
| `docker compose up --build` 可启动 | 已完成 |
| 前端可访问 | 已完成 |
| 用户可注册登录 | 已完成 |
| 用户可创建 API Key | 已完成 |
| `/v1/models` 返回模型列表 | 已完成 |
| `/v1/chat/completions` 可调用 | 已完成 |
| Mock Provider 模式可运行 | 已完成 |
| DashScope Provider 可配置 | 已完成 |
| usage_logs 有记录 | 已完成 |
| balance 会扣减 | 已完成 |
| billing_transactions 有记录 | 已完成 |
| Dashboard/Billing 可展示变化 | 已完成 |
| smoke-test 主链路通过 | 已完成 |

### 3.4 v0.1 未包含内容

以下内容不属于 v0.1 范围：

```text
Provider Health Check
完整 Provider Failover
Fallback Logs
Provider Status Page
Admin Model CRUD
Admin Provider CRUD
Organization / Workspace
RBAC
Quota / Budget
BYOK
Marketplace
Benchmark
Guardrails
Workflow / Agent
Self-hosted Inference
Private / Sovereign AI
```

### 3.5 v0.1 当前结论

```text
v0.1 已达到 Runnable MVP。
```

---

## 4. v0.2 — Reliable AI Gateway

### 4.1 阶段目标

v0.2 的目标是把 v0.1 从“能跑”升级为“稳定可运营”。

核心目标：

```text
每个请求可追踪
Provider 状态可见
Provider 失败可 fallback
Admin 可以管理模型和 Provider
错误格式统一
Smoke Test 可以验证 fallback
```

### 4.2 v0.2 目标功能列表

| 模块 | 功能 | 当前状态 | 完成度 | 说明 |
|---|---|---:|---:|---|
| Observability | request_logs 表 | 已完成 | 100% | migration 已新增 |
| Observability | 写入 request_logs | 已完成 | 80% | 成功/失败路径已写入 |
| Observability | 查询 request logs API | 已完成 | 80% | `/api/user/request-logs` 已有 |
| Observability | Request Logs 前端页面 | 已完成 | 80% | 页面已实现 |
| Observability | request detail | 已完成 | 70% | 可查看 preview/detail |
| Error | 统一 error format | 部分完成 | 70% | 已有标准结构 |
| Trace | request_id 全链路 | 部分完成 | 70% | 已有 request ID middleware |
| Routing | 基础 failover chain | 部分完成 | 60% | 会尝试多个 provider |
| Routing | fallback_count 记录 | 部分完成 | 60% | 写入 request_logs |
| Routing | fallback_logs 表 | 已完成 | 90% | provider 调用失败后 fallback 会写入 transition |
| Provider Health | health loop | 已完成 | 85% | router 启动和定时 health check，跳过 synthetic mock |
| Provider Health | provider_health_checks 表 | 已完成 | 90% | health loop 和手动检查均写入历史 |
| Provider Health | Provider Health API | 已完成 | 92% | `GET /api/admin/provider-health` 已实现，并返回 24h request/error/fallback 聚合 |
| Frontend | Provider Status Page | 基本完成 | 85% | Admin Provider Status Page 已接真实 API，并展示 24h requests/error rate/fallbacks |
| Admin | Admin Models CRUD | 已完成 | 90% | 后端 CRUD + Admin Models UI 已接真实 API |
| Admin | Admin Providers CRUD | 已完成 | 85% | 后端 create/list/update 已完成 |
| Admin | Model-Provider Mapping CRUD | 已完成 | 85% | 后端 bind/update/unbind 已完成 |
| Testing | fallback smoke-test | 已完成 | 90% | `RUN_FALLBACK_SMOKE=true` 可验证 mock failure fallback case |
| Mock | Mock Provider failure mode | 已完成 | 85% | `MOCK_FAIL_PROVIDER_IDS` 可强制 chat/stream 失败；health failure 单独配置 |

### 4.3 v0.2 已完成的新变更

#### 4.3.1 新增文档与 Codex Pack

已新增：

```text
ai_aggregator_codex_pack/
├── 00_README_FOR_CODEX.md
├── 01_MASTER_CODEX_PROMPT.md
├── 02_PRODUCT_REQUIREMENTS_PDR.md
├── 03_SCOPE_AND_VERSIONING.md
├── 04_ARCHITECTURE.md
├── 05_BUSINESS_MODEL_TO_MODULE_MAPPING.md
├── 06_ROADMAP_AND_SPRINTS.md
├── 07_MODULE_BREAKDOWN.md
├── 08_DATA_MODEL.md
├── 09_API_DESIGN.md
├── 10_CODEX_SPRINT_PROMPTS.md
├── 11_ACCEPTANCE_AND_REGRESSION.md
├── 12_MODULE_LEVEL_PROMPTS.md
├── 13_SECURITY_AND_ENGINEERING_GUARDRAILS.md
└── 14_UI_PAGE_SPEC.md
```

目标：

```text
让 Codex 能按照 PDR / Scope / Architecture / Sprint 继续开发。
```

状态：

```text
已完成，后续需要持续同步更新。
```

#### 4.3.2 新增 v0.2 数据库 migration

新增：

```text
migrations/003_v02_observability.sql
```

包含：

```text
users.is_admin
request_logs
provider_health_checks
fallback_logs
```

状态：

```text
request_logs 已接入主链路；
provider_health_checks 和 fallback_logs 已接入业务写入；Admin CRUD 仍待实现。
```

#### 4.3.3 新增 Request Logs 能力

新增后端能力：

```text
RecordRequestLog()
ListRequestLogs()
GetRequestLogByRequestID()
```

新增 API：

```text
GET /api/user/request-logs
GET /api/user/request-logs/:request_id
```

新增前端页面：

```text
/dashboard/request-logs
```

已支持查看：

```text
request_id
model
provider
final_provider
status
latency
input_tokens
output_tokens
charged_cost
request_preview
response_preview
error_message
created_at
```

状态：

```text
基本完成，是 v0.2 目前最完整的新功能。
```

#### 4.3.4 新增基础 Provider Failover

当前已支持：

```text
RouteAll(model)
→ 获取多个 provider route
→ 逐个尝试调用 provider
→ 某个 provider 成功则返回
→ 记录 final_provider 和 fallback_count
```

状态：

```text
基础完成，fallback_logs 已写入并通过手动 fallback 验证；仍需把 fallback case 固化到 smoke-test 脚本。
```

#### 4.3.5 新增基础 Provider Health Loop

当前已支持：

```text
定时调用 provider.HealthCheck()
更新 model_providers.health_status
更新 model_providers.last_health_chk
```

状态：

```text
基本完成。
已写入 provider_health_checks 表，并提供 Admin API 和 Provider Status 页面。
```

### 4.4 v0.2 剩余任务

#### P0：必须完成，才能认为 v0.2 完成

| 任务 | 说明 | 状态 |
|---|---|---:|
| 写入 fallback_logs | 每次 fallback 失败/成功都记录 | 已完成 |
| 写入 provider_health_checks | 每次 health check 记录历史 | 已完成 |
| `GET /api/admin/provider-health` | 获取 provider health list | 已完成 |
| `POST /api/admin/providers/:id/health-check` | 手动触发 health check | 已完成 |
| Provider Status Page 接真实 API | 不再使用静态数据 | 基本完成 |
| Mock Provider failure mode | 支持模拟主 provider 失败 | 已完成 |
| fallback smoke-test | 验证主 provider fail，次 provider success | 已完成，脚本已固化为可选项 |
| Admin Models CRUD | 管理模型 | 已完成 |
| Admin Providers CRUD | 管理 provider | 已完成 |
| Admin Model-Provider Mapping CRUD | 管理模型到 provider 映射 | 已完成 |
| Admin Pricing Update | 修改价格后下一次调用生效 | 已完成 |

#### P1：v0.2 增强项

| 任务 | 说明 | 状态 |
|---|---|---:|
| Request Logs 分页 | 支持 limit/offset | 已完成 |
| Request Logs 过滤 | model/status/provider/date | 已完成 |
| Request Logs 导出 | CSV | 已完成 |
| Dashboard error rate | 错误率统计 | 已完成 |
| Dashboard p95/p99 latency | 延迟统计 | 已完成 |
| Provider health history | 最近 N 次健康检查 | 已完成 |
| Admin Credit Grant | 管理员给用户充值 | 已完成 |
| Error code 文档 | 标准错误码文档 | 已完成 |

### 4.5 v0.2 完成标准

只有当以下全部完成，才能标记为：

```text
v0.2 Complete
```

| 验收项 | 当前状态 |
|---|---:|
| 每个 chat request 都写 request_logs | 基本完成 |
| 错误请求也写 request_logs | 基本完成 |
| 用户可查看 request logs | 已完成 |
| provider health check 有历史记录 | 已完成 |
| Provider Status Page 接真实数据 | 基本完成 |
| provider failover 有 fallback_logs | 已完成 |
| smoke-test 验证 fallback | 已完成，需显式开启 `RUN_FALLBACK_SMOKE=true` |
| Admin 可管理 model | 已完成 |
| Admin 可管理 provider | 已完成 |
| Admin 可管理 model-provider mapping | 已完成 |
| Admin 修改价格后计费生效 | 已完成 |
| Admin 禁用 provider 后 router 不再选择 | 已完成 |
| go test / docker compose / smoke-test 不回归 | 已完成 |

### 4.6 v0.2 当前结论

```text
v0.2 已完成。
当前已完成 Request Logs、Provider Health 历史/API、Provider Status Page、fallback_logs、脚本化 fallback smoke-test、Admin CRUD、Pricing Update 和禁用 provider/binding 后路由刷新。
```

建议状态标记：

```text
v0.2-complete
```

---

## 5. v0.3 — Enterprise Control Plane + FinOps

### 5.1 阶段目标

将平台从个人开发者工具升级成企业 AI Control Plane。

目标：

```text
企业可以管理组织、团队、Workspace、权限、预算、Quota、审计和成本归因。
```

### 5.2 计划功能

| 模块 | 功能 | 状态 |
|---|---|---:|
| Organization | organizations 表 + Admin API | 已完成 |
| Workspace | workspaces 表 + Admin API + Admin UI | 已完成 |
| Team | workspace_members | API 已完成；Admin Workspaces UI 已支持用户下拉选择、role/status upsert；邮件邀请流程未开始 |
| RBAC | roles / permissions | roles schema 已完成；admin/user 基础授权已接入；API Key workspace membership + api_keys:create + files:write + workflows:write + workspace_budgets:write + workspace_quotas:write permission enforcement 已完成；更多 admin-delegated permission 点待扩展 |
| API Key Scope | workspace 归属绑定 + active member + api_keys:create/files:write/workflows:write enforcement 已完成；模型 scope 已有基础 enforcement；budget/quota 用户侧写权限已接入 |
| Quota | workspace quota | requests_per_minute / tokens_per_month / spend_per_month runtime enforcement 已完成 |
| Budget | monthly budget | workspace monthly hard budget runtime enforcement 已完成 |
| Audit | audit_logs | Admin budget/quota/top-up 基础写入和查询已完成 |
| BYOK | provider_credentials | platform/user/workspace scoped BYOK、user self-service page、provider credential validation、request log credential scope annotation 已完成；真实 upstream smoke 待有效 provider key |
| FinOps | cost by user/workspace/project | workspace request/usage/billing 归因写入、usage summary、Top model/provider/user 成本归因已完成；project 维度未开始 |
| Export | usage export | workspace usage/cost CSV export 已完成 |

### 5.3 v0.3 验收标准

| 验收项 | 状态 |
|---|---:|
| 可创建 organization | 已完成 |
| 可创建 workspace | 已完成 |
| 可邀请成员 | 可添加/更新成员 role/status，邀请邮件流程未开始 |
| 可配置角色权限 | 已完成 api_keys:create、files:write、workflows:write、workspace_budgets:write、workspace_quotas:write 细粒度 enforcement；更多 admin-delegated permission 点待扩展 |
| API Key 可绑定 workspace | 已完成 |
| 可设置 budget/quota | 已完成 Admin API + Admin Workspaces UI |
| 超 budget/quota 可拦截 | 已完成 |
| 可查看 workspace usage | 已完成 summary API，request/usage/billing 归因已写入，Admin UI 已展示 Top model/provider/user 成本归因 |
| 可导出 usage/cost | 已完成 workspace usage/cost CSV export |
| 关键操作写 audit_logs | 已完成基础写入和查询 |

---

## 6. v0.4 — Model Marketplace

### 6.1 阶段目标

从 Gateway 变成模型市场，让用户可以发现、比较、选择模型。

### 6.2 计划功能

| 模块 | 功能 | 状态 |
|---|---|---:|
| Model Catalog | 模型市场页升级 | 已完成 |
| Model Detail | 模型详情页 | 已完成 |
| Tags | tags / capabilities | 已完成 |
| Capabilities | capabilities | 已完成 |
| Compare | 模型对比 | 已完成 |
| Pricing | pricing history | 已完成 baseline；未来价格计划、回滚、provider multiplier history 待增强 |
| Availability | provider availability | 已完成 |
| Ranking | model ranking | 已完成 |
| Use Cases | recommended use cases | 已完成 |
| Search | 模型搜索 / 筛选 | 已完成 |

### 6.3 v0.4 验收标准

| 验收项 | 状态 |
|---|---:|
| 用户可查看模型详情 | 已完成 |
| 用户可按能力标签筛选 | 已完成 |
| 用户可比较模型价格/上下文/能力 | 已完成 |
| 用户可查看 provider 可用性 | 已完成 |
| 模型详情展示 benchmark score | 未开始 |
| 模型详情展示推荐场景 | 已完成 |

---

## 7. v0.5 — Evaluation / Benchmark / Optimization

### 7.1 阶段目标

将平台升级成模型选型和路由优化平台。

### 7.2 计划功能

| 模块 | 功能 | 状态 |
|---|---|---:|
| Benchmark | benchmark_tasks | 已完成 |
| Benchmark | benchmark_runs | 已完成 |
| Results | benchmark_results | 已完成 |
| Admin UI | Benchmarks 管理页 | 已完成 |
| Eval Dataset | eval_datasets | 未开始 |
| LLM Judge | quality_evaluations | 未开始 |
| A/B Test | ab_tests | 未开始 |
| Routing Experiments | routing_experiments | 未开始 |
| Quality Score | 模型质量评分 | 已完成 |
| Cost Score | 成本评分 | 已完成 |
| Latency Score | 延迟评分 | 已完成 |
| Smart Routing | 按质量/成本/延迟自动路由 | 已完成基础闭环 |

### 7.3 v0.5 验收标准

| 验收项 | 状态 |
|---|---:|
| 可创建 benchmark task | 已完成 + Admin UI |
| 可运行 benchmark | 已完成 + Admin UI |
| 可查看 benchmark result | 已完成 + Admin UI |
| 可对比模型质量/成本/延迟 | 已完成 |
| 可创建 routing policy | 已完成 + Admin Routing UI |
| 可用 request latency/cost/error stats 影响 routing | 已完成：priority/cost/latency/balanced policy 已接入 router；`scripts/regression/smart-routing.sh` 验证低延迟 provider 被优先选择 |
| 可创建 routing experiment | 未开始：A/B split/sticky bucketing 后续增强 |
| 可用 benchmark score 影响 routing | 部分完成：Marketplace 已展示 latest benchmark_score；provider-level quality score 后续接入 routing |

---

## 8. v0.6 — Guardrails / Compliance

### 8.1 阶段目标

支持企业安全、合规、审计和 AI 风险治理。

### 8.2 计划功能

| 模块 | 功能 | 状态 |
|---|---|---:|
| PII | PII detection | 已完成 |
| PII | PII masking | 已完成 |
| Security | prompt injection detection | 已完成 |
| Security | jailbreak detection | 已完成 |
| Moderation | content moderation | 未开始 |
| Policy | guardrail_policies | 已完成 |
| Result | guardrail_results | 已完成 |
| Admin UI | Guardrails 管理页 | 已完成 |
| Audit | policy violations | 已完成 |
| Retention | file retention cleanup | 已完成基础闭环 |
| Routing | sensitive data routing | 未开始 |

### 8.3 v0.6 验收标准

| 验收项 | 状态 |
|---|---:|
| 可配置 guardrail policy | 已完成 + Admin UI |
| 请求可触发 PII 检测 | 已完成 |
| 可对 PII 做 masking | 已完成 |
| 可阻断 policy violation | 已完成 |
| 可查看 guardrail result | 已完成 + Admin UI |
| 可配置日志留存周期 | 已完成基础 audit_log_retention_days + manual cleanup |
| 可导出合规审计记录 | 已完成 Audit Log CSV export + filters |

---

## 9. v0.7 — Workflow / Agent API

### 9.1 阶段目标

从卖模型调用升级为卖任务型 AI 能力。

### 9.2 计划功能

| 模块 | 功能 | 状态 |
|---|---|---:|
| Workflow | workflows | 已完成 |
| Workflow | workflow_steps | 已完成 |
| Workflow Run | workflow_runs | 已完成 |
| Step Run | workflow_run_steps | 已完成 |
| Tools | tools | 已完成 |
| Credentials | tool_credentials | seal_version / revoked_at / rotated_at metadata baseline 已完成；KMS/Vault envelope、rotation UI/API、usage audit stream 待增强 |
| Prompt | prompt_templates | 已完成 baseline；变量 schema、版本管理、workspace template library 待增强 |
| Agent | agent_sessions | 已完成 baseline；session memory/context summarization 待增强 |
| Trace | agent_traces | 已完成 |
| Webhook | webhook_deliveries | 已完成 baseline |
| Billing | task-level billing | 已完成基础 cost 记录 |

### 9.3 v0.7 验收标准

| 验收项 | 状态 |
|---|---:|
| 可创建 workflow | 已完成 |
| 可配置 workflow steps | 已完成 |
| 可运行 workflow | 已完成 |
| 可查看 workflow run history | 已完成 |
| 用户侧 workflow 创建/运行 UI | 已完成 |
| 可接入 tool | 已完成 |
| 可记录 agent trace | 已完成 |
| 可按 workflow run 计费 | 已完成基础 cost 记录 |
| 可配置 webhook callback | 已完成 baseline；真实 delivery worker/retry/signature 待增强 |

---

## 10. v1.0 — Private / Sovereign AI + Self-hosted Inference

### 10.1 阶段目标

支持私有化、本地化、主权 AI 和自托管模型服务。

### 10.2 计划功能

| 模块 | 功能 | 状态 |
|---|---|---:|
| Deployment | private deployment package | 已完成基础 Helm package |
| Kubernetes | Helm chart | 已完成基础 chart |
| Self-hosted | vLLM provider | 已完成基础 OpenAI-compatible 接入 |
| Self-hosted | SGLang provider | 已完成基础 OpenAI-compatible 接入 |
| Cluster | inference_clusters | 已完成 + Admin UI |
| Nodes | inference_nodes | 已完成 + Admin UI |
| Deployment | model_deployments | 已完成 + Admin UI |
| Replicas | deployment_replicas | 未开始 |
| GPU | gpu_resources | 未开始 |
| Capacity | capacity_metrics | 已完成基础表 |
| Network | VPC / private endpoint | 未开始 |
| Sovereign | local logs / local data | 已完成基础本地表与私有 network_mode |

### 10.3 v1.0 验收标准

| 验收项 | 状态 |
|---|---:|
| 可通过 Helm 部署 | 已完成基础 chart，待集群实测 |
| 可注册 self-hosted inference endpoint | 已完成 |
| 可注册 GPU node | 已完成 |
| 可部署开源模型 | 已完成基础 deployment registry |
| 可通过统一 API 调自托管模型 | 已完成 |
| 可查看模型实例健康状态 | 已完成基础 status 字段 |
| 可查看 GPU capacity | 已完成基础 capacity_metrics 表 |
| 支持本地日志和数据留存 | 已完成基础本地数据表 |

---

## 11. 跨阶段核心模块跟踪

### 11.1 Gateway Core

| 功能 | 阶段 | 状态 |
|---|---|---:|
| `/v1/models` | v0.1 | 已完成 |
| `/v1/chat/completions` | v0.1 | 已完成 |
| stream chat | v0.1 | 基础完成 |
| unified error format | v0.2 | 部分完成 |
| request validation | v0.2 | 部分完成 |
| `/v1/embeddings` | v0.3 | 已完成基础 API + usage/request/billing 记录 |
| image/video async API | v0.3 | 已完成 submit + poll + async task worker；smoke-test 已增加可选覆盖 |
| audio API | v0.7+ | Gateway/mock STT/TTS 已由 `scripts/regression/audio-mock.sh` 覆盖；DashScope audio adapter 未实现 |

### 11.2 Provider System

| 功能 | 阶段 | 状态 |
|---|---|---:|
| Mock Provider | v0.1 | 已完成 |
| DashScope Provider | v0.1 | 已完成 |
| Priority routing | v0.1 | 已完成 |
| Health check loop | v0.2 | 部分完成 |
| Failover chain | v0.2 | 部分完成 |
| Fallback logs | v0.2 | 已完成 |
| Provider Status API | v0.2 | 已完成 |
| OpenAI-compatible Provider | v0.4/v0.5 | platform provider + credential baseline 已完成；OpenAI/Grok provider template 和真实 smoke 待增强 |
| BYOK Provider | v0.3+ | platform/user/workspace scoped BYOK、user self-service、provider credential validation、credential-scope request log annotation 已完成；真实 upstream smoke 待有效 provider key |
| Self-hosted Provider | v1.0 | 未开始 |

### 11.3 Billing / FinOps

| 功能 | 阶段 | 状态 |
|---|---|---:|
| balance | v0.1 | 已完成 |
| credit grant | v0.1 | 已完成 |
| usage charge | v0.1 | 已完成 |
| billing_transactions | v0.1 | 已完成 |
| DB pricing | v0.1 | 已完成 |
| upstream_cost_usd | v0.2 | 部分完成 |
| gross_margin_usd | v0.2 | 部分完成 |
| admin grant credit | v0.2/v0.3 | 已完成 |
| quota | v0.3 | 已完成基础 enforcement |
| budget | v0.3 | 已完成基础 enforcement |
| workspace cost | v0.3 | 已完成基础归因、Top model/provider/user breakdown 和 Budget/Quota enforcement |
| workspace usage export | v0.3 | 已完成 CSV export |
| invoice / PO | v0.3+ | 已完成基础闭环：postpaid terms、default PO、invoice draft/list、subtotal/total/due date、CSV export、PDF export、status workflow、audit；monthly statement/regional tax rules 后续增强 |

### 11.4 Observability

| 功能 | 阶段 | 状态 |
|---|---|---:|
| usage_logs | v0.1 | 已完成 |
| dashboard usage summary | v0.1 | 已完成 |
| request_logs | v0.2 | 基本完成 |
| request detail page | v0.2 | 基本完成 |
| provider_health_checks | v0.2 | 已完成 |
| fallback_logs | v0.2 | 已完成 |
| error rate | v0.2 | 已完成 |
| p95 / p99 latency | v0.2 | 已完成 |
| provider status | v0.2 | 基本完成 |
| audit logs | v0.3/v1.0 | 已完成基础查询 + Admin UI + CSV export |

### 11.5 Admin Console

| 功能 | 阶段 | 状态 |
|---|---|---:|
| Admin route shell | v0.1/v0.2 | 已完成 |
| users.is_admin | v0.2 | 表字段已加 |
| Admin Models CRUD | v0.2 | 已完成 |
| Admin Providers CRUD | v0.2 | 已完成 |
| Admin Model-Provider CRUD | v0.2 | 已完成 |
| Pricing Management | v0.2 | 已完成 |
| Credit Grant | v0.2/v0.3 | 已完成 |
| User Management | v0.3 | 已完成基础 Admin Users UI/API |
| Audit Logs | v0.3/v1.0 | 已完成列表 + CSV export |

---

## 12. 当前优先级 Backlog

### 12.1 立即优先级 P0

| 编号 | 任务 | 所属阶段 | 状态 |
|---|---|---|---:|
| P0-001 | 写入 fallback_logs | v0.2 | 已完成 |
| P0-002 | 写入 provider_health_checks | v0.2 | 已完成 |
| P0-003 | 实现 Provider Health API | v0.2 | 已完成 |
| P0-004 | Provider Status Page 接真实 API | v0.2 | 基本完成 |
| P0-005 | Mock Provider failure mode | v0.2 | 已完成 |
| P0-006 | fallback smoke-test case | v0.2 | 已完成，脚本已固化 |
| P0-007 | Admin Models CRUD | v0.2 | 已完成 |
| P0-008 | Admin Providers CRUD | v0.2 | 已完成 |
| P0-009 | Admin Model-Provider Mapping CRUD | v0.2 | 已完成 |
| P0-010 | Admin Pricing Update | v0.2 | 已完成 |

### 12.2 次优先级 P1

| 编号 | 任务 | 所属阶段 | 状态 |
|---|---|---|---:|
| P1-001 | Request Logs pagination/filter/export 完善 | v0.2 | 已完成 |
| P1-002 | Dashboard p95/p99 latency | v0.2 | 已完成 |
| P1-003 | Error code 文档 | v0.2 | 已完成 |
| P1-004 | Admin Credit Grant | v0.2/v0.3 | 已完成 |
| P1-005 | Provider health history chart | v0.2 | 已完成 |
| P1-006 | Dashboard error rate | v0.2 | 已完成 |
| P1-007 | Clean node_modules / zip noise | Engineering | 待检查 |
| P1-008 | go test / frontend build / smoke-test 回归 | Engineering | 持续 |

### 12.3 后续 P2

| 编号 | 任务 | 所属阶段 | 状态 |
|---|---|---|---:|
| P2-001 | Organization / Workspace | v0.3 | Admin API + Admin UI Foundation + API Key workspace binding + cost attribution breakdown 已完成 |
| P2-002 | RBAC | v0.3 | Roles schema 已完成，API Key workspace membership + api_keys:create/files:write/workflows:write permission enforcement 已完成，更多 permission 点待扩展 |
| P2-003 | Budget / Quota / Export | v0.3 | Budget/Quota enforcement + workspace usage/cost CSV export 已完成 |
| P2-004 | Model Marketplace | v0.4 | 未开始 |
| P2-005 | Benchmark Runner | v0.5 | 后端 foundation + Admin Benchmarks UI 基础闭环已完成，真实 LLM Judge/A-B Test 待增强 |
| P2-006 | Guardrails | v0.6 | 后端 foundation + Admin Guardrails UI 基础闭环已完成，完整 moderation/合规导出待增强 |
| P2-007 | Workflow Engine | v0.7 | 后端 foundation + 用户 Workflows UI + webhook delivery audit baseline + tool credentials baseline + agent sessions baseline 已完成，真实 webhook delivery worker/生产 KMS/session memory 待增强 |
| P2-008 | Self-hosted Inference | v1.0 | 后端 foundation + Admin Inference UI 基础闭环已完成，GPU 调度/生产集群实测待增强 |

---

## 13. 每次开发后的更新规范

每完成一个新功能，必须更新本文件中的以下位置：

```text
1. 当前整体阶段状态
2. 对应版本的功能表
3. 跨阶段核心模块跟踪
4. 当前优先级 Backlog
5. 最近变更记录
6. 验收标准状态
```

每次更新必须记录：

```text
变更日期
开发工具：Codex / QoderWork / Manual
变更模块
新增 API
新增数据库表/字段
新增前端页面
新增测试
是否影响 v0.1 回归
是否通过 smoke-test
```

---

## 14. 最近变更记录

### 2026-06-23 — Admin Control Plane 补全与本机验证

#### 新增

```text
backend/internal/auth/middleware.go
  ValidateJWT() / JWTClaims

backend/internal/storage/store.go
  AdminUser / SystemSetting / AnalyticsOverview / AnalyticsPoint / AlertSummary
  ListAdminUsers()
  GetAdminUser()
  UpdateUserProfile()
  UpdateAdminUser()
  GetAdminUserUsage()
  ListSettings()
  SetSettingAdmin()
  AdminAnalyticsOverview()
  AdminAnalyticsSeries()
  ListAlertSummaries()

backend/internal/gateway/router.go
  POST /api/user/auth/refresh
  GET /api/user/profile
  PUT /api/user/profile
  GET /api/admin/users
  POST /api/admin/users
  GET /api/admin/users/:id
  PUT /api/admin/users/:id
  GET /api/admin/users/:id/usage
  GET /api/admin/analytics/overview
  GET /api/admin/analytics/usage
  GET /api/admin/analytics/cost
  GET /api/admin/analytics/latency
  GET /api/admin/analytics/errors
  GET /api/admin/settings
  PUT /api/admin/settings/:key
  GET /api/admin/alerts/rules
  GET /api/admin/alerts/history
  POST /v1/audio/transcriptions
  POST /v1/audio/speech

frontend/src/lib/api.ts
  Admin users / analytics / settings / alerts / audit API client

frontend/src/pages/Admin.tsx
  Users page
  API Keys overview page
  Analytics dashboard
  Alerts page
  Settings editor
  Audit Log page

backend/Dockerfile
  使用已提交 go.mod/go.sum 构建，移除 Docker build 阶段 go mod tidy
```

#### 完成

```text
Admin Users / Credit Grant 基础闭环完成
Admin Analytics 基础闭环完成
Admin Settings 读写闭环完成
Admin Alerts 当前状态视图完成
Admin Audit Log 前端视图完成
User Profile / Token Refresh 完成
Docker backend build 稳定性修复完成
Audio transcription / speech 网关层完成；mock provider routing、request_logs、usage_logs 已回归覆盖；真实 DashScope adapter 待映射
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
API smoke:
  GET /health = 200
  POST /api/user/auth/login = 200
  GET /api/user/profile = 200
  POST /api/user/auth/refresh = 200
  GET /api/admin/users = 200
  GET /api/admin/analytics/overview = 200
  GET /api/admin/analytics/usage?days=7 = 200
  GET /api/admin/settings = 200
  PUT /api/admin/settings/default_markup = 200
  GET /api/admin/alerts/rules = 200
  GET /api/admin/alerts/history = 200
  GET /api/admin/audit-logs?limit=5 = 200
  Audio gateway handlers:
    go test ./... = Pass
    docker compose build backend = Pass
    scripts/dev-check.sh = Pass
```

#### 未完成 / 后续增强

```text
自定义 Alert Rule 持久化尚未开启
Admin API Keys 页面当前是 owner/key count 总览，不暴露 key secret，也未提供跨用户 revoke UI
Analytics 当前基于 PostgreSQL request_logs 聚合，高并发生产环境仍建议接 ClickHouse
细粒度 RBAC permissions enforcement 仍是后续增强
File OpenAI-compatible endpoint 与用户侧 Files UI 基础闭环已完成；本地 EICAR malware-scan baseline 已完成；生产 OSS/S3 adapter、云安全扫描、对象存储 lifecycle policy 后续增强
DashScope Audio adapter 仍需按百炼音频 API 继续适配；当前网关层和 Mock adapter 已可用，并由 `scripts/regression/audio-mock.sh` 覆盖
```

#### 当前阶段判断

```text
v0.3 = Verified
v1.0 foundation = Verified / control-plane completeness improved
```

### 2026-06-23 — File Upload Foundation 补全

#### 新增

```text
migrations/011_v11_file_upload_foundation.sql
  uploaded_files 表
migrations/014_v14_file_governance_settings.sql
  allowed_upload_mime_types 系统设置
migrations/015_v15_file_retention_settings.sql
  file_retention_days 系统设置

backend/internal/config/config.go
  FILE_STORAGE_BACKEND / FILE_STORAGE_DIR 配置

backend/internal/filestore/store.go
  FileStore interface
  LocalStore implementation

backend/internal/storage/store.go
  FileRecord
  CreateFileRecord()
  GetFileRecord()
  DeleteFileRecord()

backend/internal/gateway/router.go
  POST /v1/files
  GET /v1/files
  GET /v1/files/:id
  GET /v1/files/:id/content
  DELETE /v1/files/:id
  POST /api/admin/files/retention/run

docker-compose.yml
  FILE_STORAGE_DIR 默认值
```

#### 完成

```text
OpenAI-compatible files metadata API 完成
本地开发文件存储闭环完成
FileStore adapter foundation 完成，gateway 不再直接读写 os 文件
文件列表与内容下载完成
文件 owner isolation 通过 user_id SQL 条件执行
max_file_size_mb 设置接入上传限制
allowed_upload_mime_types 设置接入上传 MIME allowlist
上传时执行 MIME sniffing 并记录 detected_mime / declared_mime
上传时计算 SHA-256 并写入 file metadata
file_retention_days 设置接入 Admin retention cleanup
Retention cleanup 支持 dry_run、limit、软删除 metadata、best-effort 删除本地 bytes
上传/删除写入 audit log
Retention cleanup 写入 audit log
```

#### 验证

```text
go test ./... = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
npm run build = Pass
scripts/dev-check.sh = Pass
File API smoke:
  POST /api/user/auth/register = 201
  POST /api/user/keys = 201
  POST /v1/files = 201
  GET /v1/files = 200
  GET /v1/files/:id = 200, metadata includes detected_mime + sha256
  GET /v1/files/:id/content = 200
  DELETE /v1/files/:id = 200
  disallowed MIME with allowed_upload_mime_types=application/pdf = 415
  POST /api/admin/files/retention/run with file_retention_days=1 = deleted_count 1, storage_deleted_count 1
  GET /v1/files/:id after retention cleanup = 404
bash -n scripts/smoke-test.sh = Pass
```

#### 未完成 / 后续增强

```text
当前 bytes 存储为本地 FILE_STORAGE_DIR，生产环境应替换为 OSS/S3 adapter
已完成本地 EICAR 签名扫描 baseline；生产对象存储 lifecycle policy、ClamAV/云安全扫描后续接 OSS/S3 adapter
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / files gap reduced
```

### 2026-06-24 — FileStore Adapter Foundation 补全

#### 新增

```text
backend/internal/filestore/store.go
  Store interface
  LocalStore Put/Get/Delete

backend/cmd/server/main.go
  初始化 FileStore 并注入 Gateway

backend/internal/gateway/router.go
  文件上传/下载/删除/retention cleanup 改为调用 FileStore interface
```

#### 完成

```text
上传文件由 FileStore.Put 写入对象并返回 bytes / sha256 / path
下载文件由 FileStore.Get 读取 bytes
删除文件和 retention cleanup 由 FileStore.Delete 删除对象
metadata.source 使用 FileStore.Backend()
FILE_STORAGE_BACKEND 配置已预留，当前 local implementation 完整可用
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
FileStore HTTP smoke:
  POST /api/user/auth/register = 201
  POST /api/user/keys = 201
  POST /v1/files = 201
  metadata.source = local
  metadata.sha256 length = 64
  GET /v1/files/:id/content content_match = yes
  DELETE /v1/files/:id deleted = true
  GET /v1/files/:id after delete = 404
```

### 2026-06-24 — Request Logs Pagination / Filter / Export 补全

#### 新增

```text
backend/internal/storage/store.go
  RequestLogFilter
  RequestLogListResult
  ListRequestLogsFiltered()

backend/internal/gateway/router.go
  GET /api/user/request-logs 支持 limit / offset / model / provider / status / from / to / format=csv

frontend/src/lib/api.ts
  RequestLogQuery
  getRequestLogs(params)
  downloadRequestLogsCsv(params)

frontend/src/pages/RequestLogs.tsx
  服务端过滤
  分页控制
  CSV 导出按钮
```

#### 完成

```text
Request Logs 支持 limit/offset 分页
Request Logs 支持 model/status/provider/date filter
Request Logs 支持 CSV 导出
CSV 导出不包含 request_preview / response_preview，避免批量导出 prompt/response 内容
前端 Request Logs 页面改为服务端查询和导出
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
Request Logs HTTP smoke:
  POST /api/user/auth/register = 201
  POST /api/user/keys = 201
  POST /v1/chat/completions = 200
  GET /api/user/request-logs?limit=5&offset=0&status=success&model=qwen-turbo = total 1, count 1
  GET /api/user/request-logs?limit=1&offset=0 = one item
  GET /api/user/request-logs?format=csv&limit=5&status=success&model=qwen-turbo = CSV header returned
```

### 2026-06-24 — Dashboard / Analytics p95-p99 Latency 补全

#### 新增

```text
backend/internal/storage/store.go
  AnalyticsOverview.p95_latency_ms / p99_latency_ms
  AdminAnalyticsOverview() percentile_cont(0.95 / 0.99)
  GetUsageLatencySummary()

backend/internal/gateway/router.go
  GET /api/user/dashboard 返回 average_latency_ms / p95_latency_ms / p99_latency_ms

frontend/src/pages/Dashboard.tsx
  Dashboard latency cards: Avg / P95 / P99

frontend/src/pages/Admin.tsx
  Admin Analytics overview cards: Avg / P95 / P99
```

#### 完成

```text
用户 Dashboard 可见当前用户 avg / p95 / p99 latency
Admin Analytics 可见全局 avg / p95 / p99 latency
PostgreSQL 使用 percentile_cont 计算 p95/p99
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
Latency percentile HTTP smoke:
  POST /api/user/auth/register = 201
  promote test user to admin and re-login = Pass
  POST /api/user/keys = 201
  POST /v1/chat/completions x3 = 200
  GET /api/user/dashboard returns average_latency_ms / p95_latency_ms / p99_latency_ms
  GET /api/admin/analytics/overview returns p95_latency_ms / p99_latency_ms
```

### 2026-06-24 — Dashboard Error Rate 补全

#### 新增

```text
backend/internal/storage/store.go
  GetUsageErrorSummary() 基于 usage_logs.status_code 统计当前用户 error_requests / error_rate

backend/internal/gateway/router.go
  GET /api/user/dashboard 返回 error_requests / error_rate

frontend/src/lib/api.ts
  DashboardData 增加 error_requests / error_rate

frontend/src/pages/Dashboard.tsx
  Dashboard summary cards 增加 Error Rate
```

#### 完成

```text
用户 Dashboard 可见当前用户 usage error rate
统计口径与用户 total_requests 一致，均来自 usage_logs
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
Dashboard error-rate HTTP smoke:
  POST /api/user/auth/register = 201
  inserted controlled usage_logs rows: 1 success + 1 error
  GET /api/user/dashboard returns total_requests=2, error_requests=1, error_rate=0.5
```

### 2026-06-24 — RBAC Workspace Membership Enforcement

#### 新增

```text
backend/internal/storage/store.go
  IsActiveWorkspaceMember()

backend/internal/gateway/router.go
  POST /api/user/keys with workspace_id now requires active workspace membership
```

#### 完成

```text
用户不能为自己不属于的 workspace 创建归属 API Key
用户成为 active workspace member 后，可创建绑定该 workspace 的 API Key
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
RBAC workspace membership HTTP smoke:
  admin creates organization/workspace
  normal user POST /api/user/keys with workspace_id before membership = 403
  admin POST /api/admin/workspaces/:id/members = 201
  normal user POST /api/user/keys with same workspace_id after membership = 201
```

### 2026-06-24 — Workspace Usage / Cost CSV Export

#### 新增

```text
backend/internal/storage/store.go
  ListWorkspaceRequestLogsFiltered()
  workspace-scoped request log filters: limit/offset/model/provider/status/from/to

backend/internal/gateway/router.go
  GET /api/admin/workspaces/:id/usage/export
  CSV export excludes request_preview / response_preview

frontend/src/lib/api.ts
  adminDownloadWorkspaceUsageCsv()

frontend/src/pages/Admin.tsx
  Workspaces detail panel adds Export Usage CSV action
```

#### 完成

```text
Admin 可从 workspace 维度导出 request/cost/token/latency CSV
导出口径基于 request_logs.workspace_id，与 workspace usage summary 归因一致
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
Workspace usage export HTTP smoke:
  admin creates organization/workspace
  inserted controlled request_logs row for workspace
  GET /api/admin/workspaces/:id/usage/export?limit=10&status=success returns CSV header
  CSV includes request_id and charged/upstream/gross margin fields
```

### 2026-06-24 — Admin Inference UI Foundation

#### 新增

```text
frontend/src/lib/api.ts
  InferenceCluster / InferenceNode / ModelDeployment types
  adminList/CreateInferenceClusters()
  adminList/CreateInferenceNodes()
  adminList/CreateModelDeployments()

frontend/src/pages/Admin.tsx
  Admin sidebar adds Inference
  /admin/inference route
  Cluster / Node / Deployment create forms
  Cluster / Node / Deployment list panels
```

#### 完成

```text
Admin Console 可管理 v1.0 private/self-hosted inference 基础资源
前端与现有 /api/admin/inference/* API 对齐
deployment 注册仍沿用后端自动 upsert provider/model/model_provider binding
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
Inference HTTP smoke:
  POST /api/admin/inference/clusters = 201
  POST /api/admin/inference/nodes = 201
  POST /api/admin/inference/deployments = 201
  GET /api/admin/inference/deployments includes created deployment
```

### 2026-06-24 — Admin Guardrails UI Foundation

#### 新增

```text
frontend/src/lib/api.ts
  GuardrailPolicy / GuardrailResult types
  adminList/CreateGuardrailPolicies()
  adminListGuardrailResults()

frontend/src/pages/Admin.tsx
  Admin sidebar adds Guardrails
  /admin/guardrails route
  Guardrail policy create form
  Guardrail policy list
  Recent guardrail results list
```

#### 完成

```text
Admin Console 可创建/查看 guardrail policy
Admin Console 可查看最近 guardrail results
前端与现有 /api/admin/guardrails/* API 对齐
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
Guardrails API smoke:
  POST /api/admin/guardrails/policies = 201
  GET /api/admin/guardrails/policies includes created policy
  GET /api/admin/guardrails/results returns array
```

### 2026-06-24 — Admin Benchmarks UI Foundation

#### 新增

```text
frontend/src/lib/api.ts
  BenchmarkTask / BenchmarkRun / BenchmarkResult types
  adminList/CreateBenchmarkTasks()
  adminList/GetBenchmarkRuns()
  adminRunBenchmark()

frontend/src/pages/Admin.tsx
  Admin sidebar adds Benchmarks
  /admin/benchmarks route
  Benchmark task create form
  Benchmark run form with model shortcuts
  Benchmark task/run/result panels
```

#### 完成

```text
Admin Console 可创建 benchmark task
Admin Console 可对 selected active models 运行 benchmark
Admin Console 可查看 benchmark run detail 和 model result scores
前端与现有 /api/admin/benchmarks/* API 对齐
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
Benchmark API smoke:
  POST /api/admin/benchmarks/tasks = 201
  POST /api/admin/benchmarks/tasks/:id/runs = 201
  GET /api/admin/benchmarks/runs/:id returns results with total_score > 0
```

### 2026-06-24 — User Workflows UI Foundation

#### 新增

```text
frontend/src/lib/api.ts
  Tool / Workflow / WorkflowRun / WorkflowRunStep / AgentTrace types
  listTools()
  listWorkflows()
  createWorkflow()
  listWorkflowRuns()
  runWorkflow()
  getWorkflowRun()

frontend/src/pages/Workflows.tsx
  用户侧 workflow 创建页面
  支持 tool step / prompt step 单步 workflow
  支持 echo builtin tool 默认 smoke 路径
  支持运行 workflow、查看 run output、run steps、agent traces、cost

frontend/src/App.tsx
frontend/src/components/layout/DashboardLayout.tsx
  /dashboard/workflows 路由和侧边栏入口
```

#### 完成

```text
用户可在 Dashboard 创建 workflow
用户可运行 workflow 并查看 run history
用户可查看 step execution record 和 agent trace
前端与 /api/user/workflows、/api/user/workflow-runs API 对齐
```

#### 验证

```text
npm run build = Pass
go test ./... = Pass
workflow echo API smoke = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
```

### 2026-06-24 — User Files UI Foundation

#### 新增

```text
frontend/src/lib/api.ts
  UploadedFile / FilesResponse / DeleteFileResponse types
  listFiles()
  uploadFile()
  getFile()
  downloadFile()
  deleteFile()

frontend/src/pages/Files.tsx
  用户侧文件上传页面
  支持 purpose 过滤
  支持文件搜索、列表、metadata detail
  支持下载和删除文件
  展示 detected MIME、storage source、SHA-256 checksum

frontend/src/App.tsx
frontend/src/components/layout/DashboardLayout.tsx
  /dashboard/files 路由和侧边栏入口
```

#### 完成

```text
用户可在 Dashboard 上传文件
用户可查看 OpenAI-compatible /v1/files metadata
用户可下载原始文件 bytes
用户可删除文件并从列表移除
前端与 /v1/files API 对齐
```

#### 验证

```text
npm run build = Pass
go test ./... = Pass
File API smoke = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
Browser /dashboard/files render/search smoke = Pass
```

### 2026-06-24 — Billing Add Credits Guidance

#### 新增

```text
frontend/src/pages/Billing.tsx
  Add Credits 按钮从无反馈按钮改为可展开说明面板
  明确当前部署为 prepaid balance + admin credit grant 模式
  说明 credit_grant 和 usage_charge 的交易流
  保留 billing transaction CSV export
```

#### 完成

```text
Billing 页面不再给用户一个无动作的充值按钮
当前没有 self-service checkout API，因此不伪装成已接支付
用户可理解当前充值路径：请求 admin grant -> Admin Users grant balance -> usage 自动扣费
```

#### 验证

```text
npm run build = Pass
go test ./... = Pass
docker cp + restart frontend container = Pass
curl http://localhost:5175/dashboard/billing = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
```

#### 未完成 / 后续增强

```text
Self-service checkout / Stripe / payment provider integration 尚未实现
```

### 2026-06-24 — Docs Page Live API Reference

#### 新增

```text
frontend/src/pages/Docs.tsx
  读取 /api/marketplace/models 并展示当前 active catalog count
  Development base URL 从 http://localhost:8080/v1 修正为 http://localhost:8081/v1
  SDK examples 使用当前本地 base URL
  Endpoints 列表补齐 /v1/files list/get/download/delete
```

#### 完成

```text
Docs 页面与当前 docker-compose 暴露端口一致
Docs 页面与 OpenAI-compatible /v1/files API 覆盖面一致
Docs 页面展示 catalog 当前模型数量，避免纯静态文档脱节
```

#### 验证

```text
npm run build = Pass
GET /api/marketplace/models = Pass
curl http://localhost:5175/docs = Pass
go test ./... = Pass
docker cp + restart frontend container = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
```

#### 未完成 / 后续增强

```text
Docs 仍可继续增加 request/response schema examples 和 error format examples
```

### 2026-06-24 — Landing Featured Models Live Catalog

#### 新增

```text
frontend/src/pages/Landing.tsx
  Featured Models 从静态 mockModels 改为调用 /api/marketplace/models
  按 marketplace_score 排序取前 8 个模型
  使用 live catalog 的 display_name / modality / recommended_use / capabilities / price / availability
  Hero 和 Core Features 的模型数量文案使用 /api/marketplace/models count
  DemoCard 的 Generate 按钮改为打开 /playground?tab=image&model=wan-image&prompt=...
  Playground 支持读取 tab/model/prompt query 参数并预填状态
  增加 loading 和 unavailable empty state
```

#### 完成

```text
首页 Featured Models 与 Marketplace、Pricing、Admin Models 数据源统一
首页不再展示过期 mock model pricing / provider 信息
首页不再写死 1000+ models，本地当前按 catalog count 显示 33+
首页 DemoCard 不再是无动作静态按钮，可进入真实 Playground 执行链路
```

#### 验证

```text
npm run build = Pass
GET /api/marketplace/models = Pass, returned at least 8 models for Featured Models
GET /api/marketplace/models = Pass, returned count=33 for dynamic hero count
go test ./... = Pass
docker cp + restart frontend container = Pass
curl http://localhost:5175/ = Pass
curl http://localhost:5175/playground?tab=image&model=wan-image&prompt=A%20serene%20garden = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
```

#### 未完成 / 后续增强

```text
Landing DemoCard 目前预填 Playground，不直接执行生成；真实生成仍需用户提供 API key
```

### 2026-06-24 — Admin Overview Live Data

#### 新增

```text
frontend/src/pages/Admin.tsx
  AdminOverview 从静态占位数据改为真实 API 数据
  读取 /api/admin/analytics/overview
  读取 /api/admin/provider-health
  读取 /api/marketplace/models
  展示真实 total requests / active users / error rate / latency / cost / margin
  展示真实 provider health 摘要并链接 Provider Status
  展示 marketplace score 排序的 catalog model 摘要并链接 Marketplace
```

#### 完成

```text
Admin 首页不再使用静态 request/user/error/latency/provider/top-model 数据
Admin 首页与 Analytics、Provider Status、Marketplace catalog 数据源保持一致
Provider Status 页面规格文档更新为已接真实 API、manual check 和 history panel
```

#### 验证

```text
npm run build = Pass
Admin overview API smoke = Pass
  /api/admin/analytics/overview returned total_requests=62 total_users=105 provider_count=6 healthy_providers=5
  /api/admin/provider-health returned 6 rows
  /api/marketplace/models returned 33 models
go test ./... = Pass
docker cp + restart frontend container = Pass
curl http://localhost:5175/admin = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
```

#### 未完成 / 后续增强

```text
Admin Overview 的 Top Catalog Models 当前按 marketplace_score 排序；后续可改为真实 24h request/cost top models API
```

### 2026-06-24 — Pricing Page Live Catalog

#### 新增

```text
frontend/src/pages/Pricing.tsx
  从静态 mockModels 改为调用 /api/marketplace/models
  按 text / image / video / audio / embedding modality 分组展示
  展示 live catalog 的 price_unit、input_price、output_price、context、stream、provider availability
  增加 loading / error / empty states
```

#### 完成

```text
Pricing 页面现在跟 Admin Models 价格配置和 Marketplace catalog 保持一致
前台价格页不再展示过期 mock model pricing
Embedding pricing 被纳入公开价格表
```

#### 验证

```text
npm run build = Pass
GET /api/marketplace/models = Pass, returned 33 active marketplace models
go test ./... = Pass
docker cp + restart frontend container = Pass
curl http://localhost:5175/pricing = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
Browser render validation = Blocked by Browser local URL security policy; not bypassed
```

#### 未完成 / 后续增强

```text
可继续补 Pricing page 的浏览器截图验证，需 Browser 插件允许访问当前 localhost 地址
Pricing history / price-change audit baseline 已完成；未来价格计划、回滚、provider multiplier history 待增强
```

### 2026-06-24 — Admin Model Provider Binding UI

#### 新增

```text
frontend/src/lib/api.ts
  adminBindModelProvider()
  adminUpdateModelProvider()
  adminDeleteModelProvider()

frontend/src/pages/Admin.tsx
  Admin Models 页面支持 provider binding 内嵌编辑
  可编辑 upstream_model / priority / timeout_ms / max_retries / cost_multiplier / is_enabled
  可新增 provider binding
  可删除 provider binding
```

#### 完成

```text
Admin 不再需要直接改数据库来校准模型 provider 映射
修改 provider binding 后后端会刷新 router registry，下一次模型调用使用新配置
该能力用于后续校准 Singapore Bailian image/video/audio upstream_model 和请求映射
```

#### 验证

```text
npm run build = Pass
go test ./... = Pass
Admin binding API smoke = Pass, text-embedding-v3/bailian_intl upstream_model update then restore
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
frontend container sync = Pass, /admin/models reachable
```

#### 未完成 / 后续增强

```text
Singapore Bailian image/video upstream_model 仍需用真实官方模型名继续校准
Admin Models 后续可增加 provider 下拉选择与保存成功 toast
```

### 2026-06-24 — Playground Embeddings / Async Tasks UI Upgrade

#### 新增

```text
frontend/src/lib/api.ts
  EmbeddingRequest / EmbeddingResponse types
  AsyncTaskResponse / AsyncTaskDetail types
  createEmbedding()
  createImageGeneration()
  getImageGenerationTask()
  createVideoGeneration()
  getVideoGenerationTask()

frontend/src/pages/Playground.tsx
  新增 Embedding tab
  Image / Video tab 接入真实 async submit + polling API
  按 tab 过滤 text / embedding / image / video models
  Embedding 输出 vector dimensions、usage、preview values
  Image / Video 输出 async task id、status、result JSON
  Show Code 按当前 tab 生成对应 curl
```

#### 完成

```text
Playground 不再只支持 text chat
Embedding 可直接从页面调用 /v1/embeddings
Image / Video 可直接从页面提交 async task 并轮询状态
Audio 保留提示：网关/mock 已有并通过 `scripts/regression/audio-mock.sh` 覆盖，真实 DashScope audio adapter 待映射
```

#### 验证

```text
npm run build = Pass
Embedding API smoke = Pass, text-embedding-v3 returned 1024-d vector
Image async API smoke = Reaches Bailian upstream, but current catalog model names return InvalidParameter Model not exist
Video async API smoke = Reaches Bailian upstream, but current text-to-video request shape returns upstream URL parameter error
go test ./... = Pass
scripts/dev-check.sh = Pass
bash -n scripts/smoke-test.sh = Pass
Browser /playground render smoke = Pass
```

#### 未完成 / 后续增强

```text
Singapore Bailian image model IDs / upstream_model 映射需要按实时百炼文档校准
Singapore Bailian video text-to-video request shape 需要按实时百炼文档校准
Async task UI 当前展示 JSON result，后续可渲染 image/video preview card
```

### 2026-06-24 — Provider Health History 补全

#### 新增

```text
backend/internal/storage/store.go
  ListProviderHealthHistory()

backend/internal/gateway/router.go
  GET /api/admin/providers/:id/health/history

frontend/src/lib/api.ts
  ProviderHealthHistoryResponse
  getProviderHealthHistory()

frontend/src/pages/Admin.tsx
  Provider Status history button
  Health History panel
  Recent status bar and history list
```

#### 完成

```text
Admin 可查看单个 provider 最近 N 次 health check
Provider Status 页面可打开 health history panel
手动 health check 后会刷新 latest 和 history
History response 包含 status / latency_ms / error_code / error_message / checked_at
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
Provider health history HTTP smoke:
  POST /api/user/auth/register = 201
  promote test user to admin and re-login = Pass
  GET /api/admin/provider-health = 200
  POST /api/admin/providers/:id/health-check = request accepted and writes history
  GET /api/admin/providers/:id/health/history?limit=5 = count >= 1
  history item includes provider_id / status / latency_ms / checked_at
```

### 2026-06-24 — Admin Credit Grant 验证同步

#### 当前实现

```text
backend/internal/gateway/router.go
  POST /api/admin/users/:id/balance

backend/internal/storage/store.go
  AddBalance()
  CreateBillingTransaction()

frontend/src/pages/Admin.tsx
  Admin Users 页面 Grant Balance 控件
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
Admin Credit Grant HTTP smoke:
  POST /api/user/auth/register admin user = 201
  promote admin user and re-login = Pass
  POST /api/user/auth/register target user = 201
  GET /api/user/billing/balance before = 10
  POST /api/admin/users/:id/balance amount_usd=7.25 = ok true
  GET /api/user/billing/balance after = 17.25
  GET /api/user/billing/transactions latest tx_type = credit_grant
  latest amount_usd = 7.25
```

### 2026-06-24 — Standard Error Code 文档补全

#### 新增

```text
ai_aggregator_codex_pack/09_API_DESIGN.md
  Error Format
  Error Code Categories
  Standard Error Codes
  HTTP status / code / type / retry / client action table

README.md
  Error envelope 指引
```

#### 完成

```text
标准错误 envelope 已文档化：
  error.code
  error.type
  error.message
  error.request_id

errorTypeForCode() 当前映射已文档化：
  auth_error
  billing_error
  rate_limit_error
  provider_error
  routing_error
  validation_error
  internal_error

常见 HTTP 状态、错误 code、是否建议重试、客户端处理建议已补齐。
```

#### 验证

```text
Cross-check backend/internal/gateway/router.go:errorTypeForCode = Pass
README references standard table = Pass
bash -n scripts/smoke-test.sh = Pass
```

### 2026-06-24 — File Retention Cleanup 补全

#### 新增

```text
migrations/015_v15_file_retention_settings.sql
  file_retention_days 系统设置，0 表示禁用自动 retention cleanup

backend/internal/storage/store.go
  ListExpiredFileRecords()
  MarkFileRecordDeletedByID()

backend/internal/gateway/router.go
  POST /api/admin/files/retention/run
```

#### 完成

```text
Admin 可按 file_retention_days 执行过期文件清理
支持 dry_run 和 limit
清理时将 uploaded_files.status 标记为 deleted
清理时 best-effort 删除本地 FILE_STORAGE_DIR bytes
Retention cleanup 写入 audit log
用户再次 GET /v1/files/:id 会返回 404
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
Migration 015 applied = Pass
Retention HTTP smoke:
  POST /api/user/auth/register = 201
  POST /api/user/keys = 201
  POST /v1/files = 201
  backdate uploaded_files.created_at by 2 days
  PUT /api/admin/settings/file_retention_days = 200
  POST /api/admin/files/retention/run = 200, deleted_count=1, storage_deleted_count=1
  GET /v1/files/:id = 404
  file_retention_days restored to 0
```

### 2026-06-23 — Admin API Keys Management 补全

#### 新增

```text
backend/internal/storage/store.go
  AdminAPIKeyInfo
  ListAPIKeysAdmin()
  GetAPIKeyAdmin()
  UpdateAPIKeyAdmin()
  RevokeAPIKeyAdmin()

backend/internal/gateway/router.go
  GET /api/admin/keys
  PUT /api/admin/keys/:id
  DELETE /api/admin/keys/:id

frontend/src/lib/api.ts
  AdminApiKey / AdminApiKeysResponse
  adminListApiKeys()
  adminUpdateApiKey()
  adminRevokeApiKey()

frontend/src/pages/Admin.tsx
  Admin Keys 页面改为真实 key metadata table
  Key Controls 编辑面板
  Show revoked keys toggle
  Revoke action
```

#### 完成

```text
Admin 可跨用户查看 API key 元数据
Admin 可按 include_inactive 查看 revoked key
Admin 可撤销任意 API key
Admin 可编辑 key name / workspace / active / expiry
Admin 可编辑 per-key RPM / TPM 和 permissions JSON
permissions.models 会在 /v1 单模型请求前生效
per-key RPM 会覆盖默认 Redis sliding-window RPM
per-key TPM 会根据 provider usage 或 stream preflight estimate 计数并返回 tpm_limit
Key secret / key_hash 继续保持不可见
Update / Revoke 写入 audit log
前端 Admin Keys 页面接入真实接口
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
scripts/dev-check.sh = Pass
Admin key smoke:
  POST /api/user/auth/register admin = 201
  POST /api/user/auth/register owner = 201
  POST /api/user/keys = 201
  GET /api/admin/keys?include_inactive=true = 200
  PUT /api/admin/keys/:id = 200
  GET /v1/models with rate_limit_rpm=1 = 200 then 429
  POST /v1/embeddings with disallowed permissions.models = 403
  POST /v1/chat/completions with rate_limit_tpm=1 = 429 tpm_limit
  POST /v1/chat/completions stream=true with rate_limit_tpm=1 = 429 tpm_limit
  DELETE /api/admin/keys/:id = 200
  GET /v1/models with revoked key = 401
```

#### 未完成 / 后续增强

```text
尚未提供 workspace/user 组合过滤 UI
streaming chat 当前使用 preflight estimate，后续可继续提升估算精度
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / admin key management improved
```

### 2026-06-23 — Alert Rule Persistence 补全

#### 新增

```text
migrations/012_v12_alert_rules.sql
  alert_rules 表

backend/internal/storage/store.go
  AlertRule
  ListAlertRules()
  CreateAlertRule()
  GetAlertRule()
  UpdateAlertRule()

backend/internal/gateway/router.go
  GET /api/admin/alerts/rules
  POST /api/admin/alerts/rules
  PUT /api/admin/alerts/rules/:id

frontend/src/lib/api.ts
  adminCreateAlertRule()
  adminUpdateAlertRule()

frontend/src/pages/Admin.tsx
  Alert rule create form
  Alert rule enable/disable action
  Include disabled rules in Admin Alerts view
```

#### 完成

```text
Alert rules 从硬编码列表升级为 PostgreSQL 持久化
默认内置规则通过 migration seed
Admin 可创建自定义 alert rule
Admin 可启用/禁用 alert rule
Rule create/update 写入 audit log
Admin Alerts 页面接入真实 CRUD
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
scripts/dev-check.sh = Pass
Alert rule smoke:
  GET /api/admin/alerts/rules?include_disabled=true = 200
  POST /api/admin/alerts/rules = 201
  PUT /api/admin/alerts/rules/:id = 200
  GET /api/admin/alerts/history = 200
```

#### 未完成 / 后续增强

```text
Alert evaluation currently still derives current alerts from provider health and request error rate
尚未实现 alert_history 持久化表、notification channel、dedup/silence policy
尚未按自定义规则自动生成持久化 firing history
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / alert rules persistence improved
```

### 2026-06-23 — Alert Events / Acknowledge 补全

#### 新增

```text
migrations/013_v13_alert_events.sql
  alert_events 表

backend/internal/storage/store.go
  UpsertAlertEvent()
  ListAlertEvents()
  AcknowledgeAlertEvent()
  ResolveAlertEvent()

backend/internal/gateway/router.go
  POST /api/admin/alerts/history/:id/ack
  POST /api/admin/alerts/history/:id/resolve

frontend/src/lib/api.ts
  adminAcknowledgeAlert()
  adminResolveAlert()

frontend/src/pages/Admin.tsx
  Alert event Ack action
  Alert event Resolve action
  Alert status / last seen display
```

#### 完成

```text
Alert firing history 从纯实时派生升级为 PostgreSQL 持久化
Provider health / error-rate 当前告警会 upsert 到 alert_events
Admin 可 acknowledge alert event
Acknowledged status 会在 history 中保留
Admin 可 resolve alert event
Resolved status 会在 history 中保留，不会被同一个当前信号立即重开
Ack 操作写入 audit log
Resolve 操作写入 audit log
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
scripts/dev-check.sh = Pass
Alert event smoke:
  INSERT unhealthy provider health check = OK
  GET /api/admin/alerts/history = 200, open event found
  POST /api/admin/alerts/history/:id/ack = 200
  GET /api/admin/alerts/history = 200, event remains acknowledged
Alert resolve smoke:
  INSERT unhealthy provider health check = OK
  GET /api/admin/alerts/history = 200, open event found
  POST /api/admin/alerts/history/:id/resolve = 200
  GET /api/admin/alerts/history = 200, event remains resolved
```

#### 未完成 / 后续增强

```text
尚未实现 notification channel
尚未实现 silence / mute policy
尚未实现自动 resolved 状态转换；当前 resolved 为 Admin 手动关闭
尚未按每条自定义 rule 独立执行 evaluator
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / alert event lifecycle improved
```

### YYYY-MM-DD — v0.2 Observability 初步实现

#### 新增

```text
ai_aggregator_codex_pack/
migrations/003_v02_observability.sql
request_logs 表
provider_health_checks 表
fallback_logs 表
users.is_admin 字段
RecordRequestLog()
ListRequestLogs()
GetRequestLogByRequestID()
GET /api/user/request-logs
GET /api/user/request-logs/:request_id
frontend RequestLogs 页面
基础 provider failover chain
基础 provider health loop
统一 error format 增强
```

#### 完成

```text
Request Logs 主功能基本完成
v0.2 文档体系基本完成
基础 fallback 框架完成
```

#### 未完成

```text
fallback_logs 写入
provider_health_checks 写入
Provider Health API
Provider Status Page 真实数据
Admin CRUD
Fallback Smoke Test
Mock Provider failure mode
```

#### 当前阶段判断

```text
v0.2-complete
```

---

### 2026-06-25 — RBAC Workspace Permission Enforcement

#### 新增

```text
backend/internal/storage/store.go WorkspaceMemberHasPermission()
backend/internal/storage/store.go workspaceRoleNameAllows()
backend/internal/storage/store.go permissionSetAllows()
backend/internal/gateway/router.go POST /api/user/keys requires api_keys:create when workspace_id is present
backend/internal/gateway/router.go permission_denied now maps to permission_error
```

#### 完成

```text
workspace-scoped API key creation now requires active workspace membership plus api_keys:create permission
workspace-scoped file upload now requires files:write permission
workspace-scoped workflow create/run now requires workflows:write permission
workspace budget create now requires workspace_budgets:write for non-global-admin JWT callers
workspace quota create now requires workspace_quotas:write for non-global-admin JWT callers
Default role behavior:
  owner/admin = allow all workspace permissions
  member = allow api_keys:create/files:write/workflows:write plus read/write operational defaults
  viewer = read-only, denies api_keys:create/files:write/workflows:write/workspace_budgets:write/workspace_quotas:write
Custom roles may allow "*" or specific permissions through roles.permissions
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
bash -n scripts/smoke-test.sh = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
RBAC HTTP smoke:
  admin creates organization/workspace
  admin adds normal user as viewer
  viewer POST /api/user/keys with workspace_id = 403 permission_denied / permission_error
  admin updates same user to member
  member POST /api/user/keys with workspace_id = 201
File RBAC HTTP smoke:
  controlled viewer workspace API key POST /v1/files = 403 permission_denied / permission_error
  member workspace API key POST /v1/files = 201
  uploaded file workspace_id matches API key workspace_id
Workflow RBAC HTTP smoke:
  viewer POST /api/user/workflows with workspace_id = 403 permission_denied / permission_error
  member POST /api/user/workflows with workspace_id = 201
  member POST /api/user/workflows/:id/runs = 201 completed
  workflow and workflow_run workspace_id match target workspace
Budget/Quota RBAC HTTP smoke:
  viewer POST /api/user/workspaces/:id/budgets = 403 permission_denied / permission_error
  owner POST /api/user/workspaces/:id/budgets = 201
  owner POST /api/user/workspaces/:id/quotas with tokens_per_month = 201
  invalid quota_type returns 400 invalid_request before database constraint failure
```

#### 当前阶段判断

```text
v0.3/v1.0 foundation = Verified / workspace role permission enforcement expanded to api_keys:create, files:write, workflows:write, workspace_budgets:write, and workspace_quotas:write
Remaining RBAC enhancement: expand admin-delegated operations and custom role management UI.
```

---

### 2026-06-25 — Admin Workspaces Budget / Quota UI

#### 新增

```text
frontend/src/lib/api.ts adminListWorkspaceBudgets()
frontend/src/lib/api.ts adminCreateWorkspaceBudget()
frontend/src/lib/api.ts adminListWorkspaceQuotas()
frontend/src/lib/api.ts adminCreateWorkspaceQuota()
frontend/src/pages/Admin.tsx AdminWorkspaces loads and displays selected workspace budgets/quotas
frontend/src/pages/Admin.tsx AdminWorkspaces can create monthly/daily budgets and quota rows
```

#### 验证

```text
npm run build = Pass
scripts/dev-check.sh = Pass
frontend container updated with docker cp frontend/. aag-frontend:/app && docker restart aag-frontend
```

#### 当前阶段判断

```text
v0.3/v1.0 foundation = Verified / workspace budget/quota governance is now available from Admin Workspaces UI as well as API.
```

---

### 2026-06-25 — Admin Workspaces Member Management UI

#### 新增

```text
frontend/src/pages/Admin.tsx AdminWorkspaces loads admin users for member selection
frontend/src/pages/Admin.tsx Add Member changed from raw user_id input to user dropdown + role + status
frontend/src/pages/Admin.tsx Members list now displays email/username when available, with user_id fallback
```

#### 验证

```text
npm run build = Pass
scripts/dev-check.sh = Pass
frontend container updated with docker cp frontend/. aag-frontend:/app && docker restart aag-frontend
```

#### 当前阶段判断

```text
v0.3/v1.0 foundation = Verified / workspace team management is usable from Admin Workspaces UI without manually copying user IDs.
```

---

### 2026-06-25 — Completed Feature Test Case Matrix

#### 新增

```text
ai_aggregator_codex_pack/15_COMPLETED_FEATURE_TEST_CASES.md
```

#### 覆盖范围

```text
Core health / toolchain / Docker / Helm
Auth / Profile
API Key lifecycle and workspace RBAC
OpenAI-compatible chat/models/billing/usage
Embeddings / image async / video async / audio gateway / files
Request logs / provider health / fallback
Admin models/providers/pricing
Organization / Workspace / Team / RBAC / Budget / Quota
Admin users / keys / analytics / alerts / audit logs
Marketplace / pricing / docs / landing
Benchmark foundation
Guardrails / compliance
Workflow / agent foundation
Private/self-hosted inference and deployment
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
scripts/dev-check.sh = Pass
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / completed feature test case matrix now documents test goal, method, expected result, current result, pass criteria, and evidence source for completed features.
```

---

### 2026-06-25 — Full Test Execution Results

#### 新增

```text
ai_aggregator_codex_pack/16_TEST_EXECUTION_RESULTS_2026-06-25.md
```

#### 执行结果

```text
backend go test ./... = Pass
frontend npm run build = Pass
scripts/dev-check.sh = Pass
mock full smoke = 23 pass / 1 skip / 0 fail
real provider smoke = 18 pass / 4 skip / 0 fail
enterprise/admin/private inference API regression = 38 pass / 0 fail
```

#### 修复

```text
backend/internal/gateway/router.go
  Added enum validation for inference cluster network_mode/status
  Added enum validation for inference node status
  Added enum validation for model deployment runtime/status
```

#### 当前阶段判断

```text
v1.0 foundation = Verified by executable local regression. Real Bailian embedding works; qwen chat needs a targeted model-selection smoke because the generic script selected a local self-hosted smoke model.
```

---

### 2026-06-26 — Targeted Bailian Qwen Smoke

#### 新增

```text
ai_aggregator_codex_pack/17_TARGETED_BAILIAN_QWEN_SMOKE_2026-06-26.md
scripts/smoke-test.sh SMOKE_MODEL_ID / SMOKE_EMBEDDING_MODEL_ID / SMOKE_IMAGE_MODEL_ID / SMOKE_VIDEO_MODEL_ID
```

#### 执行结果

```text
BASE_URL=http://localhost:8081 MOCK_PROVIDER_MODE=false SMOKE_MODEL_ID=qwen3.7-max RUN_EMBEDDING_SMOKE=true RUN_ASYNC_SMOKE=false bash scripts/smoke-test.sh

Total: 22
Passed: 19
Skipped: 3
Failed: 0
```

#### 验证点

```text
qwen3.7-max real Bailian chat = HTTP 200
balance deduction = Pass
usage log model=qwen3.7-max = Pass
billing transaction usage_charge = Pass
text-embedding-v3 real embedding vector_len=1024 = Pass
```

#### 当前阶段判断

```text
v1.0 foundation = Real Bailian Qwen chat and embedding are now directly verified through explicit smoke model selection.
```

---

### 2026-06-26 — Provider Fallback Smoke Execution

#### 新增

```text
ai_aggregator_codex_pack/18_FALLBACK_SMOKE_RESULTS_2026-06-26.md
```

#### 执行结果

```text
MOCK_PROVIDER_MODE=true MOCK_FAIL_PROVIDER_IDS=bailian_cn docker compose up -d --force-recreate backend
BASE_URL=http://localhost:8081 MOCK_PROVIDER_MODE=true RUN_FALLBACK_SMOKE=true SMOKE_MODEL_ID=qwen3.7-plus RUN_EMBEDDING_SMOKE=true RUN_ASYNC_SMOKE=true bash scripts/smoke-test.sh

Total: 24
Passed: 24
Skipped: 0
Failed: 0
```

#### 验证点

```text
qwen-max primary provider bailian_cn failed as expected
fallback final_provider_id=bailian_intl
request_logs fallback_count=1
fallback_logs row written: bailian_cn -> bailian_intl
```

#### 当前阶段判断

```text
v0.2 reliability gateway = fallback smoke is now executed evidence, not only an optional script definition.
```

---

### 2026-06-26 — UI Browser Smoke + Mobile Dashboard Fix

#### 新增

```text
ai_aggregator_codex_pack/19_UI_BROWSER_SMOKE_RESULTS_2026-06-26.md
```

#### 修改

```text
frontend/src/components/layout/DashboardLayout.tsx
  Mobile: sidebar becomes top horizontal nav
  Desktop: fixed left sidebar preserved
  Main content uses md:ml-64 only on desktop
  Sidebar user block loads real /api/user/profile
  Logout button clears auth and returns to /login
```

#### 验证结果

```text
Login/register UI flow = Pass
Dashboard render = Pass
Request Logs render + success filter interaction = Pass
Files render = Pass
Workflows render + Create workflow interaction = Pass
Playground render = Pass
Dashboard logout = Pass
Admin login + Admin Overview render = Pass
Mobile 390x844 Files layout = Pass after fix
frontend npm run build = Pass
scripts/dev-check.sh = Pass
```

#### 当前阶段判断

```text
v1.0 frontend foundation = Basic browser UI smoke is now executed; mobile dashboard layout, real profile display, logout, and Admin Overview E2E have been verified. Admin Workspaces deep E2E remains a P1 follow-up.
```

---

### 2026-06-25 — Admin Audit Log Retention Policy

#### 新增

```text
migrations/016_v16_audit_log_retention_settings.sql
  audit_log_retention_days 系统设置，0 表示禁用 cleanup
POST /api/admin/audit-logs/retention/run
backend/internal/storage/store.go ListExpiredAuditLogs()
backend/internal/storage/store.go DeleteAuditLogsByIDs()
frontend/src/lib/api.ts adminRunAuditLogRetention()
frontend/src/pages/Admin.tsx Audit Log Retention panel
```

#### 完成

```text
Admin 可配置 audit_log_retention_days
Admin 可 dry-run 审计日志 retention cleanup
Admin 可执行审计日志 retention cleanup
cleanup 支持 limit，最高 10000
cleanup 写入 audit_log.retention_run audit event
Admin Settings 页面可执行 Dry Run / Run Cleanup 并展示 matched/deleted
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
bash -n scripts/smoke-test.sh = Pass
scripts/dev-check.sh = Pass
PUT /api/admin/settings/audit_log_retention_days = 200
POST /api/admin/audit-logs/retention/run dry_run=true = matched old event, deleted_count=0
old audit event remains after dry_run
POST /api/admin/audit-logs/retention/run dry_run=false = deleted expired event
old audit event removed after cleanup
audit_log_retention_days restored to 0
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / compliance audit export, filters and retention baseline completed
```

---

### 2026-06-25 — Admin Audit Log CSV Export

#### 新增

```text
GET /api/admin/audit-logs/export
GET /api/admin/audit-logs filters: action / workspace_id / resource_type / resource_id / from / to
GET /api/admin/audit-logs/export filters: action / workspace_id / resource_type / resource_id / from / to
backend/internal/gateway/router.go writeAuditLogsCSV()
backend/internal/gateway/router.go parseAuditLogFilter()
backend/internal/storage/store.go AuditLogFilter / ListAuditLogsFiltered()
frontend/src/lib/api.ts adminDownloadAuditLogsCsv()
frontend/src/pages/Admin.tsx Audit Log filters + Export CSV button
```

#### 完成

```text
Admin 可从 Audit Log 页面导出 CSV
Admin 可按 action / workspace_id / resource_type / resource_id / from / to 过滤 Audit Log 列表
Admin CSV 导出沿用当前 Audit Log 过滤条件
CSV 包含 id / created_at / user_id / organization_id / workspace_id / action / resource / ip / user_agent / details_json
导出接口使用 admin JWT 鉴权
ListAuditLogs 支持导出场景最高 10000 条记录
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
docker compose up -d backend = Pass
bash -n scripts/smoke-test.sh = Pass
scripts/dev-check.sh = Pass
GET /api/admin/audit-logs/export?limit=1 = 200
GET /api/admin/audit-logs?action=alert_rule.create&resource_type=alert_rule&resource_id=:id&from=2026-01-01 returns only target audit event
GET /api/admin/audit-logs/export?action=alert_rule.create&resource_type=alert_rule&resource_id=:id&from=2026-01-01 returns only target audit event
Content-Type = text/csv; charset=utf-8
Content-Disposition = attachment; filename="audit-logs.csv"
CSV header matches expected audit fields
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / compliance audit export and filters baseline completed
Follow-up completed: configurable audit log retention policy baseline added on 2026-06-25
```

---

### 2026-06-24 — User Settings / Profile UI

#### 新增

```text
frontend/src/pages/Settings.tsx
frontend/src/lib/api.ts UserProfile / getProfile() / updateProfile()
frontend/src/App.tsx /dashboard/settings route
frontend/src/components/layout/DashboardLayout.tsx Settings sidebar link
```

#### 完成

```text
用户 Settings 页面完成
当前登录用户 profile 加载完成
username 编辑保存完成
metadata JSON 编辑、校验、保存完成
email / role / balance 展示完成
未登录访问跳转 /login 完成
```

#### 验证

```text
npm run build = Pass
go test ./... = Pass
bash -n scripts/smoke-test.sh = Pass
curl http://localhost:5175/dashboard/settings = Pass
scripts/dev-check.sh = Pass
register -> GET /api/user/profile -> PUT /api/user/profile smoke = Pass
```

#### 当前阶段判断

```text
v1.0 foundation = Verified / user profile settings surface completed
```

---

### 2026-06-26 — Workspace Cost Attribution Breakdown

#### 本轮完成

```text
backend/internal/storage/store.go WorkspaceUsageSummary 增加 by_model / by_provider / by_user
backend/internal/storage/store.go GetWorkspaceUsageSummary() 增加 Top model/provider/user group-by 聚合
frontend/src/lib/api.ts 增加 WorkspaceUsageAttribution 类型
frontend/src/pages/Admin.tsx Admin Workspaces detail 增加 Cost Attribution 面板
scripts/regression/workspace-cost-attribution.sh 新增自动化回归
```

#### 行为变化

```text
GET /api/admin/workspaces/:id/usage 继续返回 total_requests / total_cost / total_tokens
同一接口新增 by_model / by_provider / by_user Top 归因数组
Admin Workspaces 页面可直接查看选中 workspace 的模型、provider、用户成本拆分
CSV export 保持兼容
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
scripts/dev-check.sh = Pass
scripts/regression/admin-foundation.sh = 24 pass / 0 fail
scripts/regression/workspace-cost-attribution.sh = 12 pass / 0 fail
```

#### 当前阶段判断

```text
v0.3/v1.0 foundation = Verified / workspace FinOps attribution now includes Top model/provider/user breakdown and regression coverage
```

---

### 2026-06-26 — Provider Credentials Baseline and Commercial Gap Analysis

#### 本轮完成

```text
backend/internal/storage/store.go ProviderKey lifecycle:
  CreateProviderKey()
  ListProviderKeys()
  RevokeProviderKey()
  GetActiveProviderKeySecret()

backend/internal/router/model_router.go:
  provider key resolution now checks active DB credential before environment fallback

backend/internal/gateway/router.go:
  GET /api/admin/providers/:id/keys
  POST /api/admin/providers/:id/keys
  DELETE /api/admin/providers/:id/keys/:key_id

frontend/src/pages/Admin.tsx:
  Admin Models page now includes Provider Credentials panel
  Admin can create OpenAI-compatible/DashScope/self_hosted/mock providers
  Admin can save/load/revoke masked provider credentials

scripts/regression/provider-credentials.sh:
  Provider credential regression added

ai_aggregator_codex_pack/35_COMMERCIALIZATION_GAP_ANALYSIS_AND_TARGETS_2026-06-26.md:
  Commercial gap analysis and targets documented
```

#### 行为变化

```text
Provider credentials can be stored in provider_keys without returning raw secrets.
Credential responses return key_mask only.
Runtime routing uses the latest active DB credential for a provider before falling back to env vars.
Provider credential create/revoke actions write audit_logs.
```

#### 验证

```text
go test ./... = Pass
npm run build = Pass
scripts/dev-check.sh = Pass
scripts/regression/admin-foundation.sh = 24 pass / 0 fail
scripts/regression/provider-credentials.sh = 11 pass / 0 fail
scripts/regression/invoices-po-postpaid.sh = 12 pass / 0 fail
```

#### 当前阶段判断

```text
v0.3/v1.0 foundation = Verified / platform/scoped provider credentials, provider credential validation, request log credential scope, project cost center, smart routing, and invoice/PO/postpaid baselines completed
Remaining commercial gap = provider real upstream smoke with valid keys, monthly statement/regional tax rules, persistent webhook queue/dead-letter UI, KMS/Vault envelope and credential rotation UI/API, Anthropic tool-use/native streaming refinement
```

---

## 15. 阶段状态标记规则

使用以下状态：

| 状态 | 含义 |
|---|---|
| Not Started | 未开始 |
| Planned | 已规划，未开发 |
| In Progress | 开发中 |
| Partial | 部分完成 |
| Implemented | 已实现 |
| Verified | 已通过测试验证 |
| Blocked | 被阻塞 |
| Deferred | 延后 |
| Deprecated | 废弃 |

版本完成规则：

```text
只有当该版本所有 P0 功能达到 Verified，才可以标记为 Complete。
```

---

## 16. 当前最终判断

当前项目状态：

```text
v0.1 = Runnable MVP / 基本完成
v0.2 = Complete / 完成 90%+
v0.3 = Complete / 完成 90%+
v0.4 = Complete foundation / Marketplace + pricing history baseline 已验证
v0.5 = Complete foundation / Benchmark + Smart Routing baseline 已验证
v0.6 = Complete foundation / Guardrails baseline 已验证
v0.7 = Complete foundation / Workflow-Agent baseline 已验证
v1.0 = In Progress foundation / Private deploy、files、admin regression、UI smoke、platform/scoped provider credentials、user BYOK self-service、provider credential validation、request log credential scope、OpenAI/Grok provider validation、secret management metadata、project cost center、smart routing baseline、invoice/PO/postpaid baseline、invoice CSV export/PDF export/status workflow、OpenAI/Grok/Anthropic provider onboarding templates、Anthropic native adapter、DashScope audio chain、signed webhook delivery worker 已完成，剩余 KMS/Vault envelope、monthly statement、provider real upstream smoke 等增强项
```

当前最重要的下一步：

```text
1. 增强 monthly statement automation、regional tax rules
2. 增强 provider real upstream smoke、DashScope live ASR/TTS smoke、Anthropic tool-use/native streaming refinement
3. 增强 Smart Routing A/B experiments / sticky bucketing / provider-level quality score
4. 增强 KMS/Vault envelope、credential rotation UI/API、usage audit stream
5. 增强 persistent webhook queue、dead-letter、replay UI
```

项目长期方向：

```text
短期：Reliable AI Gateway
中期：Enterprise Control Plane + AI FinOps
长期：Marketplace + Benchmark + Guardrails + Workflow / Agent + Private / Sovereign AI
```
