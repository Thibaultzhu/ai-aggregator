# SDK Addon 主线影响评估与独立开发清单

## 1. 文档目的

本文档用于将 **SDK 功能需求** 从 AI Aggregator 当前主线开发中独立拆出，作为一个单独的 **Addon / Extension Track** 管理。

核心目标：

```text
SDK Addon 不阻塞当前主线。
SDK Addon 不改变 Gateway Core 主链路。
SDK Addon 不影响 Request Logs、Provider Health、Failover、Admin CRUD 等 v0.2 核心开发。
SDK Addon 只在 API contract 稳定后启动。
```

---

## 2. 结论：是否影响当前主线开发？

### 2.1 默认不影响

SDK Addon **默认不影响当前主线开发**。

| 判断项 | 结论 |
|---|---|
| 是否需要修改 Gateway Core | 不需要 |
| 是否需要修改 Provider Routing | 不需要 |
| 是否需要修改 Billing | 不需要 |
| 是否需要修改数据库 | 初始版本不需要 |
| 是否影响 Request Logs | 不影响 |
| 是否影响 Failover | 不影响 |
| 是否影响 Admin CRUD | 不影响 |
| 是否阻塞 v0.2 | 不阻塞 |

SDK 初始版本只依赖已有或即将稳定的 API：

```text
GET /v1/models
POST /v1/chat/completions
streaming response
standard error format
request_id
```

---

### 2.2 会产生间接依赖的地方

SDK 本身不阻塞主线，但依赖以下主线能力稳定：

| 主线能力 | SDK 是否依赖 | 说明 |
|---|---|---|
| `/v1/models` | 是 | SDK list models 需要 |
| `/v1/chat/completions` | 是 | SDK chat completion 需要 |
| Streaming 格式 | 是 | SDK streaming parser 需要 |
| 标准错误格式 | 强依赖 | SDK error mapping 需要 |
| request_id | 强依赖 | SDK 排查日志需要 |
| Request Logs | 弱依赖 | SDK 文档可引导用户用 request_id 查日志 |
| Quota / Budget Error | v0.3 依赖 | 企业 SDK 阶段再做 |
| Workflow / Agent API | v0.7 依赖 | 后续 Agent SDK 再做 |

合理启动时机：

```text
v0.2 后半段：当 request_id、standard error format、streaming contract 基本稳定后，再开始 TypeScript / Python SDK。
```

---

## 3. 当前主线优先级保持不变

当前主线仍然优先完成：

```text
1. Request Logs / Observability
2. Provider Health Check
3. Provider Failover
4. Admin Model / Provider Management
5. Better Error Format
6. Smoke Test / Regression
```

SDK 不应插入这些任务之前。

---

## 4. SDK Addon 的产品价值

SDK 的目标不是简单封装 HTTP，而是降低终端用户使用 AI Aggregator 的成本。

| 成本类型 | SDK 降低方式 |
|---|---|
| 接入成本 | 一行代码完成模型调用 |
| 模型切换成本 | 用户换 model，不换业务代码 |
| Streaming 成本 | 封装 SSE / chunk parser |
| 错误处理成本 | 标准化 AuthenticationError、RateLimitError、QuotaError 等 |
| 日志排查成本 | 自动暴露 request_id |
| FinOps 接入成本 | 后续自动携带 workspace、cost_center、end_user_id |
| Workflow 接入成本 | 后续封装 workflow / agent run API |

---

## 5. SDK Addon 范围

### 5.1 Addon v0.1：SDK Design Lock

只做文档，不写代码。

产出：

```text
docs/addons/SDK_ADDON_REQUIREMENTS.md
docs/addons/SDK_ADDON_ARCHITECTURE.md
docs/addons/SDK_ADDON_API_CONTRACT.md
docs/addons/SDK_ADDON_ACCEPTANCE.md
```

### 5.2 Addon v0.2：TypeScript SDK MVP

功能范围：

```text
Client 初始化
API Key Auth
baseURL 配置
timeout 配置
retry 配置
GET /v1/models
POST /v1/chat/completions
streaming chat completions
standard errors
request_id extraction
examples
tests
```

建议目录：

```text
sdk/typescript/
├── src/
│   ├── client.ts
│   ├── models.ts
│   ├── chat.ts
│   ├── streaming.ts
│   ├── errors.ts
│   ├── retries.ts
│   ├── types.ts
│   └── index.ts
├── examples/
├── tests/
├── package.json
├── tsconfig.json
└── README.md
```

### 5.3 Addon v0.3：Python SDK MVP

功能范围：

```text
Client 初始化
API Key Auth
base_url 配置
timeout 配置
retry 配置
GET /v1/models
POST /v1/chat/completions
streaming chat completions
standard errors
request_id extraction
examples
tests
```

建议目录：

```text
sdk/python/
├── ai_aggregator/
│   ├── __init__.py
│   ├── client.py
│   ├── models.py
│   ├── chat.py
│   ├── streaming.py
│   ├── errors.py
│   ├── retries.py
│   └── types.py
├── examples/
├── tests/
├── pyproject.toml
└── README.md
```

### 5.4 Addon v0.4：Playground Export Code

功能范围：

```text
Export cURL
Export TypeScript SDK
Export Python SDK
Export OpenAI-compatible client example
不导出真实 API Key
```

### 5.5 Addon v0.5：Enterprise Metadata Support

依赖 v0.3 Enterprise Control Plane / FinOps。

功能范围：

```text
metadata.workspace_id
metadata.project_id
metadata.cost_center
metadata.end_user_id
metadata.trace_id
metadata.tags
```

### 5.6 Addon v0.6：Workflow / Agent SDK

依赖 v0.7 Workflow / Agent API。

功能范围：

```text
client.workflows.run()
client.workflows.getRun()
client.agents.run()
client.agents.getRun()
async polling
webhook helper
task-level cost retrieval
```

---

## 6. 明确不做的内容

当前 SDK Addon 初始阶段不做：

```text
Admin SDK
Billing SDK
Workflow SDK
Agent SDK
Knowledge Base SDK
Fine-tuning SDK
File Upload SDK
Batch Job SDK
BYOK SDK
SDK telemetry auto-upload
package publishing automation
```

---

## 7. 风险与控制方式

| 风险 | 控制方式 |
|---|---|
| API contract 变动导致 SDK 频繁返工 | 等 v0.2 API contract 稳定后再启动代码开发 |
| SDK retry 和 Gateway retry 叠加放大请求 | SDK 默认 maxRetries=2，文档说明 Gateway 也可能 retry/fallback |
| SDK 示例泄露 API Key | 所有示例使用环境变量 |
| SDK 影响主线代码 | SDK 放在 sdk/ 目录，不修改 backend/frontend |
| 过早引入 Workflow/Agent | 明确放到 Addon v0.6 |

---

## 8. Codex 执行提示词

```text
You are working on an existing AI Aggregator / AI Gateway project.

This task is an ADDON track for SDK design and development. It must not block or destabilize current mainline development.

Mainline priority remains:
1. Request Logs / Observability
2. Provider Health Check
3. Provider Failover
4. Admin Model / Provider Management
5. Better Error Format
6. Smoke Test / Regression

SDK Addon should be isolated under docs/addons and sdk directories.

Do not modify Gateway Core behavior.
Do not modify billing.
Do not modify provider routing.
Do not modify failover.
Do not modify admin CRUD.
Do not introduce workflow, agent, knowledge base, BYOK, or enterprise metadata support in the first SDK MVP.

First create SDK Addon documentation only:
1. docs/addons/SDK_ADDON_REQUIREMENTS.md
2. docs/addons/SDK_ADDON_ARCHITECTURE.md
3. docs/addons/SDK_ADDON_API_CONTRACT.md
4. docs/addons/SDK_ADDON_ACCEPTANCE.md

After the API contract is stable, implement TypeScript SDK and Python SDK under:
- sdk/typescript
- sdk/python

SDK MVP scope:
- API Key authentication
- configurable baseURL
- configurable timeout
- configurable maxRetries
- /v1/models
- /v1/chat/completions
- streaming chat completions
- normalized errors
- request_id extraction
- examples
- tests

Final output must include:
1. Added files
2. SDK Addon phase completed
3. APIs covered
4. Examples added
5. Tests added
6. Any dependency on mainline v0.2
7. Confirmation that mainline behavior was not changed
```

---

## 9. 验收清单

```text
[ ] SDK Addon 已独立为 docs/addons 文档
[ ] 已明确不阻塞主线
[ ] 已明确不修改 Gateway Core
[ ] 已明确依赖的 API contract
[ ] 已明确 TypeScript SDK MVP 范围
[ ] 已明确 Python SDK MVP 范围
[ ] 已明确不做 Admin / Workflow / Agent
[ ] 已明确 request_id 处理方式
[ ] 已明确 error mapping
[ ] 已明确 retry / timeout 策略
[ ] examples 不包含真实 API Key
[ ] SDK 目录不影响 backend/frontend 构建
```

---

## 10. 推荐项目落位

```text
docs/addons/SDK_ADDON_MAINLINE_IMPACT_AND_CHECKLIST.md
```

主线 Roadmap 只引用一句：

```text
SDK Addon is tracked separately in docs/addons/SDK_ADDON_MAINLINE_IMPACT_AND_CHECKLIST.md and does not block current mainline development.
```
