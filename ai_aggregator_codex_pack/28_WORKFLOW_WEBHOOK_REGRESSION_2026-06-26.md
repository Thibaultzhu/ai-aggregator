# Workflow Webhook Regression Results - 2026-06-26

## Scope

本轮覆盖 v0.7 Workflow / Agent Foundation 的 webhook callback baseline：

- `workflow_deliveries` 数据表与索引
- `POST /api/user/workflows/:id/runs` 接收 `callback_url`
- workflow run 成功后记录 `workflow.run.completed`
- run response 与 run detail 返回 webhook delivery
- 非 HTTP/HTTPS callback URL 被拒绝

## Code Changes

| Area | Files | Change |
|---|---|---|
| Backend Gateway | `backend/internal/gateway/router.go` | workflow run request 支持 `callback_url`；校验 URL scheme；run completed/failed 后记录 webhook delivery |
| Storage | `backend/internal/storage/store.go` | 新增 `WebhookDelivery`、`RecordWebhookDelivery`、`ListWebhookDeliveries`；workflow run detail 返回 `webhooks` |
| Migration | `migrations/017_v17_webhook_deliveries.sql` | 新增 `webhook_deliveries` 表、索引和 updated_at trigger |
| Frontend API | `frontend/src/lib/api.ts` | `runWorkflow` 支持 `callback_url`；`WorkflowRun` 支持 `webhooks` |
| Frontend UI | `frontend/src/pages/Workflows.tsx` | Run detail 展示 webhook delivery event/status/callback URL |
| Regression | `scripts/regression/workflow-webhook.sh` | 自动化覆盖 workflow webhook baseline |

## Execution

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/workflow-webhook.sh
```

Result:

| Case | Objective | Expected | Result |
|---|---|---|---|
| TC-REG-WF-WEBHOOK-001 | Ensure `webhook_deliveries` table | Migration/table available | Pass |
| TC-REG-WF-WEBHOOK-002 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-WF-WEBHOOK-003 | User auth setup | Register returns JWT | Pass |
| TC-REG-WF-WEBHOOK-004 | Create workflow | HTTP 201，returns `workflow_id` | Pass |
| TC-REG-WF-WEBHOOK-005 | Run with callback URL | Run response includes recorded webhook | Pass |
| TC-REG-WF-WEBHOOK-006 | Run detail webhook | `event_type=workflow.run.completed`，`status=recorded` | Pass |
| TC-REG-WF-WEBHOOK-007 | DB persistence | `webhook_deliveries` has row for run | Pass |
| TC-REG-WF-WEBHOOK-008 | URL validation | `ftp://...` rejected with HTTP 400 `invalid_request` | Pass |

Summary: Total 8，Passed 8，Failed 0。

## Related Verification

| Command | Result |
|---|---|
| `go test ./...` | Pass |
| `npm run build` | Pass |
| `./scripts/dev-check.sh` | Pass |
| `BASE_URL=http://localhost:8081 scripts/regression/user-marketplace-workflow.sh` | Pass - 18 / 0 |

## Remaining Work

当前 webhook 为 delivery audit baseline：记录回调目标、事件、payload 和状态，但尚未执行真实 HTTP POST 投递、重试、签名、DLQ 或 delivery worker。

v0.7 后续仍需补齐：

- tool credentials 管理与加密存储
- agent sessions
- webhook delivery worker、retry/backoff、签名验签
