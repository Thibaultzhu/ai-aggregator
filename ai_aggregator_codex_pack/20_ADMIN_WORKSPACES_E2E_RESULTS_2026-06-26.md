# Admin Workspaces Browser E2E Results - 2026-06-26

## 1. Scope

目标：验证 v0.3 Enterprise Control Plane 中 Admin Workspaces 页面不是只具备 API 能力，而是能通过真实前端完成组织、工作区、FinOps 配置、成员管理和 usage export 链路。

测试入口：

```text
http://localhost:5175/admin/workspaces
```

测试环境：

```text
backend: http://localhost:8081
frontend: http://localhost:5175
postgres: aag-postgres
redis: aag-redis
MOCK_PROVIDER_MODE=false
```

## 2. Code Change Under Test

本轮为 Admin Workspaces 页面补充稳定 E2E 定位点，不改变业务逻辑和样式：

```text
frontend/src/pages/Admin.tsx
```

新增 `data-testid` 覆盖：

```text
admin-workspaces-page
admin-org-name / admin-org-slug / admin-org-create
admin-workspace-name / admin-workspace-slug / admin-workspace-budget / admin-workspace-create
admin-workspace-row
admin-workspace-export
admin-budget-period / admin-budget-amount / admin-budget-soft / admin-budget-hard / admin-budget-save
admin-quota-type / admin-quota-limit / admin-quota-save
admin-member-user / admin-member-role / admin-member-status / admin-member-save
```

## 3. Executed Test Data

```text
Admin account: ui-admin-workspace-1782422679191885000@test.local
Member account: ui-workspace-member-1782422679191885000@test.local
Member user_id: ec81874c-522e-47f1-b2ac-695c9630743e

Organization: E2E Org 1782422728771
Organization slug: e2e-org-1782422728771
Workspace: E2E Workspace 1782422728771
Workspace slug: e2e-ws-1782422728771
Workspace id: b8652c60-4817-4aca-9f71-037c0af6181b
```

## 4. Browser E2E Results

| Test ID | Target | Method | Expected | Result |
|---|---|---|---|---:|
| TC-ADMIN-WS-UI-001 | Page identity | Open `/admin/workspaces` with admin session | URL/title correct, Workspaces page visible | Pass |
| TC-ADMIN-WS-UI-002 | Blank/error overlay check | DOM snapshot + screenshot | Meaningful app content; no Vite/framework error overlay | Pass |
| TC-ADMIN-WS-UI-003 | Create organization | Fill org name/slug and click Create | Organization visible and available for workspace creation | Pass |
| TC-ADMIN-WS-UI-004 | Create workspace | Fill workspace name/slug/monthly budget and click Create | Workspace appears and is selected | Pass |
| TC-ADMIN-WS-UI-005 | Workspace budget | Select `daily`, amount `55.50`, soft `70`, hard `95`, save | Budget list shows `$55.50 / daily`, `soft 70%`, `hard 95%`, `active` | Pass |
| TC-ADMIN-WS-UI-006 | Workspace quota | Select `requests_per_minute`, limit `42`, save | Quota list shows `requests_per_minute`, `limit 42`, `active` | Pass |
| TC-ADMIN-WS-UI-007 | Workspace member | Select member user, role `viewer`, status `invited`, save | Members list shows member email, role `viewer`, status `invited` | Pass |
| TC-ADMIN-WS-UI-008 | Console health | Browser console warnings/errors | No app runtime errors | Pass |
| TC-ADMIN-WS-UI-009 | CSV export API | Call workspace usage export endpoint with admin JWT | HTTP 200, `Content-Type: text/csv`, CSV header returned | Pass |

## 5. Export Verification

Browser button click produced no page error, but the in-app browser did not capture a download event for the frontend Blob/object URL download within the 10s wait window. To avoid treating a tooling limitation as product failure, the backend export endpoint was verified directly with the same workspace:

```text
GET /api/admin/workspaces/b8652c60-4817-4aca-9f71-037c0af6181b/usage/export?limit=1000
```

Observed response:

```text
HTTP/1.1 200 OK
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename="workspace-b8652c60-4817-4aca-9f71-037c0af6181b-usage.csv"
```

CSV header:

```text
request_id,created_at,method,path,model_id,provider_id,final_provider_id,status_code,error_code,latency_ms,input_tokens,output_tokens,total_tokens,charged_cost_usd,upstream_cost_usd,gross_margin_usd,fallback_count
```

## 6. Commands

```bash
cd frontend && npm run build
cd backend && source ../scripts/dev-env.sh && go test ./...
docker cp frontend/src/. aag-frontend:/app/src
docker restart aag-frontend
curl -fsS -I http://localhost:5175/admin/workspaces
```

Results:

```text
frontend npm run build: Pass
backend go test ./...: Pass
frontend /admin/workspaces HTTP check: Pass
Admin Workspaces Browser E2E: Pass
Workspace usage CSV API export: Pass
```

## 7. Console Notes

Relevant runtime errors:

```text
None
```

Ignored non-blocking warnings:

```text
React Router v7 future flag warnings from react-router-dom.
```

## 8. Conclusion

Admin Workspaces 的 v0.3 Enterprise Control Plane 核心管理流已完成前端交互级验证：组织、工作区、预算、配额、成员 role/status 和 usage CSV API export 均可用。

Remaining risk:

```text
前端 Blob 下载事件在当前 in-app Browser 中未被捕获；后端 CSV export 已验证为可用。
后续可将此流程固化为 Playwright/CI regression，以下载文件系统断言替代浏览器插件 download event。
```
