# Product Requirements Document — AI Aggregator Platform

## 1. Product Overview

AI Aggregator Platform 是一个面向开发者、企业、SI、Telco、政府和垂直行业的多模型统一调用、治理、计费、路由、观测和工作流平台。

它的底层是 AI Gateway，中层是 Model Marketplace、Enterprise Control Plane、FinOps、Reliability、Evaluation、Guardrails，上层是 Workflow / Agent API 和行业场景应用。

## 2. Problem Statement

企业和开发者在使用 AI 模型时面临以下问题：

| 问题 | 影响 |
|---|---|
| 模型供应商众多 | 每接一个模型都要适配不同 API、鉴权、错误格式 |
| 成本不可控 | token 用量、部门归因、预算、上游成本不透明 |
| 生产稳定性不足 | 单一模型宕机、限流、延迟波动会影响业务 |
| 缺少治理 | API Key、权限、审计、日志、数据合规难统一 |
| 模型选择困难 | 不知道哪个模型适合哪个任务 |
| 业务落地门槛高 | 只有模型调用，缺少 workflow / agent / tool / RAG 编排 |

## 3. Product Goals

| 目标 | 说明 |
|---|---|
| Unified Access | 一个 API Key 调用多个模型和 provider |
| Production Reliability | 支持 fallback、retry、health check、status page |
| Cost Governance | 支持 billing、quota、budget、cost attribution、margin |
| Enterprise Control | 支持 org、workspace、RBAC、audit、BYOK |
| Model Marketplace | 支持模型发现、比较、价格、能力标签 |
| Result API | 支持 workflow / agent / task-level API |
| Private / Sovereign AI | 支持本地化部署、自托管模型和数据驻留 |

## 4. Target Users

| 用户 | 需求 |
|---|---|
| 独立开发者 | 一个 API Key 快速接入多模型 |
| AI 应用团队 | 模型切换、fallback、日志、成本控制 |
| 企业 IT / 平台团队 | AI 调用统一入口、权限、审计、合规 |
| FinOps 团队 | token 成本、预算、配额、成本归因 |
| SI / MSP / Telco | 白标、渠道、统一采购和客户转售 |
| 政府 / 金融 | 私有部署、数据驻留、审计、SLA |
| 垂直业务团队 | 直接调用行业 Agent / Workflow，而不是裸模型 |

## 5. Business Models

| 商业模式 | 收入方式 | 对应模块 |
|---|---|---|
| API Resale / Thin Gateway | token markup、充值抽成 | Gateway、Provider Adapter、Billing |
| Model Marketplace | 平台抽成、模型推荐、企业采购 | Catalog、Pricing、Comparison |
| Self-hosted Inference Provider | token 计费、专属实例、GPU 托管 | Inference Cluster、Model Deployment |
| AI Gateway / Governance | 企业订阅、私有化 license | Workspace、RBAC、Audit、Policy |
| AI FinOps | 成本治理模块订阅 | Budget、Quota、Cost Attribution |
| Smart Routing | 高级路由策略订阅 | Cost / Latency / Quality Router |
| Reliability / SLA | SLA 套餐、生产级网关 | Health Check、Fallback、Status Page |
| Evaluation / Benchmark | 模型评测、持续评估 | Benchmark、A/B Test、LLM Judge |
| Guardrails / Compliance | 合规包、审计包 | PII、Moderation、Policy Engine |
| Workflow / Agent API | 按任务、按 seat、按 workflow 收费 | Workflow、Agent、Tool Registry |
| BYOK / BYOM | 企业控制面订阅 | Provider Credentials、BYOK Routing |
| Private / Sovereign AI | 年费、部署费、运维费 | Helm、Local Model、Private Network |
| White-label / OEM | 渠道授权、收入分成 | Branding、Partner Console |
| Unified Billing | 采购代理、账单统一 | Invoice、Cost Center、PO |
| AI Assurance | 风险报告、人工审核 | Evaluation、Human Review、Incident |

## 6. User Journey

### Developer Journey

```text
注册 → 创建 Workspace → 充值或领取额度 → 创建 API Key → 查看模型列表 → Playground 测试 → 调用 /v1/chat/completions → 查看 usage / logs / billing
```

### Enterprise Admin Journey

```text
创建 Organization → 邀请成员 → 分配角色 → 配置模型访问策略 → 配置预算和 quota → 配置 provider / BYOK → 查看 audit logs 和成本归因
```

### Provider Routing Journey

```text
请求进入 Gateway → API Key 鉴权 → 检查余额 / quota / rate limit → 解析模型 → 查找可用 provider → health-based routing → fallback → 记录 request log → 计费 → 返回
```

### Billing Journey

```text
请求完成 → 解析 token usage → 计算用户收费 → 计算上游成本 → 写 usage_records → 写 billing_transactions → 更新 balance → 计算 gross margin
```

## 7. Scope

### v0.1 In Scope

- User auth
- API Key
- `/v1/models`
- `/v1/chat/completions`
- Mock provider
- DashScope provider
- Basic routing
- Usage logs
- Billing transactions
- Balance deduction
- Dashboard
- Billing UI
- Playground
- Docker compose
- Smoke test

### v0.2 In Scope

- Request logs
- Provider health check
- Provider failover
- Provider status page
- Admin model / provider management
- Unified error format
- Request ID trace
- Fallback smoke test

### Out of Scope for v0.2

- Full workflow / agent runtime
- Self-hosted GPU cluster
- Complex benchmark engine
- Full guardrails engine
- Fine-tuning
- White-label billing settlement
- Full marketplace ranking

## 8. Acceptance Criteria

v0.2 通过条件：

1. v0.1 主链路仍可运行。
2. 每个 `/v1/chat/completions` 请求都有 request_id。
3. 成功和失败请求都写入 request_logs。
4. provider health check 可记录状态。
5. router 不选择 disabled / unhealthy provider，除非无可用 provider。
6. fallback 可通过 mock failure mode 验证。
7. 管理员可以 CRUD models、providers、model_provider mappings。
8. 前端可以查看 request logs 和 provider status。
9. go test、docker compose、smoke test 全部通过。

## 9. Risks

| 风险 | 缓解 |
|---|---|
| Codex 重构过度 | 强制保留 v0.1 主链路 |
| 计费错误 | billing 逻辑单测 + smoke test |
| provider secret 泄露 | secret 加密/脱敏，不返回前端 |
| 日志成本膨胀 | request_preview / response_preview 做长度限制 |
| 工作流污染 Gateway Core | 明确模块边界 |
| 过早做 GPU / Agent | v0.2 只做架构设计，不实现 |
