# Master Codex Prompt — AI Aggregator Platform

你正在维护一个已经可以运行的 AI Model Aggregator / AI Gateway MVP。你的任务不是推翻重写，而是在保留 v0.1 主链路的前提下，把项目逐步升级为完整的 AI Aggregator Platform。

## 当前项目假设

当前 v0.1 已具备或应保留以下链路：

```text
用户注册 / 登录
→ 创建 API Key
→ 调用 /v1/models
→ 调用 /v1/chat/completions
→ API Key 鉴权
→ 模型路由
→ Mock / DashScope Provider
→ 返回 response
→ 记录 usage_logs
→ 扣减 balance
→ 写 billing_transactions
→ Dashboard / Billing 展示
```

在开始任何开发前，你必须先阅读：

```text
README.md
API_EXAMPLES.md
MVP_IMPLEMENTATION_PLAN.md
migrations/
backend/internal/
frontend/src/
scripts/smoke-test.sh
docker-compose.yml
```

## 产品升级目标

将项目从 Thin AI Gateway MVP 升级为：

```text
AI Aggregator Platform
= AI Gateway
+ Model Marketplace
+ Self-hosted Inference Provider
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

## 必须支持的商业模式

文档和架构中必须纳入以下商业模式，并说明它们映射到哪些工程模块：

1. API Resale / Thin Gateway
2. Model Marketplace
3. Self-hosted Inference Provider
4. AI Gateway / Enterprise Governance
5. AI FinOps
6. Smart Routing
7. Reliability / SLA Layer
8. Evaluation / Benchmark
9. Guardrails / Compliance
10. Workflow / Agent API
11. BYOK / BYOM
12. Private / Sovereign AI
13. Data Loop / Fine-tuning / Synthetic Data
14. White-label / OEM / Channel
15. Developer / Agent / Tool Marketplace
16. Procurement / Unified Billing
17. AI Assurance / Risk Management

## 分阶段开发原则

不要一次性实现所有能力。必须按版本推进：

```text
v0.1 Runnable Thin Aggregator
v0.2 Reliable AI Gateway
v0.3 Enterprise Control Plane + FinOps
v0.4 Model Marketplace
v0.5 Evaluation / Benchmark / Optimization
v0.6 Guardrails / Compliance
v0.7 Workflow / Agent API
v1.0 Private / Sovereign AI + Self-hosted Inference
```

当前优先级是 v0.2，不要直接跳到 Agent / Workflow / GPU 私有化。

## v0.2 必须实现的内容

1. request_logs migration + storage + write path + frontend table
2. provider_health_checks migration + health job + provider status API + frontend page
3. failover chain + mock failure mode + smoke-test failover case
4. admin model / provider CRUD
5. model catalog metadata upgrade
6. better error format
7. request_id 全链路追踪

## 代码约束

1. 不要硬编码 secret。
2. 不要打印 API Key 明文。
3. API Key 必须 hash 存储，只在创建时展示一次。
4. 所有数据库操作必须使用 context 和参数化查询。
5. Provider Adapter 必须统一 response、usage、error、stream chunk。
6. Billing 必须区分：
   - charged_cost_usd
   - upstream_cost_usd
   - gross_margin_usd
7. Observability 必须记录 request_id，并能从前端查到请求详情。
8. Gateway Core 不要和 Workflow / Agent Layer 混在一起。
9. Admin 管理模型和 provider，不应长期依赖 seed SQL。
10. 每个 Sprint 完成后必须保证 go test、docker compose、smoke-test 不回归。

## 每次输出必须包含

完成每个任务后，请输出：

1. 修改了哪些文件
2. 新增了哪些数据库表 / 字段
3. 新增了哪些 API
4. 新增了哪些前端页面
5. 如何验证
6. 是否影响 v0.1 主链路
7. 下一步建议

## 禁止事项

- 禁止直接重写整个项目。
- 禁止删除现有可运行能力。
- 禁止在没有测试的情况下大规模改路由、鉴权、计费。
- 禁止把 Workflow / Agent Runtime 塞进 Gateway Core。
- 禁止把 provider secret 明文返回给前端。
