# Workflow Webhook Worker Regression - 2026-06-27

## Scope

This regression verifies real workflow webhook delivery after the previous callback audit baseline.

## Implemented Components

| Area | Status | Evidence |
|---|---:|---|
| Delivery metadata schema | Verified | `attempt_count`, `max_attempts`, `response_body`, `signature`, `delivered_at` |
| Background delivery worker | Verified | Workflow run records webhook and asynchronously POSTs callback |
| HMAC signature | Verified | `X-AAG-Signature: sha256=...` |
| Delivery headers | Verified | `X-AAG-Event`, `X-AAG-Delivery`, `User-Agent` |
| Retry state | Verified | `retrying` status supported; max attempts defaults to 3 |
| Delivered state | Verified | Successful callback updates DB to `delivered`, `response_status=204`, `attempt_count=1` |
| Run detail | Verified | `GET /api/user/workflow-runs/:id` shows delivered webhook metadata |

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/workflow-webhook-worker.sh
```

## Test Result

```text
Total: 8
Passed: 8
Failed: 0
```

## Remaining Enhancements

- Add persistent queued worker for process restarts.
- Add dead-letter and replay UI.
- Add configurable webhook secret per workspace/customer.

