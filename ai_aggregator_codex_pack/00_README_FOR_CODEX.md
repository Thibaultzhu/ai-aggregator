# AI Aggregator Codex Development Pack

本目录是一套用于在 Codex 中分阶段开发 AI Aggregator Platform 的工程提示词与架构文档包。

## 使用方式

1. 将本目录复制到项目根目录，例如 `docs/codex/`。
2. 在 Codex 中先执行 `01_MASTER_CODEX_PROMPT.md`。
3. 不要一次性要求 Codex 完成全部平台，必须按 Sprint 推进。
4. 每个 Sprint 结束后，必须执行 `11_ACCEPTANCE_AND_REGRESSION.md` 中的验收命令。
5. 如果 Codex 出现上下文漂移，重新喂入：
   - `01_MASTER_CODEX_PROMPT.md`
   - 当前 Sprint 对应段落
   - `07_MODULE_BREAKDOWN.md`
   - `08_DATA_MODEL.md`
   - `09_API_DESIGN.md`

## 文档清单

| 文件 | 作用 |
|---|---|
| 01_MASTER_CODEX_PROMPT.md | 给 Codex 的总控提示词 |
| 02_PRODUCT_REQUIREMENTS_PDR.md | 产品需求文档 |
| 03_SCOPE_AND_VERSIONING.md | v0.1-v1.0 范围定义 |
| 04_ARCHITECTURE.md | 总体架构与技术分层 |
| 05_BUSINESS_MODEL_TO_MODULE_MAPPING.md | 商业模式到工程模块映射 |
| 06_ROADMAP_AND_SPRINTS.md | 分 Sprint 开发路线 |
| 07_MODULE_BREAKDOWN.md | 后端/前端模块拆解 |
| 08_DATA_MODEL.md | 数据模型与数据库演进 |
| 09_API_DESIGN.md | API 设计 |
| 10_CODEX_SPRINT_PROMPTS.md | 每个 Sprint 的可复制提示词 |
| 11_ACCEPTANCE_AND_REGRESSION.md | 验收与防回归清单 |
| 12_MODULE_LEVEL_PROMPTS.md | 模块级详尽开发提示词 |
| 13_SECURITY_AND_ENGINEERING_GUARDRAILS.md | 安全与工程约束 |
| 14_UI_PAGE_SPEC.md | 前端页面功能规格 |

## 总体开发原则

- 保留当前 v0.1 主链路，不重写可运行 MVP。
- 先补齐文档和稳定性，再扩展 Marketplace、Agent、Self-hosted Inference。
- Gateway Core 与 Workflow / Agent Layer 必须解耦。
- Provider Adapter 必须统一 response、usage、error、stream chunk。
- Billing 必须区分用户收费、上游成本和毛利。
- 每次迭代必须有迁移、测试、Smoke Test、回滚说明。
