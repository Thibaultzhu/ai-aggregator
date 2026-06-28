# Codex Tracker Maintenance Instructions

请将以下两个文件加入项目根目录：

- `PROJECT_STAGE_TRACKER.md`
- `FEATURE_IMPLEMENTATION_TRACKER.md`

## 使用目标

`PROJECT_STAGE_TRACKER.md` 用于跟踪版本阶段、阶段目标、完成状态、验收标准和 backlog。

`FEATURE_IMPLEMENTATION_TRACKER.md` 用于跟踪每个功能、页面、API、数据库表、实现方法论、技术路径、核心文件路径和验收方式。

## Codex 开发规则

每次新增、修改、完成一个功能后，必须同步更新：

1. `PROJECT_STAGE_TRACKER.md`
2. `FEATURE_IMPLEMENTATION_TRACKER.md`
3. 如涉及 API，更新 `API_DESIGN.md` / `API_EXAMPLES.md`
4. 如涉及数据库，更新 `DATA_MODEL.md` 和 migrations 说明
5. 如涉及前端页面，更新 `UI_PAGE_SPEC.md`
6. 如涉及架构变化，更新 `ARCHITECTURE.md` / `MODULE_BREAKDOWN.md`

## 每次变更必须记录

- 做了什么
- 为什么做
- 使用什么技术实现
- 修改了哪些后端文件
- 修改了哪些前端文件
- 新增了哪些 API
- 新增了哪些数据库表或字段
- 如何通过 curl / smoke-test / frontend manual test / SQL 验证
- 是否影响 v0.1 主链路
- 是否通过回归测试
- 已知限制
- 下一步优化

## 版本完成规则

只有当某版本所有 P0 功能达到 `Verified`，才能将该版本标记为 `Complete`。

当前版本状态：

```text
v0.1 = Runnable MVP / 基本完成
v0.2 = In Progress / 完成约 45%～55%
v0.3+ = Planned / 未开始
```

## 当前 v0.2 下一步优先级

1. 完成 fallback_logs 写入
2. 完成 provider_health_checks 写入
3. 完成 Provider Health API
4. 完成 Provider Status Page
5. 完成 Admin Model / Provider CRUD
6. 完成 fallback smoke-test
7. 回归验证 v0.1 主链路不破坏
