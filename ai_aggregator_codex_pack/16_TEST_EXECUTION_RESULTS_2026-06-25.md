# AI Aggregator Test Execution Results - 2026-06-25

文档名称：`16_TEST_EXECUTION_RESULTS_2026-06-25.md`  
关联测试用例：`15_COMPLETED_FEATURE_TEST_CASES.md`  
执行时间：2026-06-25 22:49 +04  
执行环境：本机 Docker Compose

---

## 1. 测试环境

| 项目 | 值 |
|---|---|
| Backend | `http://localhost:8081` |
| Frontend | `http://localhost:5175` |
| PostgreSQL | `aag-postgres`, `localhost:5433`, database `aggregator` |
| Redis | `aag-redis`, `localhost:6380` |
| Mock Provider Mode | mock 回归时 `true`；真实百炼 smoke 时 `false` |
| 外部 Provider | Alibaba Cloud Bailian / DashScope API key 已配置在本机 `.env`，文档不记录密钥 |

---

## 2. 执行摘要

| 测试组 | 命令 / 方法 | 结果 |
|---|---|---:|
| Backend Go tests | `cd backend && go test ./...` | Pass |
| Frontend production build | `cd frontend && npm run build` | Pass |
| Dev environment health | `scripts/dev-check.sh` | Pass |
| Mock full smoke | `MOCK_PROVIDER_MODE=true RUN_EMBEDDING_SMOKE=true RUN_ASYNC_SMOKE=true bash scripts/smoke-test.sh` | 23 pass / 1 skip / 0 fail |
| Real provider smoke | `MOCK_PROVIDER_MODE=false RUN_EMBEDDING_SMOKE=true RUN_ASYNC_SMOKE=false bash scripts/smoke-test.sh` | 18 pass / 4 skip / 0 fail |
| Enterprise / Admin / Private Inference API regression | inline API regression script | 38 pass / 0 fail |

总体结论：本轮自动化覆盖范围内的功能通过。真实百炼 smoke 已验证 embedding 可用；chat smoke 没有有效命中百炼 qwen 路由，原因是本地已有 self-hosted 测试模型排序靠前，脚本选择了该模型并按预期跳过 502，因此不能作为 qwen chat 模型可用性的最终证明。

---

## 3. 已执行测试明细

### 3.1 基础运行与构建

| 用例 | 测试目标 | 预期效果 | 测试结果 | 是否通过 |
|---|---|---|---|---|
| TC-OPS-001 | Docker Compose 服务运行 | backend/frontend/postgres/redis 均运行 | `scripts/dev-check.sh` 通过 | Pass |
| TC-OPS-002 | Backend health | `/health` 返回 `status=ok` | HTTP 200 | Pass |
| TC-OPS-003 | Helm render | chart 可渲染 | `helm template ok` | Pass |
| TC-BUILD-001 | Go 编译测试 | `go test ./...` 成功 | 全包通过 | Pass |
| TC-BUILD-002 | Frontend build | TypeScript + Vite build 成功 | build 成功，仅有 chunk size warning | Pass |

### 3.2 Mock Full Smoke

| 覆盖范围 | 测试结果 |
|---|---|
| 注册、登录、余额、API Key 创建/列表/撤销 | Pass |
| `/v1/models` | Pass |
| `/v1/chat/completions` mock 非流式调用 | Pass |
| 余额扣减、usage log、billing transaction | Pass |
| `/v1/embeddings` | Pass，vector length = 8 |
| Image async task submit/status | Pass |
| Video async task submit/status | Pass |
| Files upload/list/detail/download/delete | Pass |
| Revoked API key rejection | Pass |
| Provider fallback optional smoke | Skip，未开启 `RUN_FALLBACK_SMOKE=true` |

Mock smoke 汇总：Total 24，Passed 23，Skipped 1，Failed 0。

### 3.3 Real Provider Smoke

| 覆盖范围 | 测试结果 |
|---|---|
| 注册、登录、余额、API Key | Pass |
| `/v1/models` | Pass，返回 34 个模型 |
| `/v1/embeddings` | Pass，命中真实 provider，模型 `text-embedding-v3`，vector length = 1024 |
| Files API | Pass |
| API Key revoke / revoked key reject | Pass |
| Chat completion | Skip，脚本选中本地 self-hosted smoke 模型，外部端点不存在，返回 502 后被脚本按真实 provider smoke 规则跳过 |
| Image / Video async real provider | Skip，`RUN_ASYNC_SMOKE=false` |
| Provider fallback optional smoke | Skip，未开启 `RUN_FALLBACK_SMOKE=true` |

Real provider smoke 汇总：Total 22，Passed 18，Skipped 4，Failed 0。

### 3.4 Enterprise / Admin / Private Inference API Regression

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-AUTH-001 | 注册测试 admin 用户 | HTTP 201 | Pass |
| TC-AUTH-002 | admin 登录 | HTTP 200 + JWT | Pass |
| TC-AUTH-002B | 注册 viewer 用户 | HTTP 201 | Pass |
| TC-AUTH-003 | 缺失 JWT 被拒绝 | HTTP 401 | Pass |
| TC-AUTH-004 | 读取 profile | HTTP 200 | Pass |
| TC-MKT-001 | Marketplace model list | HTTP 200 | Pass |
| TC-ADMIN-MODEL-001 | Admin model list | HTTP 200 | Pass |
| TC-ADMIN-PROV-001 | Admin provider list | HTTP 200 | Pass |
| TC-OBS-005 | Provider health list | HTTP 200 | Pass |
| TC-ANALYTICS-001~005 | Overview/usage/cost/latency/errors | HTTP 200 | Pass |
| TC-ORG-001 | Organization create | HTTP 201 | Pass |
| TC-WS-001 | Workspace create | HTTP 201 | Pass |
| TC-TEAM-000 | Admin owner member upsert | HTTP 201 | Pass |
| TC-TEAM-001 | Viewer member upsert | HTTP 201 | Pass |
| TC-BUDGET-UI-001 | Workspace budget create | HTTP 201 | Pass |
| TC-BUDGET-UI-002 | Workspace quota create | HTTP 201 | Pass |
| TC-WS-USAGE-001 | Workspace usage JSON | HTTP 200 | Pass |
| TC-WS-USAGE-002 | Workspace usage CSV | HTTP 200 | Pass |
| TC-ADMIN-WS-UI-001 | Admin Workspaces page identity / not blank / no overlay | Browser `/admin/workspaces` | Pass |
| TC-ADMIN-WS-UI-003 | Admin Workspaces create organization from UI | Browser form submit | Pass |
| TC-ADMIN-WS-UI-004 | Admin Workspaces create workspace from UI | Browser form submit | Pass |
| TC-ADMIN-WS-UI-005 | Admin Workspaces budget create/list from UI | Browser form submit | Pass |
| TC-ADMIN-WS-UI-006 | Admin Workspaces quota create/list from UI | Browser form submit | Pass |
| TC-ADMIN-WS-UI-007 | Admin Workspaces member role/status from UI | Browser select user/role/status | Pass |
| TC-ADMIN-WS-UI-009 | Admin Workspaces CSV export API | HTTP 200 `text/csv` | Pass |
| TC-WF-001 | Workflow create | HTTP 201 | Pass |
| TC-WF-003 | Workflow run | HTTP 201 | Pass |
| TC-BENCH-001 | Benchmark task create | HTTP 201 | Pass |
| TC-BENCH-003 | Benchmark run create | HTTP 201 | Pass |
| TC-BENCH-004 | Benchmark run detail | HTTP 200 | Pass |
| TC-GUARD-001 | Guardrail policy create | HTTP 201 | Pass |
| TC-GUARD-004 | Guardrail results list | HTTP 200 | Pass |
| TC-ALERT-001 | Alert rule create | HTTP 201 | Pass |
| TC-AUDIT-001 | Audit log list | HTTP 200 | Pass |
| TC-AUDIT-002 | Audit CSV export | HTTP 200 | Pass |
| TC-AUDIT-003 | Audit retention dry run | HTTP 200 | Pass |
| TC-INF-001 | Inference cluster create | HTTP 201 | Pass |
| TC-INF-002A | Invalid inference node status rejected | HTTP 400 | Pass |
| TC-INF-002 | Inference node create | HTTP 201 | Pass |
| TC-INF-003 | Model deployment create | HTTP 201 | Pass |
| TC-INF-004 | Inference cluster list | HTTP 200 | Pass |

Enterprise regression 汇总：Pass 38，Fail 0。

### 3.5 Admin Foundation Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/admin-foundation.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-GUARD-001~003 | Guardrail policy/list/results | 创建成功、列表可见、results shape 正确 | Pass |
| TC-REG-BENCH-001~003 | Benchmark task/run/detail | run completed 且 results >= 1 | Pass |
| TC-REG-INF-001~005 | Inference cluster/node/deployment | cluster/node/deployment 创建成功，非法 node status 返回 400 | Pass |
| TC-REG-AUDIT-001~004 | Audit filter/export/retention | filter 返回 data array，CSV text/csv，retention dry-run 返回计数 | Pass |
| TC-REG-ALERT-001~005 | Alert rule/event ack/resolve | rule 创建、event 可见、ack 后 acknowledged、resolve 后 resolved | Pass |

Admin foundation regression 汇总：Total 24，Passed 24，Failed 0。详见 `21_ADMIN_FOUNDATION_REGRESSION_2026-06-26.md`。

### 3.6 User Marketplace + Workflow Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/user-marketplace-workflow.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-MKT-001~003 | Marketplace list/filter/search | 返回模型列表，modality filter 正确，query response shape 正确 | Pass |
| TC-REG-MKT-004 | Marketplace detail | 返回 model 和 providers | Pass |
| TC-REG-MKT-005~006 | Marketplace compare | 选中模型可比较；缺失 ids 返回 400 invalid_request | Pass |
| TC-AUTH-003 | JWT missing-token rejection | 未带 Authorization 调用 `/api/user/dashboard` 返回 401 `authentication_error` | Pass |
| TC-REG-WF-001~004 | Workflow user/create/detail | 用户注册登录成功，workflow 创建并包含两个 steps | Pass |
| TC-REG-WF-005~007 | Workflow run traces | run completed，steps=2，traces=2，detail 包含 step.completed traces | Pass |
| TC-REG-WF-008 | Workflow missing run error | 缺失 run 返回 404 not_found | Pass |

User marketplace/workflow regression 汇总：Total 18，Passed 18，Failed 0。详见 `22_USER_MARKETPLACE_WORKFLOW_REGRESSION_2026-06-26.md`。

### 3.7 Guardrails Policy Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/guardrails-policy.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-GUARD-POL-001~002 | Admin setup + blocking policy | 创建临时 `pii_action=block` / `injection_action=block` policy | Pass |
| TC-REG-GUARD-PII-001 | PII detection/block | 含 email/phone 的 chat 请求返回 400 `policy_violation` | Pass |
| TC-REG-GUARD-INJ-001 | Prompt injection block | injection pattern 请求返回 400 `policy_violation` | Pass |
| TC-REG-GUARD-RESULT-001~002 | guardrail_results | PII 和 injection 均写入 blocked result + findings | Pass |
| TC-REG-GUARD-AUDIT-001 | Audit | `guardrail.block` audit logs 写入 | Pass |
| TC-REG-GUARD-DB-001~002 | DB persistence | `pii_detections` 与 `policy_violations` 写入 | Pass |
| TC-REG-GUARD-CLEANUP-001 | Cleanup | 临时 regression policy 禁用，active count=0 | Pass |

Guardrails policy regression 汇总：Total 14，Passed 14，Failed 0。详见 `23_GUARDRAILS_POLICY_REGRESSION_2026-06-26.md`。

### 3.8 Auth Conflict Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/auth-conflict.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-AUTH-CONFLICT-001 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-AUTH-CONFLICT-002 | Initial registration | 新用户注册成功，返回 JWT 和 user | Pass |
| TC-REG-AUTH-CONFLICT-003 | Duplicate registration conflict | 重复注册返回 HTTP 409，`code=conflict`，`type=client_error`，稳定 message | Pass |
| TC-REG-AUTH-CONFLICT-004 | Error hygiene | 响应不包含 constraint、SQLSTATE、insert、pgx/pq 等数据库细节 | Pass |

Auth conflict regression 汇总：Total 4，Passed 4，Failed 0。详见 `24_AUTH_CONFLICT_REGRESSION_2026-06-26.md`。

### 3.9 Audio Mock Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/audio-mock.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-AUDIO-001 | Mock audio provider/model setup | upsert `mock_audio_regression` provider 和 `mock-audio-regression` model | Pass |
| TC-REG-AUDIO-002 | Backend reload | backend restart 后 `/health` 正常 | Pass |
| TC-REG-AUDIO-003 | Regression user registration | 新用户注册成功，返回 JWT 和 user | Pass |
| TC-REG-AUDIO-004 | Audio transcription | `/v1/audio/transcriptions` 返回 mock transcription text | Pass |
| TC-REG-AUDIO-005 | Audio speech | `/v1/audio/speech` 返回 mock audio bytes | Pass |
| TC-REG-AUDIO-006 | request_logs persistence | 两条 audio request_logs 写入 | Pass |
| TC-REG-AUDIO-007 | usage_logs persistence | 两条 audio usage_logs 写入 | Pass |

Audio mock regression 汇总：Total 7，Passed 7，Failed 0。详见 `25_AUDIO_MOCK_REGRESSION_2026-06-26.md`。

### 3.10 File Malware Scan Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/file-scan.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-FILE-SCAN-001 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-FILE-SCAN-002 | Regression user registration | 新用户注册成功，返回 JWT 和 user | Pass |
| TC-REG-FILE-SCAN-003 | Clean file scan metadata | clean upload 返回 `scan_status=clean` 和 `scan_scanner=local-signature-v1` | Pass |
| TC-REG-FILE-SCAN-004 | EICAR blocking | EICAR 测试签名返回 HTTP 400 `policy_violation` | Pass |
| TC-REG-FILE-SCAN-005 | Blocked file persistence | 被阻断文件不写入 `uploaded_files` | Pass |
| TC-REG-FILE-SCAN-006 | Blocked upload audit | 写入 `file.upload_blocked` audit event | Pass |

File malware scan regression 汇总：Total 6，Passed 6，Failed 0。详见 `26_FILE_MALWARE_SCAN_REGRESSION_2026-06-26.md`。

### 3.11 Provider Health Stats Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/provider-health-stats.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-PH-STATS-001 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-PH-STATS-002~003 | Admin auth | 注册并提升 admin，登录返回 JWT | Pass |
| TC-REG-PH-STATS-004 | Controlled data setup | 插入 provider 与 3 条 request_logs | Pass |
| TC-REG-PH-STATS-005 | Provider stats API shape | `/api/admin/provider-health` 返回目标 provider row | Pass |
| TC-REG-PH-STATS-006 | 24h stats aggregation | requests=3，errors=1，error_rate=0.3333，fallback=3，avg latency=200 | Pass |

Provider health stats regression 汇总：Total 6，Passed 6，Failed 0。详见 `27_PROVIDER_HEALTH_STATS_REGRESSION_2026-06-26.md`。

### 3.12 Workflow Webhook Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/workflow-webhook.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-WF-WEBHOOK-001 | Webhook schema | `webhook_deliveries` table 可用 | Pass |
| TC-REG-WF-WEBHOOK-002~004 | Health/auth/workflow setup | health ok，注册成功，workflow 创建成功 | Pass |
| TC-REG-WF-WEBHOOK-005 | Run response webhook | run response includes recorded webhook delivery | Pass |
| TC-REG-WF-WEBHOOK-006 | Run detail webhook | run detail includes `workflow.run.completed` delivery | Pass |
| TC-REG-WF-WEBHOOK-007 | DB persistence | delivery row persisted in DB | Pass |
| TC-REG-WF-WEBHOOK-008 | Invalid callback URL | 非 HTTP/HTTPS URL 返回 HTTP 400 `invalid_request` | Pass |

Workflow webhook regression 汇总：Total 8，Passed 8，Failed 0。详见 `28_WORKFLOW_WEBHOOK_REGRESSION_2026-06-26.md`。

### 3.13 Tool Credentials Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/tool-credentials.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-TOOL-CRED-001 | Tool credentials schema | `tool_credentials` table 可用 | Pass |
| TC-REG-TOOL-CRED-002~004 | Health/auth/tool setup | health ok，注册成功，enabled `echo` tool 可用 | Pass |
| TC-REG-TOOL-CRED-005 | Create credential | HTTP 201，response 不包含 plaintext secret | Pass |
| TC-REG-TOOL-CRED-006 | List credentials | list API 只返回 masked secret | Pass |
| TC-REG-TOOL-CRED-007 | DB persistence | `secret_encrypted` 与 `secret_mask` 持久化，mask 不等于明文 | Pass |
| TC-REG-TOOL-CRED-008 | Revoke credential | status 更新为 `revoked` | Pass |
| TC-REG-TOOL-CRED-009 | Invalid input | 缺少 secret 返回 HTTP 400 `invalid_request` | Pass |

Tool credentials regression 汇总：Total 9，Passed 9，Failed 0。详见 `29_TOOL_CREDENTIALS_REGRESSION_2026-06-26.md`。

### 3.14 Agent Sessions Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/agent-sessions.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-AGENT-SESSION-001 | Agent sessions schema | `agent_sessions` table 和 `workflow_runs.agent_session_id` 可用 | Pass |
| TC-REG-AGENT-SESSION-002~004 | Health/auth/workflow setup | health ok，注册成功，workflow 创建成功 | Pass |
| TC-REG-AGENT-SESSION-005 | Create session | HTTP 201，session active 并绑定 workflow | Pass |
| TC-REG-AGENT-SESSION-006 | List sessions | list API 返回已创建 session | Pass |
| TC-REG-AGENT-SESSION-007 | Run with session | workflow run completed 且包含 `agent_session_id` | Pass |
| TC-REG-AGENT-SESSION-008 | Session last run | session detail 返回 `last_run_id` 和 `last_activity_at` | Pass |
| TC-REG-AGENT-SESSION-009 | DB persistence | `workflow_runs.agent_session_id` 持久化 | Pass |
| TC-REG-AGENT-SESSION-010~011 | Close session guard | close 后 status=`closed`；closed session 运行被拒绝 | Pass |

Agent sessions regression 汇总：Total 11，Passed 11，Failed 0。详见 `30_AGENT_SESSIONS_REGRESSION_2026-06-26.md`。

### 3.15 Prompt Templates Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/prompt-templates.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-PROMPT-TPL-001 | Prompt templates schema | `prompt_templates` table 可用 | Pass |
| TC-REG-PROMPT-TPL-002~003 | Health/auth setup | health ok，注册成功 | Pass |
| TC-REG-PROMPT-TPL-004 | Create template | HTTP 201，template active，variables persisted | Pass |
| TC-REG-PROMPT-TPL-005~006 | List/detail | list 和 detail 返回目标 template | Pass |
| TC-REG-PROMPT-TPL-007~008 | Prompt workflow | 使用 template 创建 prompt workflow，并运行 completed | Pass |
| TC-REG-PROMPT-TPL-009 | DB persistence | template row 和 variables 持久化 | Pass |
| TC-REG-PROMPT-TPL-010 | Archive template | status 更新为 `archived` | Pass |
| TC-REG-PROMPT-TPL-011 | Invalid input | 缺少 template 返回 HTTP 400 `invalid_request` | Pass |

Prompt templates regression 汇总：Total 11，Passed 11，Failed 0。详见 `31_PROMPT_TEMPLATES_REGRESSION_2026-06-26.md`。

### 3.16 Pricing History Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/pricing-history.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-PRICE-HIST-001 | Pricing history schema | `model_pricing_history` table 可用 | Pass |
| TC-REG-PRICE-HIST-002~005 | Health/admin auth setup | health ok，注册并提升 admin，登录成功 | Pass |
| TC-REG-PRICE-HIST-006 | Create model pricing | 创建模型并写入初始 input/output price | Pass |
| TC-REG-PRICE-HIST-007 | Create history | `change_type=create` history row 写入 DB | Pass |
| TC-REG-PRICE-HIST-008 | Update model pricing | input/output price 更新成功 | Pass |
| TC-REG-PRICE-HIST-009 | Pricing history API | 返回 old/new price update row | Pass |
| TC-REG-PRICE-HIST-010 | Model detail history | model detail includes `pricing_history` | Pass |
| TC-REG-PRICE-HIST-011 | DB persistence | update history row 持久化 | Pass |

Pricing history regression 汇总：Total 11，Passed 11，Failed 0。详见 `32_PRICING_HISTORY_REGRESSION_2026-06-26.md`。

### 3.17 Workspace Cost Attribution Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/workspace-cost-attribution.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-WS-COST-001~003 | Health/admin/workspace setup | health ok，注册并提升 admin，创建 organization/workspace | Pass |
| TC-REG-WS-COST-004 | Seed request logs | workspace 下插入 3 条可控 request_logs | Pass |
| TC-REG-WS-COST-005 | Workspace total summary | 返回 3 requests、900 tokens、$0.18 cost | Pass |
| TC-REG-WS-COST-006 | Model attribution | `by_model` 包含 `qwen-plus` 2 requests / $0.09 | Pass |
| TC-REG-WS-COST-007 | Provider attribution | `by_provider` 包含 `bailian_intl` 2 requests / $0.09 | Pass |
| TC-REG-WS-COST-008 | User attribution | `by_user` 包含 regression admin 3 requests / $0.18 | Pass |
| TC-REG-WS-COST-009 | CSV compatibility | workspace usage CSV 仍包含 seeded request rows | Pass |

Workspace cost attribution regression 汇总：Total 12，Passed 12，Failed 0。详见 `33_WORKSPACE_COST_ATTRIBUTION_REGRESSION_2026-06-26.md`。

### 3.18 Provider Credentials Regression Script

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/provider-credentials.sh
```

| 用例 | 测试目标 | 预期效果 | 测试结果 |
|---|---|---|---|
| TC-REG-PROVIDER-CRED-001~002 | Health/admin setup | health ok，注册并提升 admin，登录成功 | Pass |
| TC-REG-PROVIDER-CRED-003 | Create provider | Admin 创建 OpenAI-compatible provider | Pass |
| TC-REG-PROVIDER-CRED-004 | Create credential | 创建 provider key，response 只返回 masked key | Pass |
| TC-REG-PROVIDER-CRED-005 | List credential | list 不泄露明文 secret | Pass |
| TC-REG-PROVIDER-CRED-006 | DB persistence | `provider_keys.key_ref` 为 `local:v1:*`，不是明文 | Pass |
| TC-REG-PROVIDER-CRED-007~008 | Revoke | revoke API 成功，DB `is_active=false` | Pass |
| TC-REG-PROVIDER-CRED-009 | Audit | `provider_key.create` 和 `provider_key.revoke` audit events 已记录 | Pass |

Provider credentials regression 汇总：Total 11，Passed 11，Failed 0。详见 `34_PROVIDER_CREDENTIALS_REGRESSION_2026-06-26.md`。

---

## 4. 本轮发现并修复的问题

| 问题 | 影响 | 修复 |
|---|---|---|
| `POST /api/admin/inference/nodes` 传入非法 `status=active` 时返回 500 | 非法客户端输入被表现为内部错误，不利于 API 调试和回归测试 | 已在 backend gateway 增加 cluster/node/deployment 枚举校验，非法值返回 HTTP 400 |
| private inference node 合法创建路径未在脚本中覆盖 | v1.0 private inference 回归覆盖不足 | 已补测 `status=healthy` 创建路径，HTTP 201 |

涉及代码：`backend/internal/gateway/router.go`

---

## 5. 未完全覆盖 / 需后续优化

| 项目 | 当前状态 | 建议 |
|---|---|---|
| Qwen chat 真实 provider 调用 | 2026-06-26 已补充定向 smoke：`SMOKE_MODEL_ID=qwen3.7-max`，chat HTTP 200，usage/billing/balance 均正常 | 详见 `17_TARGETED_BAILIAN_QWEN_SMOKE_2026-06-26.md` |
| Real image/video async | 本轮未开启真实 provider async，避免产生外部调用成本 | 后续用独立开关和小样本 prompt 执行 |
| UI 点击级 E2E | 2026-06-26 已补充 in-app Browser smoke：login/register、Dashboard、Request Logs、Files、Workflows、Playground、mobile Files、Admin Overview；并修复 Dashboard mobile sidebar clipping 与 logout/profile 缺失 | 详见 `19_UI_BROWSER_SMOKE_RESULTS_2026-06-26.md` |
| Admin Workspaces 深度 E2E | 2026-06-26 已补充 in-app Browser E2E：create org/workspace、budget、quota、member role/status；CSV export API 返回 text/csv | 详见 `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| Admin Foundation regression | 2026-06-26 已新增并执行 `scripts/regression/admin-foundation.sh`：Guardrails、Benchmark、Inference、Audit export/retention、Alert ack/resolve 共 24 pass / 0 fail | 详见 `21_ADMIN_FOUNDATION_REGRESSION_2026-06-26.md` |
| User Marketplace + Workflow regression | 2026-06-26 已新增并执行 `scripts/regression/user-marketplace-workflow.sh`：JWT missing-token rejection、Marketplace list/filter/detail/compare、Workflow create/run/list/detail/traces 共 18 pass / 0 fail | 详见 `22_USER_MARKETPLACE_WORKFLOW_REGRESSION_2026-06-26.md` |
| Guardrails policy regression | 2026-06-26 已新增并执行 `scripts/regression/guardrails-policy.sh`：PII block、prompt injection block、guardrail_results、audit、pii_detections、policy_violations 共 14 pass / 0 fail | 详见 `23_GUARDRAILS_POLICY_REGRESSION_2026-06-26.md` |
| Provider fallback smoke | 2026-06-26 已补充执行：`RUN_FALLBACK_SMOKE=true`，24 pass / 0 skip / 0 fail，`fallback_logs` 已确认写入 | 详见 `18_FALLBACK_SMOKE_RESULTS_2026-06-26.md` |
| 注册接口唯一约束错误信息 | 2026-06-26 已修复并新增 `scripts/regression/auth-conflict.sh`：重复注册返回稳定 409 conflict，不暴露数据库细节，共 4 pass / 0 fail | 详见 `24_AUTH_CONFLICT_REGRESSION_2026-06-26.md` |
| Audio STT/TTS mock gateway | 2026-06-26 已新增并执行 `scripts/regression/audio-mock.sh`：mock provider routing、STT、TTS、request_logs、usage_logs 共 7 pass / 0 fail；真实 DashScope audio adapter 仍待按百炼音频 API 映射 | 详见 `25_AUDIO_MOCK_REGRESSION_2026-06-26.md` |
| File malware scan baseline | 2026-06-26 已新增并执行 `scripts/regression/file-scan.sh`：clean upload scan metadata、EICAR block、blocked file not persisted、audit event 共 6 pass / 0 fail | 详见 `26_FILE_MALWARE_SCAN_REGRESSION_2026-06-26.md` |
| Provider health 24h stats | 2026-06-26 已新增并执行 `scripts/regression/provider-health-stats.sh`：request/error/fallback/latency 聚合共 6 pass / 0 fail | 详见 `27_PROVIDER_HEALTH_STATS_REGRESSION_2026-06-26.md` |
| Workflow webhook callback baseline | 2026-06-26 已新增并执行 `scripts/regression/workflow-webhook.sh`：callback URL 校验、run response/detail webhook、DB persistence 共 8 pass / 0 fail；真实 HTTP delivery worker/retry/signature 仍待增强 | 详见 `28_WORKFLOW_WEBHOOK_REGRESSION_2026-06-26.md` |
| Tool credentials baseline | 2026-06-26 已新增并执行 `scripts/regression/tool-credentials.sh`：create/list mask、DB encrypted persistence、revoke、invalid input 共 9 pass / 0 fail；生产 KMS/Vault、rotation、usage audit 仍待增强 | 详见 `29_TOOL_CREDENTIALS_REGRESSION_2026-06-26.md` |
| Agent sessions baseline | 2026-06-26 已新增并执行 `scripts/regression/agent-sessions.sh`：create/list/get/close、workflow run binding、last_run tracking、DB persistence 共 11 pass / 0 fail；session memory/context summarization 仍待增强 | 详见 `30_AGENT_SESSIONS_REGRESSION_2026-06-26.md` |
| Prompt templates baseline | 2026-06-26 已新增并执行 `scripts/regression/prompt-templates.sh`：create/list/get/archive、prompt workflow creation/run、DB persistence 共 11 pass / 0 fail；变量 schema、版本管理、workspace library 仍待增强 | 详见 `31_PROMPT_TEMPLATES_REGRESSION_2026-06-26.md` |
| Pricing history baseline | 2026-06-26 已新增并执行 `scripts/regression/pricing-history.sh`：model create/update pricing history、Admin detail/history API、DB persistence 共 11 pass / 0 fail；未来价格计划、回滚、provider multiplier history 仍待增强 | 详见 `32_PRICING_HISTORY_REGRESSION_2026-06-26.md` |
| Workspace cost attribution baseline | 2026-06-26 已新增并执行 `scripts/regression/workspace-cost-attribution.sh`：workspace total usage、model/provider/user Top 成本归因、CSV compatibility 共 12 pass / 0 fail；project cost center 与 invoice/PO baseline 已在后续回归补齐，时间窗口筛选仍待增强 | 详见 `33_WORKSPACE_COST_ATTRIBUTION_REGRESSION_2026-06-26.md` |
| Provider credentials baseline | 2026-06-26 已新增并执行 `scripts/regression/provider-credentials.sh`：Admin provider create、masked credential create/list、sealed DB persistence、revoke、audit 共 11 pass / 0 fail；scoped BYOK、secret metadata、Anthropic native adapter 已在后续回归补齐，KMS/Vault envelope 仍待增强 | 详见 `34_PROVIDER_CREDENTIALS_REGRESSION_2026-06-26.md` |
| Project cost centers baseline | 2026-06-26 已新增并执行 `scripts/regression/project-cost-centers.sh`：project create/list、workspace owner membership、project-bound API key create/store/authenticate、Top Projects attribution、CSV `project_id` export/filter 共 16 pass / 0 fail；project-level budget/quota、project RBAC、invoice grouping 仍待增强 | 详见 `36_PROJECT_COST_CENTERS_REGRESSION_2026-06-26.md` |
| Smart Routing baseline | 2026-06-26 已新增并执行 `scripts/regression/smart-routing.sh`：routing policy migration、Admin policy create/list、mock provider setup、latency stats seeding、chat completion、低延迟 provider 优先选择 共 16 pass / 0 fail；A/B split、sticky bucketing、provider-level quality score 仍待增强 | 详见 `37_SMART_ROUTING_REGRESSION_2026-06-26.md` |
| Invoice / PO / postpaid baseline | 2026-06-27 已新增并执行 `scripts/regression/invoices-po-postpaid.sh`：postpaid organization terms、default PO、workspace invoice draft、subtotal/total/due date calculation、invoice list、DB persistence、audit 共 12 pass / 0 fail；invoice CSV export、PDF export 与 status workflow 已在后续回归补齐，monthly statement/regional tax rules 仍待增强 | 详见 `38_INVOICES_PO_POSTPAID_REGRESSION_2026-06-27.md` |
| Scoped BYOK baseline | 2026-06-27 已新增并执行 `scripts/regression/scoped-byok.sh`：platform/user/workspace provider keys、masked list、no plaintext leakage、workspace credential priority、workspace API key route、request attribution、audit 共 19 pass / 0 fail；provider credential validation 与 user BYOK self-service 已在后续回归补齐 | 详见 `39_SCOPED_BYOK_REGRESSION_2026-06-27.md` |
| Provider onboarding templates | 2026-06-27 已新增并执行 `scripts/regression/provider-onboarding-templates.sh`：OpenAI/Grok/Anthropic template list/install、provider/model/binding persistence、catalog availability、audit 共 19 pass / 0 fail；真实 upstream smoke 仍待有效 provider key | 详见 `40_PROVIDER_ONBOARDING_TEMPLATES_REGRESSION_2026-06-27.md` |
| Anthropic native adapter | 2026-06-27 已新增并执行 `scripts/regression/anthropic-native-adapter.sh`：native `/messages` request、`x-api-key`、`anthropic-version`、system mapping、OpenAI-compatible response mapping、request_logs、usage_logs 共 13 pass / 0 fail；tool-use mapping、native streaming refinement、真实 upstream smoke 仍待增强 | 详见 `43_ANTHROPIC_NATIVE_ADAPTER_REGRESSION_2026-06-27.md` |
| Workflow webhook worker | 2026-06-27 已新增并执行 `scripts/regression/workflow-webhook-worker.sh`：真实 HTTP callback delivery、HMAC signature、delivery headers、DB `delivered` 状态、response status、attempt count、run detail metadata 共 8 pass / 0 fail；persistent queue、dead-letter/replay UI、per-workspace webhook secrets 仍待增强 | 详见 `41_WORKFLOW_WEBHOOK_WORKER_REGRESSION_2026-06-27.md` |
| Secret management metadata | 2026-06-27 已新增并执行 `scripts/regression/secret-management-metadata.sh`：provider/tool credential seal metadata、provider key last_used tracking、safe Admin metadata list、revoked_at、tool credential metadata 共 19 pass / 0 fail；KMS/Vault envelope、rotation UI/API、usage audit stream 仍待增强 | 详见 `42_SECRET_MANAGEMENT_METADATA_REGRESSION_2026-06-27.md` |
| Provider credential validation | 2026-06-27 已新增并执行 `scripts/regression/provider-credential-validation.sh`：mock provider key validation、Anthropic fake upstream `/messages` validation、provider health persistence、audit event persistence 共 9 pass / 0 fail；真实 OpenAI/Grok/Anthropic/Bailian upstream smoke 仍待有效 provider key | 详见 `44_PROVIDER_CREDENTIAL_VALIDATION_REGRESSION_2026-06-27.md` |
| Request log credential scope + OpenAI/Grok validation | 2026-06-27 已新增并执行 `scripts/regression/request-log-credential-scope.sh` 与 `scripts/regression/openai-grok-provider-validation.sh`：request_logs 非敏感 credential scope/key id 标注、API/CSV/UI 暴露、OpenAI/Grok fake upstream `/models` validation、Bearer key header、health/audit persistence 共 29 pass / 0 fail；真实 upstream smoke 仍待有效 provider key | 详见 `45_PROVIDER_REQUEST_LOG_CREDENTIAL_SCOPE_AND_OPENAI_GROK_VALIDATION_2026-06-27.md` |
| User BYOK self-service | 2026-06-27 已新增并执行 `scripts/regression/user-byok-self-service.sh`：普通用户 provider list、user-scoped provider key create/list/revoke、API/DB no plaintext leakage、ownership、inactive revoke state、audit events 共 13 pass / 0 fail；真实 upstream smoke 仍待有效 provider key | 详见 `46_USER_BYOK_SELF_SERVICE_REGRESSION_2026-06-27.md` |
| Invoice status / export | 2026-06-27 已新增并执行 `scripts/regression/invoice-status-export.sh`：invoice draft -> issued -> paid status workflow、invalid status stable error、invoice CSV export、invoice PDF export、DB persistence、audit events 共 18 pass / 0 fail；monthly statement/regional tax rules 仍待增强 | 详见 `47_INVOICE_STATUS_EXPORT_REGRESSION_2026-06-27.md` |
| DashScope audio chain | 2026-06-27 已新增并执行 `scripts/regression/dashscope-audio-chain.sh`：fake DashScope `/models` health、TTS、ASR、Bearer provider key forwarding、request_logs credential scope、usage_logs 共 11 pass / 0 fail；Playground 已支持 TTS 播放，ASR 当前为 multipart API-first；真实百炼 ASR 如要求公网文件 URL，后续接对象存储中转 | 详见 `48_DASHSCOPE_AUDIO_CHAIN_REGRESSION_2026-06-27.md` |

---

## 6. 结论

按照当前文档中已完成功能的自动化可执行范围，本轮测试通过。平台核心闭环、企业管理面、FinOps、benchmark、guardrails、audit、workflow、files、private inference foundation 均完成可运行验证。

真实百炼 embedding 已确认可用；真实 qwen chat 已于 2026-06-26 通过 `SMOKE_MODEL_ID=qwen3.7-max` 完成定向验收。DashScope audio provider 链路已通过 fake upstream 回归验证，真实 ASR/TTS upstream smoke 需在确认成本和账号模型权限后执行。
