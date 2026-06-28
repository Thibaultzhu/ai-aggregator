# Workspace Cost Attribution Regression - 2026-06-26

## Scope

This regression verifies the v0.3 FinOps workspace cost attribution baseline:

- Admin workspace usage summary still returns total requests, cost, and tokens.
- The same endpoint returns top attribution by model, provider, and user.
- Admin Workspaces UI can render the attribution fields through typed frontend API models.
- Workspace usage CSV export remains compatible.

## Changed Files

- `backend/internal/storage/store.go`
  - Extended `WorkspaceUsageSummary` with `by_model`, `by_provider`, and `by_user`.
  - Added grouped attribution queries against `request_logs`.
- `frontend/src/lib/api.ts`
  - Added `WorkspaceUsageAttribution` and optional attribution arrays.
- `frontend/src/pages/Admin.tsx`
  - Added Cost Attribution panel under selected workspace usage summary.
- `scripts/regression/workspace-cost-attribution.sh`
  - Added executable API/DB regression.

## Test Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/workspace-cost-attribution.sh
```

## Result

```text
Total: 12
Passed: 12
Failed: 0
```

## Covered Test Cases

| Case | Target | Expected | Result |
|---|---|---|---|
| TC-REG-WS-COST-001 | Health | `/health` returns `status=ok` | Pass |
| TC-REG-WS-COST-002 | Admin setup | Register, promote, and login admin user | Pass |
| TC-REG-WS-COST-003 | Workspace setup | Create organization and workspace through Admin API | Pass |
| TC-REG-WS-COST-004 | Seed usage | Insert controlled `request_logs` rows with `workspace_id` | Pass |
| TC-REG-WS-COST-005 | Total summary | Usage endpoint returns 3 requests, 900 tokens, $0.18 cost | Pass |
| TC-REG-WS-COST-006 | Model attribution | `by_model` includes `qwen-plus` with 2 requests and $0.09 cost | Pass |
| TC-REG-WS-COST-007 | Provider attribution | `by_provider` includes `bailian_intl` with 2 requests and $0.09 cost | Pass |
| TC-REG-WS-COST-008 | User attribution | `by_user` includes regression admin with 3 requests and $0.18 cost | Pass |
| TC-REG-WS-COST-009 | CSV compatibility | Workspace usage export includes seeded rows | Pass |

## Additional Verification

```bash
source scripts/dev-env.sh && cd backend && go test ./...
npm run build
source scripts/dev-env.sh && ./scripts/dev-check.sh
BASE_URL=http://localhost:8081 scripts/regression/admin-foundation.sh
```

All commands passed on 2026-06-26.

## Remaining Enhancements

- Project-level cost center dimension is still not implemented.
- Time-window selectors and attribution filtering in the Admin UI remain future improvements.
- Invoice/PO level FinOps reporting remains out of scope for this baseline.
