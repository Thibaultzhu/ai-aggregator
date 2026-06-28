# Project Cost Centers Regression - 2026-06-26

## Scope

验证企业 FinOps 的项目/成本中心基础闭环：

- `projects` schema 和 `project_id` 归因字段。
- Admin 创建/查看 Workspace Project。
- 用户 API Key 绑定 `workspace_id + project_id`。
- API Key 认证后仍可访问 `/v1/models`。
- `request_logs` 按 project 归因进入 Workspace Usage。
- Workspace Usage CSV 导出包含 `project_id` 并支持 `project_id` 过滤。

## Changed Files

- `migrations/022_v22_project_cost_centers.sql`
- `backend/internal/storage/store.go`
- `backend/internal/auth/middleware.go`
- `backend/internal/gateway/router.go`
- `frontend/src/lib/api.ts`
- `frontend/src/types/index.ts`
- `frontend/src/pages/ApiKeys.tsx`
- `frontend/src/pages/Admin.tsx`
- `scripts/regression/project-cost-centers.sh`

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/project-cost-centers.sh
```

## Result

```text
Total: 16
Passed: 16
Failed: 0
```

## Verified Checks

| # | Check | Result |
|---:|---|---:|
| 1 | Apply project cost center migration | PASS |
| 2 | Health status field | PASS |
| 3 | Registration returned user id | PASS |
| 4 | Promote user to admin | PASS |
| 5 | Login returned JWT | PASS |
| 6 | Organization id returned | PASS |
| 7 | Workspace id returned | PASS |
| 8 | Project id returned | PASS |
| 9 | Project list includes created project | PASS |
| 10 | Workspace owner membership created | PASS |
| 11 | API key response includes project id | PASS |
| 12 | API key stored project id | PASS |
| 13 | Project-bound key accesses models | PASS |
| 14 | Seed project request logs | PASS |
| 15 | Workspace usage includes project attribution | PASS |
| 16 | Workspace usage CSV includes project id | PASS |

## Acceptance Status

Project Cost Center baseline is verified.

Remaining future enhancements:

- Project-level budget/quota enforcement.
- Project owner/member RBAC.
- Project selector UI instead of manual UUID entry in user API Key page.
- Project-level dashboards and invoice/statement grouping.
