# Completed Feature Test Cases

文档名称：`15_COMPLETED_FEATURE_TEST_CASES.md`
项目名称：AI Aggregator Platform
最后更新日期：2026-06-26
维护人：Project Owner / Codex

---

## 0. 最新执行结果

实际执行记录见：

```text
ai_aggregator_codex_pack/16_TEST_EXECUTION_RESULTS_2026-06-25.md
ai_aggregator_codex_pack/17_TARGETED_BAILIAN_QWEN_SMOKE_2026-06-26.md
ai_aggregator_codex_pack/18_FALLBACK_SMOKE_RESULTS_2026-06-26.md
ai_aggregator_codex_pack/19_UI_BROWSER_SMOKE_RESULTS_2026-06-26.md
```

本轮执行摘要：

```text
backend go test ./... = Pass
frontend npm run build = Pass
scripts/dev-check.sh = Pass
mock full smoke = 23 pass / 1 skip / 0 fail
real provider smoke = 18 pass / 4 skip / 0 fail
enterprise/admin/private inference API regression = 38 pass / 0 fail
targeted real Bailian qwen3.7-max smoke = 19 pass / 3 skip / 0 fail
provider fallback smoke = 24 pass / 0 skip / 0 fail
UI browser smoke = Pass with mobile layout fix + logout/profile/admin overview verification
```

---

## 1. 文档目标

本文档把当前已经完成或已经完成基础闭环的功能，整理为可执行、可回归、可验收的测试用例。

每个测试用例包含：

```text
测试编号
覆盖功能
测试目标
前置条件
测试方法
预期效果
当前测试结果
是否符合测试标准
证据来源
```

本文件用于：

```text
1. 发布前回归测试
2. 新功能开发后的影响面检查
3. 本地开发环境验收
4. 后续 CI 自动化测试拆分依据
5. 给 QoderWork / Codex / Manual 开发同步测试标准
```

---

## 2. 当前验证基线

### 2.1 本轮已执行基础验证

| 验证项 | 命令 | 当前结果 | 是否达标 |
|---|---|---:|---:|
| Go 后端编译测试 | `source scripts/dev-env.sh && cd backend && go test ./...` | Pass | 是 |
| 前端 TypeScript + Vite build | `source scripts/dev-env.sh && cd frontend && npm run build` | Pass | 是 |
| 本地服务 / 工具链 / Helm 渲染 | `source scripts/dev-env.sh && scripts/dev-check.sh` | Pass | 是 |

### 2.2 本地服务状态

| 服务 | 地址 | 当前状态 |
|---|---|---:|
| Backend | `http://localhost:8081` | Running |
| Frontend | `http://localhost:5175` | Running |
| Postgres | `localhost:5433` | Healthy |
| Redis | `localhost:6380` | Healthy |

### 2.3 推荐回归命令

```bash
source scripts/dev-env.sh
cd backend && go test ./...

cd ../frontend
npm run build

cd ..
scripts/dev-check.sh
MOCK_PROVIDER_MODE=true bash scripts/smoke-test.sh
```

可选增强回归：

```bash
MOCK_PROVIDER_MODE=true RUN_FALLBACK_SMOKE=true bash scripts/smoke-test.sh
MOCK_PROVIDER_MODE=true RUN_EMBEDDING_SMOKE=true RUN_ASYNC_SMOKE=true bash scripts/smoke-test.sh
```

真实 provider 回归应单独执行，避免把上游模型映射问题误判为本地功能失败：

```bash
DASHSCOPE_API_KEY_INTL=<redacted> MOCK_PROVIDER_MODE=false RUN_EMBEDDING_SMOKE=true bash scripts/smoke-test.sh
```

---

## 3. 测试结果状态定义

| 状态 | 含义 |
|---|---|
| Pass | 本轮或最近一次明确执行通过 |
| Pass - Documented Smoke | 已有明确 smoke/API 验证记录，当前未重复执行完整链路 |
| Pass - Build Verified | 通过编译/构建和当前服务健康验证，适合 UI/API client 改动 |
| Conditional Pass | Mock/local 通过；真实 provider 依赖上游模型映射或外部配置 |
| Not Run | 已定义测试用例，但本轮未执行 |
| Fail | 测试不符合预期，需要修复 |

---

## 4. 功能测试用例矩阵

### 4.1 平台启动与基础健康

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-CORE-001 | 本地 Docker Compose 服务 | 验证 backend/frontend/postgres/redis 可运行 | Docker Desktop 可用 | `scripts/dev-check.sh` | 服务全部 running/healthy | Pass | 是 | 本轮执行 |
| TC-CORE-002 | Backend health | 验证 backend 健康检查 | backend 运行 | `curl -fsS http://localhost:8081/health` | 返回 `{"status":"ok"}` | Pass | 是 | `dev-check.sh` |
| TC-CORE-003 | Frontend availability | 验证前端页面可访问 | frontend 运行 | `curl -fsS http://localhost:5175` | HTTP 200 | Pass | 是 | `dev-check.sh` |
| TC-CORE-004 | Helm chart 渲染 | 验证私有部署 chart 基础可渲染 | Helm 可用 | `helm template ai-aggregator deploy/helm/ai-aggregator ...` | 渲染无错误 | Pass | 是 | `dev-check.sh` |

### 4.2 用户注册、登录、Profile

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-AUTH-001 | 用户注册 | 新用户可创建账号 | backend 运行 | `POST /api/user/auth/register` | HTTP 201，返回 JWT 和 user | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-AUTH-002 | 用户登录 | 已注册用户可登录 | 已有用户 | `POST /api/user/auth/login` | HTTP 200，返回 JWT | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-AUTH-003 | JWT 鉴权 | 未带 token 时拒绝用户接口 | backend 运行 | 无 Authorization 调用 `/api/user/dashboard` | HTTP 401，`authentication_error` | Pass - Regression Script | 是 | `scripts/regression/user-marketplace-workflow.sh` |
| TC-AUTH-004 | 用户 Profile 读取 | 当前用户可读取 profile | 已登录 | `GET /api/user/profile` | 返回 email/username/role | Pass - Documented Smoke | 是 | `PROJECT_STAGE_TRACKER.md` |
| TC-AUTH-005 | 用户 Profile 更新 | 当前用户可更新 username/metadata | 已登录 | `PUT /api/user/profile` | HTTP 200，返回更新后 profile | Pass - Documented Smoke | 是 | `PROJECT_STAGE_TRACKER.md` |
| TC-AUTH-006 | 注册冲突错误 | 重复注册不暴露数据库 constraint 细节 | backend 运行 | 重复调用 `POST /api/user/auth/register` | HTTP 409，`code=conflict`，`type=client_error`，稳定 message | Pass - Regression Script | 是 | `scripts/regression/auth-conflict.sh` |

### 4.3 API Key 生命周期与权限

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-KEY-001 | 创建 API Key | 用户可创建 key，明文只返回一次 | 已登录 | `POST /api/user/keys` | HTTP 201，返回 `sk-aag-...` | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-KEY-002 | 列出 API Key | 用户可查看自己的 key metadata | 已创建 key | `GET /api/user/keys` | 列表包含 key id/prefix | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-KEY-003 | 撤销 API Key | 用户可撤销自己的 key | 已创建 key | `DELETE /api/user/keys/:id` | HTTP 200/204，key inactive | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-KEY-004 | 撤销后不可调用 | 已撤销 key 不能调用 `/v1` | key 已撤销 | 用 revoked key 调 `/v1/chat/completions` | HTTP 401 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-KEY-005 | Workspace key 权限 | viewer 不可创建 workspace key | workspace + viewer member | `POST /api/user/keys` with `workspace_id` | HTTP 403 `permission_error` | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-KEY-006 | Workspace key 允许路径 | member 可创建 workspace key | workspace + member | `POST /api/user/keys` with `workspace_id` | HTTP 201 | Pass - Documented Smoke | 是 | RBAC smoke |

### 4.4 OpenAI-compatible Chat / Models / Billing 主链路

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-GW-001 | `/v1/models` | API key 可获取模型列表 | 有 active API key | `GET /v1/models` | HTTP 200，`data` 非空 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-GW-002 | Chat non-stream | 可完成非流式模型调用 | 有 active API key | `POST /v1/chat/completions` | HTTP 200，返回 choices/usage | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-GW-003 | Chat stream | 可完成 SSE 流式调用 | 有 active API key | `POST /v1/chat/completions` with `stream=true` | HTTP 200，返回 SSE chunk | Pass - Documented Smoke | 是 | `PROJECT_STAGE_TRACKER.md` |
| TC-GW-004 | Balance 扣费 | 调用后余额减少 | 初始余额 > 0 | 调用前后 `GET /api/user/billing/balance` | 后余额小于前余额 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-GW-005 | Usage logs | 调用后写 usage_logs | 完成一次调用 | `GET /api/user/usage` | 存在对应模型/成本/token | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-GW-006 | Billing transactions | 调用后写 usage_charge | 完成一次调用 | `GET /api/user/billing/transactions` | 包含 `credit_grant` 和 `usage_charge` | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-GW-007 | Rate limit RPM | 超过 RPM 后限流 | key 设置低 RPM | 连续调用 `/v1/chat/completions` | HTTP 429 `rate_limit_error` | Pass - Documented Smoke | 是 | Admin key smoke |
| TC-GW-008 | Rate limit TPM | 超过 TPM 后限流 | key 设置低 TPM | 调用超 token 请求 | HTTP 429 `tpm_limit` | Pass - Documented Smoke | 是 | TPM smoke |

### 4.5 Embeddings / Image / Video / Audio / Files

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-MM-001 | Embeddings | 可生成 embedding 并记录用量 | mock 或真实 provider | `POST /v1/embeddings` | HTTP 200，embedding 非空 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-MM-002 | Image async submit | 可提交图片异步任务 | mock 或真实 provider | `POST /v1/images/generations` | HTTP 202/200，返回 task id | Conditional Pass | 是 | mock smoke；真实映射待校准 |
| TC-MM-003 | Image async poll | 可查询图片任务状态 | 已有 image task | `GET /v1/images/generations/:id` | 返回 pending/processing/completed/failed | Conditional Pass | 是 | mock smoke；真实映射待校准 |
| TC-MM-004 | Video async submit | 可提交视频异步任务 | mock 或真实 provider | `POST /v1/video/generations` | HTTP 202/200，返回 task id | Conditional Pass | 是 | mock smoke；真实映射待校准 |
| TC-MM-005 | Video async poll | 可查询视频任务状态 | 已有 video task | `GET /v1/video/generations/:id` | 返回状态和结果 | Conditional Pass | 是 | mock smoke；真实映射待校准 |
| TC-MM-006 | Audio transcription | 网关具备 STT endpoint | mock provider binding 可用 | `POST /v1/audio/transcriptions` | HTTP 200，返回 mock transcription，写入 request_logs/usage_logs | Pass - Regression Script | 是 | `scripts/regression/audio-mock.sh`；DashScope audio adapter 待映射 |
| TC-MM-007 | Audio speech | 网关具备 TTS endpoint | mock provider binding 可用 | `POST /v1/audio/speech` | HTTP 200，返回 audio bytes，写入 request_logs/usage_logs | Pass - Regression Script | 是 | `scripts/regression/audio-mock.sh`；DashScope audio adapter 待映射 |
| TC-FILE-001 | File upload | 用户可上传允许 MIME 文件 | active key/JWT | `POST /v1/files` multipart | HTTP 201，返回 file id/sha256 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-FILE-002 | File list/detail | 用户可查看自己的文件 | 已上传文件 | `GET /v1/files` 和 `GET /v1/files/:id` | 返回文件 metadata | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-FILE-003 | File download | 用户可下载文件内容 | 已上传文件 | `GET /v1/files/:id/content` | HTTP 200，内容一致 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-FILE-004 | File delete | 用户可删除文件 | 已上传文件 | `DELETE /v1/files/:id` | HTTP 200，文件不可再访问 | Pass - Documented Smoke | 是 | `scripts/smoke-test.sh` |
| TC-FILE-005 | Workspace file RBAC | viewer workspace key 不可上传 | viewer workspace key | `POST /v1/files` | HTTP 403 `permission_error` | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-FILE-006 | File retention cleanup | Admin 可按 retention 清理文件 | admin JWT | `POST /api/admin/files/retention/run` | 返回 matched/deleted 计数 | Pass - Documented Smoke | 是 | tracker |
| TC-FILE-007 | File malware scan | EICAR 测试签名上传被阻断 | JWT | `POST /v1/files` 上传 EICAR 测试文件 | HTTP 400 `policy_violation`，不写入 `uploaded_files`，写入 `file.upload_blocked` audit | Pass - Regression Script | 是 | `scripts/regression/file-scan.sh` |

### 4.6 Observability / Request Logs / Provider Health / Fallback

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-OBS-001 | Request logs 写入 | 每次请求写 request_logs | 完成模型调用 | `GET /api/user/request-logs` | 列表出现最新 request | Pass - Documented Smoke | 是 | fallback/request logs smoke |
| TC-OBS-002 | Request log detail | 可查看单条请求详情 | 已有 request_id | `GET /api/user/request-logs/:request_id` | 返回 preview/provider/latency | Pass - Documented Smoke | 是 | tracker |
| TC-OBS-003 | Request log filters | 支持 model/status/provider/date | 已有日志 | 带 query 调 `/api/user/request-logs` | 返回匹配过滤结果 | Pass - Documented Smoke | 是 | tracker |
| TC-OBS-004 | Request log CSV | 支持 CSV export | 已有日志 | `GET /api/user/request-logs?format=csv` | `text/csv` 下载 | Pass - Documented Smoke | 是 | tracker |
| TC-OBS-005 | Provider health list | Admin 可查看 provider health | admin JWT | `GET /api/admin/provider-health` | 返回 provider health rows | Pass - Documented Smoke | 是 | dev/admin smoke |
| TC-OBS-006 | Provider health history | Admin 可看历史 | admin JWT | `GET /api/admin/providers/:id/health/history` | 返回历史记录 | Pass - Documented Smoke | 是 | tracker |
| TC-OBS-007 | Manual health check | Admin 可手动检查 provider | admin JWT | `POST /api/admin/providers/:id/health-check` | 写入 health check | Pass - Documented Smoke | 是 | tracker |
| TC-OBS-008 | Fallback chain | 主 provider 失败后 fallback | mock failure mode | `RUN_FALLBACK_SMOKE=true bash scripts/smoke-test.sh` | 调用成功且 fallback_count > 0 | Pass - Executed Smoke | 是 | `18_FALLBACK_SMOKE_RESULTS_2026-06-26.md` |
| TC-OBS-009 | Provider 24h stats | Admin 可查看 provider 请求量/错误率/fallback 聚合 | admin JWT + request_logs | `GET /api/admin/provider-health` | 返回 `request_count_24h`、`error_rate_24h`、`fallback_count_24h` | Pass - Regression Script | 是 | `scripts/regression/provider-health-stats.sh` |

### 4.7 Admin Models / Providers / Pricing

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-ADMIN-MODEL-001 | Admin model list | Admin 可查看模型 | admin JWT | `GET /api/admin/models` | HTTP 200，data 非空 | Pass - Documented Smoke | 是 | Admin UI/API smoke |
| TC-ADMIN-MODEL-002 | Admin model update | 可更新模型价格/状态 | admin JWT | `PUT /api/admin/models/:id` | 返回更新后 model | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-MODEL-003 | Model provider bind | 可绑定 provider | admin JWT | `POST /api/admin/models/:id/providers` | 返回 binding | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-MODEL-004 | Model provider update | 可更新 priority/upstream_model | admin JWT | `PUT /api/admin/models/:id/providers/:pid` | 返回更新后 binding | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-MODEL-005 | Model provider delete | 可删除 binding | admin JWT | `DELETE /api/admin/models/:id/providers/:pid` | 返回 deleted | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-PROV-001 | Provider CRUD | 可 list/create/update provider | admin JWT | `/api/admin/providers` GET/POST/PUT | 数据持久化且路由刷新 | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-PRICE-001 | Pricing 生效 | DB pricing 影响扣费 | 更新模型价格后调用 | 调用模型并比较 usage_charge | 扣费按新价格计算 | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-PRICE-002 | Pricing history | Admin 模型价格变更可审计 | admin JWT | `POST/PUT /api/admin/models`，`GET /api/admin/models/:id/pricing-history`，查 DB | create/update 均写入 old/new price history，model detail 返回 pricing_history | Pass - Regression Script | 是 | `scripts/regression/pricing-history.sh` |

### 4.8 Organization / Workspace / Team / RBAC / FinOps

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-ORG-001 | Create organization | Admin 可创建组织 | admin JWT | Browser `/admin/workspaces` 填写 name/slug 并点击 Create | 组织创建并显示于 workspace 创建上下文 | Pass - Browser E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| TC-WS-001 | Create workspace | Admin 可创建 workspace | 已有 organization | Browser `/admin/workspaces` 填写 name/slug/budget 并点击 Create | HTTP 创建成功，workspace 出现在列表并被选中 | Pass - Browser E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| TC-WS-002 | List workspace | Admin 可列出 workspace | admin JWT | 打开 `/admin/workspaces` | 返回并渲染 workspace list | Pass - Browser E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| TC-TEAM-001 | Add/update member | Admin 可添加或更新成员 role/status | admin JWT + user | Browser 选择 user/role/status 并点击 Add or Update Member | 成员 upsert 成功，列表显示 email、role、status | Pass - Browser E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| TC-TEAM-002 | Member UI | UI 可通过用户下拉管理成员 | frontend running | `/admin/workspaces` 选择 user dropdown、role `viewer`、status `invited` | 不需手填 user_id，成员状态正确展示 | Pass - Browser E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| TC-WS-USAGE-001 | Workspace usage | Admin 可查看 workspace usage | workspace 有请求 | `GET /api/admin/workspaces/:id/usage` | 返回 request/cost/token | Pass - Documented Smoke | 是 | tracker |
| TC-WS-USAGE-002 | Workspace usage export | Admin 可导出 usage CSV | workspace 有请求 | Browser 点击 Export Usage CSV + API export 校验 | CSV endpoint HTTP 200，返回 `text/csv` 与标准表头 | Pass - Browser/API E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |
| TC-WS-USAGE-003 | Workspace cost attribution | Admin 可按 workspace 查看 model/provider/user 成本归因 | workspace 有 request_logs | `GET /api/admin/workspaces/:id/usage` | 返回 `by_model`、`by_provider`、`by_user` Top 归因，成本/token/request 与 DB 聚合一致 | Pass - Regression Script | 是 | `scripts/regression/workspace-cost-attribution.sh` |
| TC-RBAC-001 | `api_keys:create` | viewer 被拒，member 允许 | workspace roles | 创建 workspace key | viewer 403，member 201 | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-RBAC-002 | `files:write` | viewer key 被拒，member key 允许 | workspace keys | 上传文件 | viewer 403，member 201 | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-RBAC-003 | `workflows:write` | viewer 被拒，member 允许 workflow | workspace roles | 创建/运行 workflow | viewer 403，member 201 | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-RBAC-004 | `workspace_budgets:write` | viewer 被拒，owner 允许 budget | workspace roles | `POST /api/user/workspaces/:id/budgets` | viewer 403，owner 201 | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-RBAC-005 | `workspace_quotas:write` | owner 可创建 quota，非法类型 400 | owner role | 创建 quota | 201；非法类型 400 | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-BUDGET-001 | Runtime budget enforcement | 超 monthly hard budget 拦截 | 配置 budget | 调用模型超过额度 | HTTP 429 `quota_exceeded` | Pass - Documented Smoke | 是 | tracker |
| TC-QUOTA-001 | Runtime quota enforcement | 超请求/token/spend quota 拦截 | 配置 quota | 调用触发限制 | HTTP 429 `quota_exceeded` | Pass - Documented Smoke | 是 | tracker |
| TC-BUDGET-UI-001 | Admin budget/quota UI | Admin UI 可创建和查看 budget/quota | admin JWT | Browser 创建 daily budget 和 requests_per_minute quota | 列表刷新展示 `$55.50 / daily`、soft/hard、quota type/limit、active | Pass - Browser E2E | 是 | `20_ADMIN_WORKSPACES_E2E_RESULTS_2026-06-26.md` |

### 4.9 Admin Users / API Keys / Billing / Analytics / Alerts / Audit

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-ADMIN-USER-001 | Admin user list | Admin 可列出用户 | admin JWT | `GET /api/admin/users` | 返回 users | Pass - Documented Smoke | 是 | Admin Users UI |
| TC-ADMIN-USER-002 | Admin user update | Admin 可改用户角色/状态 | admin JWT | `PUT /api/admin/users/:id` | 返回更新后 user | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-BILL-001 | Credit grant | Admin 可给用户充值 | admin JWT + user | `POST /api/admin/users/:id/balance` | balance 增加，交易写入 | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-KEY-001 | Admin key list | Admin 可查看 key metadata | admin JWT | `GET /api/admin/keys` | 返回 key list | Pass - Documented Smoke | 是 | tracker |
| TC-ADMIN-KEY-002 | Admin key limits | Admin 可更新 RPM/TPM/permissions | admin JWT | `PUT /api/admin/keys/:id` | 后续调用按限制生效 | Pass - Documented Smoke | 是 | admin key smoke |
| TC-ADMIN-KEY-003 | Admin key revoke | Admin 可 revoke 任意 key | admin JWT | `DELETE /api/admin/keys/:id` | key inactive | Pass - Documented Smoke | 是 | tracker |
| TC-ANALYTICS-001 | Analytics overview | Admin 可查看总体指标 | admin JWT | `GET /api/admin/analytics/overview` | 返回 requests/users/cost/latency | Pass - Documented Smoke | 是 | Admin Overview UI |
| TC-ANALYTICS-002 | Analytics usage/cost/latency/errors | Admin 可查看分项分析 | admin JWT | 调四个 analytics endpoint | 返回列表或聚合 | Pass - Documented Smoke | 是 | Admin Analytics UI |
| TC-ALERT-001 | Alert rule CRUD | Admin 可创建/更新 alert rule | admin JWT | GET/POST/PUT `/api/admin/alerts/rules` | 规则持久化 | Pass - Documented Smoke | 是 | alert smoke |
| TC-ALERT-002 | Alert event ack/resolve | Admin 可确认和解决告警 | 已有 alert event | POST ack/resolve | 状态变为 acknowledged/resolved | Pass - Documented Smoke | 是 | alert smoke |
| TC-AUDIT-001 | Audit list filters | Admin 可按 action/resource/time 过滤 | 已有 audit event | `GET /api/admin/audit-logs?...` | 返回目标事件 | Pass - Documented Smoke | 是 | audit smoke |
| TC-AUDIT-002 | Audit CSV export | Admin 可导出审计日志 | admin JWT | `GET /api/admin/audit-logs/export` | `text/csv` attachment | Pass - Documented Smoke | 是 | audit smoke |
| TC-AUDIT-003 | Audit retention | Admin 可 dry-run/run 清理过期审计 | 设置 retention | `POST /api/admin/audit-logs/retention/run` | dry-run 不删；run 删除 | Pass - Documented Smoke | 是 | audit retention smoke |

### 4.10 Model Marketplace / Pricing / Docs / Landing

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-MKT-001 | Marketplace model list | 公共模型市场返回列表 | backend 运行 | `GET /api/marketplace/models` | 返回 data/count | Pass - Documented Smoke | 是 | marketplace API smoke |
| TC-MKT-002 | Search/filter | 支持 q/modality 过滤 | 有模型数据 | `GET /api/marketplace/models?q=...&modality=...` | 返回匹配模型 | Pass - Documented Smoke | 是 | Models UI |
| TC-MKT-003 | Model detail | 模型详情包含价格/能力/provider | 有 model id | `GET /api/marketplace/models/:id` | 返回 model/providers | Pass - Documented Smoke | 是 | Models UI |
| TC-MKT-004 | Model compare | 可比较多个模型 | 多个 model id | `GET /api/marketplace/models/compare?ids=...` | 返回多个模型 | Pass - Documented Smoke | 是 | Models UI |
| TC-MKT-005 | Benchmark score 展示 | 模型详情展示 benchmark_score | benchmark 已运行 | Models detail panel | 显示分数或 pending | Pass - Build Verified | 是 | `Models.tsx` |
| TC-MKT-006 | Landing featured models | 首页使用真实模型 catalog | frontend running | 打开 `/` | featured models 来自 API | Pass - Documented Smoke | 是 | tracker |
| TC-MKT-007 | Pricing catalog | Pricing 页面使用真实 catalog | frontend running | 打开 `/pricing` | 按 modality 展示价格 | Pass - Documented Smoke | 是 | tracker |
| TC-MKT-008 | Docs API reference | Docs 展示当前 API 信息 | frontend running | 打开 `/docs` | 显示 local base URL 和 endpoints | Pass - Documented Smoke | 是 | tracker |

### 4.11 Benchmark / Evaluation Foundation

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-BENCH-001 | Create benchmark task | Admin 可创建 benchmark task | admin JWT | `POST /api/admin/benchmarks/tasks` | HTTP 201 | Pass - Documented Smoke | 是 | benchmark smoke |
| TC-BENCH-002 | List benchmark tasks | Admin 可查看 task 列表 | admin JWT | `GET /api/admin/benchmarks/tasks` | 返回 tasks | Pass - Documented Smoke | 是 | Admin Benchmarks UI |
| TC-BENCH-003 | Run benchmark | Admin 可运行确定性 benchmark | 已有 task | `POST /api/admin/benchmarks/tasks/:id/runs` | run completed，结果生成 | Pass - Documented Smoke | 是 | benchmark smoke |
| TC-BENCH-004 | Get benchmark run | Admin 可查看 run/result | 已有 run | `GET /api/admin/benchmarks/runs/:id` | 返回 results | Pass - Documented Smoke | 是 | Admin Benchmarks UI |
| TC-BENCH-005 | Marketplace score backfill | latest benchmark_score 进入市场 | benchmark 已运行 | `GET /api/marketplace/models` | model 含 benchmark_score | Pass - Documented Smoke | 是 | tracker |

### 4.12 Guardrails / Compliance

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-GUARD-001 | Policy list/create | Admin 可管理 guardrail policy | admin JWT | GET/POST `/api/admin/guardrails/policies` | policy 创建并返回 | Pass - Documented Smoke | 是 | Guardrails smoke |
| TC-GUARD-002 | PII masking | 请求含 PII 时 mask | 启用 policy | 调用 chat with PII | request 被 mask 或记录 | Pass - Documented Smoke | 是 | PII smoke |
| TC-GUARD-003 | Prompt injection block | 注入文本被拦截 | 启用 policy | 调用 chat with injection text | HTTP 400/403，写 result | Pass - Documented Smoke | 是 | guardrails smoke |
| TC-GUARD-004 | Guardrail results | Admin 可查看结果 | 已触发 policy | `GET /api/admin/guardrails/results` | 返回 result list | Pass - Documented Smoke | 是 | Admin Guardrails UI |
| TC-GUARD-005 | Policy violation audit | block 行为写审计/violation | 已触发 block | 查 audit/results | 有 violation 记录 | Pass - Documented Smoke | 是 | tracker |

### 4.13 Workflow / Agent Foundation

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-WF-001 | Create workflow | 用户可创建 workflow | JWT | `POST /api/user/workflows` | HTTP 201 | Pass - Documented Smoke | 是 | workflow smoke |
| TC-WF-002 | List/get workflow | 用户可查看 workflow | 已创建 workflow | GET list/detail | 返回 workflow/steps | Pass - Documented Smoke | 是 | Workflows UI |
| TC-WF-003 | Run workflow | 用户可同步运行 workflow | 已创建 workflow | `POST /api/user/workflows/:id/runs` | run completed | Pass - Documented Smoke | 是 | workflow smoke |
| TC-WF-004 | Run detail/traces | 用户可查看 run steps/traces | 已运行 workflow | GET runs/run detail | 返回 steps/traces | Pass - Documented Smoke | 是 | Workflows UI |
| TC-WF-005 | Workflow RBAC | viewer 不能创建 workspace workflow | viewer/member roles | POST workflow with workspace_id | viewer 403，member 201 | Pass - Documented Smoke | 是 | RBAC smoke |
| TC-WF-006 | Workflow billing | run 成本记录 | 运行 workflow | 查 billing/usage | 有 task-level cost | Pass - Documented Smoke | 是 | tracker |
| TC-WF-007 | Workflow webhook callback baseline | workflow run 可记录 callback delivery audit | JWT + workflow + callback_url | `POST /api/user/workflows/:id/runs` with `callback_url`，再查 run detail/DB | 生成 `workflow.run.completed` delivery，状态 `recorded`，非法 URL 返回 400 | Pass - Regression Script | 是 | `scripts/regression/workflow-webhook.sh` |
| TC-WF-008 | Tool credentials baseline | 用户可保存、查看 masked credential 并 revoke | JWT + enabled tool | `POST/GET/DELETE /api/user/tool-credentials`，查 DB | response 不暴露明文，DB 写入 encrypted/mask，revoke 后状态 `revoked` | Pass - Regression Script | 是 | `scripts/regression/tool-credentials.sh` |
| TC-WF-009 | Agent sessions baseline | 用户可创建 agent session 并关联 workflow run | JWT + workflow | `POST/GET/DELETE /api/user/agent-sessions`；`POST /api/user/workflows/:id/runs` with `agent_session_id` | run 写入 `agent_session_id`，session 更新 `last_run_id/last_activity_at`，closed session 被拒绝 | Pass - Regression Script | 是 | `scripts/regression/agent-sessions.sh` |
| TC-WF-010 | Prompt templates baseline | 用户可创建 prompt template 并用于 prompt workflow | JWT | `POST/GET/DELETE /api/user/prompt-templates`；创建 prompt workflow 并运行 | template lifecycle 可用，prompt step 使用 template，workflow run completed | Pass - Regression Script | 是 | `scripts/regression/prompt-templates.sh` |

### 4.14 Private / Self-hosted Inference / Deployment

| 测试编号 | 覆盖功能 | 测试目标 | 前置条件 | 测试方法 | 预期效果 | 当前测试结果 | 是否达标 | 证据来源 |
|---|---|---|---|---|---|---:|---:|---|
| TC-INF-001 | Create inference cluster | Admin 可注册 cluster | admin JWT | `POST /api/admin/inference/clusters` | HTTP 201 | Pass - Documented Smoke | 是 | self-hosted smoke |
| TC-INF-002 | Create inference node | Admin 可注册 node | 已有 cluster | `POST /api/admin/inference/nodes` | HTTP 201 | Pass - Documented Smoke | 是 | self-hosted smoke |
| TC-INF-003 | Create model deployment | Admin 可注册 deployment | cluster/provider/model | `POST /api/admin/inference/deployments` | provider/model binding 创建 | Pass - Documented Smoke | 是 | self-hosted smoke |
| TC-INF-004 | List inference resources | Admin UI 可查看 cluster/node/deployment | admin JWT | 打开 `/admin/inference` | 列表加载成功 | Pass - Build Verified | 是 | Admin Inference UI |
| TC-INF-005 | Unified route to self-hosted | 自托管 provider 进入统一路由 | deployment active | `/v1/chat/completions` with model | 请求路由到 OpenAI-compatible endpoint | Pass - Documented Smoke | 是 | tracker |
| TC-DEPLOY-001 | Helm package | 私有部署基础 chart 可用 | Helm 可用 | `scripts/dev-check.sh` | helm template pass | Pass | 是 | 本轮执行 |
| TC-PROVIDER-CRED-001 | Provider credential baseline | Admin 可保存 provider API key 且不泄露明文 | admin JWT + provider | `POST/GET/DELETE /api/admin/providers/:id/keys`，查 DB/audit | response 只返回 mask，DB `key_ref` sealed，revoke 后 inactive，audit 记录 create/revoke | Pass - Regression Script | 是 | `scripts/regression/provider-credentials.sh` |

---

## 5. 现有自动化覆盖图

| 自动化入口 | 覆盖重点 | 当前状态 |
|---|---|---:|
| `go test ./...` | Go 包编译、类型、基础测试 | Pass |
| `npm run build` | TypeScript、React、Vite build | Pass |
| `scripts/dev-check.sh` | 工具链、Docker 服务、health、frontend、Helm render | Pass |
| `scripts/smoke-test.sh` | 注册、登录、API key、models、chat、billing、usage、files、revoke | 已定义 |
| `RUN_FALLBACK_SMOKE=true scripts/smoke-test.sh` | provider fallback、request_logs fallback_count | 已定义 |
| `scripts/regression/admin-foundation.sh` | Guardrails、Benchmark、Private Inference、Audit export/retention、Alert ack/resolve | Pass - 2026-06-26 |
| `scripts/regression/user-marketplace-workflow.sh` | JWT missing-token rejection、Marketplace list/filter/detail/compare、Workflow create/run/detail/traces | Pass - 2026-06-26 |
| `scripts/regression/guardrails-policy.sh` | Guardrails PII block、prompt injection block、results/audit/DB persistence | Pass - 2026-06-26 |
| `scripts/regression/auth-conflict.sh` | Duplicate registration conflict、stable error envelope、no DB detail leakage | Pass - 2026-06-26 |
| `scripts/regression/audio-mock.sh` | Audio STT/TTS mock provider routing、request_logs、usage_logs | Pass - 2026-06-26 |
| `scripts/regression/file-scan.sh` | Clean upload scan metadata、EICAR block、blocked upload audit | Pass - 2026-06-26 |
| `scripts/regression/provider-health-stats.sh` | Provider 24h request/error/fallback aggregation | Pass - 2026-06-26 |
| `scripts/regression/workflow-webhook.sh` | Workflow callback URL validation、webhook delivery response/detail/DB persistence | Pass - 2026-06-26 |
| `scripts/regression/tool-credentials.sh` | Tool credential create/list mask、DB encrypted persistence、revoke、invalid input | Pass - 2026-06-26 |
| `scripts/regression/agent-sessions.sh` | Agent session create/list/get/close、workflow run binding、DB persistence | Pass - 2026-06-26 |
| `scripts/regression/prompt-templates.sh` | Prompt template create/list/get/archive、prompt workflow creation/run、DB persistence | Pass - 2026-06-26 |
| `scripts/regression/pricing-history.sh` | Model pricing create/update history、Admin detail/history API、DB persistence | Pass - 2026-06-26 |
| `scripts/regression/workspace-cost-attribution.sh` | Workspace usage total + model/provider/user cost attribution + CSV compatibility | Pass - 2026-06-26 |
| `scripts/regression/provider-credentials.sh` | Provider create、masked credential create/list、sealed DB persistence、revoke、audit | Pass - 2026-06-26 |
| API smoke 手工脚本 | audit retention、RBAC、workspace budget/quota、workflow、benchmark、guardrails | 已执行并记录于 tracker |

---

## 6. 尚未完全自动化的测试项

这些功能已有实现和手工/API smoke 记录，但建议后续拆成脚本或 CI：

| 功能 | 建议自动化方式 | 优先级 |
|---|---|---:|
| Admin Overview UI | in-app Browser admin login + overview render | Completed 2026-06-26 |
| Admin Workspaces member role/status UI | in-app Browser deep E2E | Completed 2026-06-26 |
| Admin Workspaces budget/quota UI | in-app Browser deep E2E | Completed 2026-06-26 |
| Dashboard mobile layout | in-app Browser 390x844 | Completed 2026-06-26 |
| Guardrails policy/results API | `scripts/regression/admin-foundation.sh` | Completed 2026-06-26 |
| Guardrails PII/injection | `scripts/regression/guardrails-policy.sh` | Completed 2026-06-26 |
| Benchmark task/run/result | `scripts/regression/admin-foundation.sh` | Completed 2026-06-26 |
| Inference cluster/node/deployment | `scripts/regression/admin-foundation.sh` | Completed 2026-06-26 |
| Audit log filters/export/retention | `scripts/regression/admin-foundation.sh` | Completed 2026-06-26 |
| Alert ack/resolve | `scripts/regression/admin-foundation.sh` | Completed 2026-06-26 |
| Marketplace compare/detail | `scripts/regression/user-marketplace-workflow.sh` | Completed 2026-06-26 |
| Workflow run traces | `scripts/regression/user-marketplace-workflow.sh` | Completed 2026-06-26 |

---

## 7. 当前测试结论

```text
当前已完成或基础闭环完成的功能，均已有对应测试用例和验收标准。
本轮执行的基础工程验证全部通过：
  - go test ./...
  - npm run build
  - scripts/dev-check.sh

核心业务链路已有 scripts/smoke-test.sh 覆盖。
企业控制面、RBAC、Audit、Benchmark、Guardrails、Workflow、Self-hosted Inference 等增强链路已有 tracker 中记录的 API smoke 结果。

后续建议继续将剩余 tracker 手工/API smoke 固化为 scripts/regression/*.sh 或 Playwright UI regression；Admin foundation P1 已先固化为 scripts/regression/admin-foundation.sh。
```
