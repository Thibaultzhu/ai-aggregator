# Pricing History Regression Results - 2026-06-26

## Scope

本轮覆盖 v0.4/v0.5 的 pricing history / price-change audit baseline：

- `model_pricing_history` 数据表和索引
- Admin 创建模型时记录初始价格 history
- Admin 更新模型价格时记录 old/new input/output price
- Admin model detail 返回最近 pricing history
- 独立 pricing history API 可查询变更历史
- Admin Models UI 展示最近价格变更

## Code Changes

| Area | Files | Change |
|---|---|---|
| Migration | `migrations/021_v21_model_pricing_history.sql` | 新增 `model_pricing_history` table |
| Storage | `backend/internal/storage/store.go` | 新增 `ModelPricingHistory`、record/list methods |
| Backend Gateway | `backend/internal/gateway/router.go` | model create/update 记录 pricing history；新增 `/api/admin/models/:id/pricing-history` |
| Frontend API | `frontend/src/lib/api.ts` | 新增 pricing history types/API client |
| Frontend UI | `frontend/src/pages/Admin.tsx` | Admin Models detail 展示 Pricing History |
| Regression | `scripts/regression/pricing-history.sh` | 自动化覆盖 create/update/detail/history/DB persistence |

## Execution

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/pricing-history.sh
```

Result:

| Case | Objective | Expected | Result |
|---|---|---|---|
| TC-REG-PRICE-HIST-001 | Ensure schema | `model_pricing_history` table available | Pass |
| TC-REG-PRICE-HIST-002 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-PRICE-HIST-003~005 | Admin auth setup | Register, promote to admin, login returns JWT | Pass |
| TC-REG-PRICE-HIST-006 | Create model pricing | Model created with initial input/output price | Pass |
| TC-REG-PRICE-HIST-007 | Create history | `change_type=create` pricing history persisted | Pass |
| TC-REG-PRICE-HIST-008 | Update model pricing | Input/output price updated | Pass |
| TC-REG-PRICE-HIST-009 | History API | API returns old/new price update row | Pass |
| TC-REG-PRICE-HIST-010 | Detail API | Admin model detail includes `pricing_history` | Pass |
| TC-REG-PRICE-HIST-011 | DB persistence | Update row persisted in DB | Pass |

Summary: Total 11，Passed 11，Failed 0。

## Related Verification

| Command | Result |
|---|---|
| `go test ./...` | Pass |
| `npm run build` | Pass |
| `./scripts/dev-check.sh` | Pass |
| `BASE_URL=http://localhost:8081 scripts/regression/admin-foundation.sh` | Pass - 24 / 0 |

## Remaining Work

当前 pricing history 是 admin price-change audit baseline。后续增强：

- 在 audit_logs 中增加专门 price-change audit action
- 支持价格生效时间、未来价格计划和回滚
- 支持 provider-level cost multiplier history
- Pricing 页面增加历史/生效时间展示
