# Smart Routing Regression - 2026-06-26

## Scope

验证 Smart Routing 基础闭环：

- `routing_policies` schema。
- Admin Routing Policy create/list API。
- Admin Routing UI 基础入口。
- Router 根据 active policy 重新排序 provider fallback chain。
- `latency` strategy 使用最近 24h `request_logs` latency stats。
- 真实 `/v1/chat/completions` 调用选择低延迟 provider，即使该 provider 的静态 priority 更低。

## Changed Files

- `migrations/023_v23_smart_routing_policies.sql`
- `backend/internal/storage/store.go`
- `backend/internal/router/model_router.go`
- `backend/internal/gateway/router.go`
- `frontend/src/lib/api.ts`
- `frontend/src/pages/Admin.tsx`
- `scripts/regression/smart-routing.sh`

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/smart-routing.sh
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
| 1 | Apply smart routing migration | PASS |
| 2 | Health status field | PASS |
| 3 | Registration returned user id | PASS |
| 4 | Promote user to admin | PASS |
| 5 | Login returned JWT | PASS |
| 6 | Mock slow provider created | PASS |
| 7 | Mock fast provider created | PASS |
| 8 | Smart routing test model created | PASS |
| 9 | Slow provider bound with higher static priority | PASS |
| 10 | Fast provider bound with lower static priority | PASS |
| 11 | Seed provider latency stats | PASS |
| 12 | Latency routing policy created | PASS |
| 13 | Routing policy list includes latency policy | PASS |
| 14 | API key created | PASS |
| 15 | Chat completion returned mock response | PASS |
| 16 | Latency policy selected faster provider | PASS |

## Acceptance Status

Smart Routing baseline is verified.

Implemented strategies:

- `priority`
- `cost`
- `latency`
- `balanced`

Remaining future enhancements:

- Workspace-aware routing context in `/v1` route selection.
- A/B routing experiment split and sticky bucketing.
- Provider-level benchmark quality score, not only model-level benchmark score.
- Admin disable/update policy controls.
- Request log annotation with selected routing policy ID.
