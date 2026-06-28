# Admin Foundation Regression Results - 2026-06-26

## 1. Scope

目标：将此前记录为手工/API smoke 的 P1 控制面能力固化为可重复执行的本地回归脚本。

新增脚本：

```text
scripts/regression/admin-foundation.sh
```

覆盖范围：

```text
Guardrails policy/list/results
Benchmark task/run/detail
Private inference cluster/node/deployment
Audit log filter/export/retention dry-run
Alert rule + alert event ack/resolve
```

## 2. Environment

```text
BASE_URL=http://localhost:8081
backend=aag-backend
postgres=aag-postgres
redis=aag-redis
frontend=http://localhost:5175
MOCK_PROVIDER_MODE=false
```

脚本前置要求：

```text
curl
jq
docker
local Postgres container aag-postgres
```

脚本会创建一次性 admin 测试账号，并通过本地 Postgres 将其提升为 `admin`。Alert ack/resolve 回归会插入一条本地测试 `alert_events` 记录，再通过 Admin API 完成 ack/resolve。

## 3. Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/admin-foundation.sh
```

## 4. Result Summary

```text
Total: 24
Passed: 24
Failed: 0
```

## 5. Test Cases

| Test ID | Area | Target | Expected | Result |
|---|---|---|---|---:|
| TC-REG-ADMIN-001 | Health/Auth | Backend health + admin registration/login | health ok, JWT returned | Pass |
| TC-REG-GUARD-001 | Guardrails | Create guardrail policy | HTTP success, id returned | Pass |
| TC-REG-GUARD-002 | Guardrails | List policies | Created policy appears | Pass |
| TC-REG-GUARD-003 | Guardrails | List guardrail results | Response contains `data` array | Pass |
| TC-REG-BENCH-001 | Benchmark | Create benchmark task | Task id returned | Pass |
| TC-REG-BENCH-002 | Benchmark | Run benchmark for `qwen-max` | Completed run with results | Pass |
| TC-REG-BENCH-003 | Benchmark | Get benchmark run detail | Detail includes results | Pass |
| TC-REG-INF-001 | Inference | Create inference cluster | Cluster id returned | Pass |
| TC-REG-INF-002 | Inference | Reject invalid node status | HTTP 400 `invalid_request` | Pass |
| TC-REG-INF-003 | Inference | Create healthy inference node | Node id returned | Pass |
| TC-REG-INF-004 | Inference | Create model deployment | Deployment id returned | Pass |
| TC-REG-INF-005 | Inference | List model deployments | Created deployment appears | Pass |
| TC-REG-AUDIT-001 | Audit | Filter audit logs by action | HTTP success, `data` array | Pass |
| TC-REG-AUDIT-002 | Audit | Export audit logs CSV | HTTP 200 `text/csv` | Pass |
| TC-REG-AUDIT-003 | Audit | CSV header shape | Expected audit CSV columns | Pass |
| TC-REG-AUDIT-004 | Audit | Retention dry-run | `dry_run=true`, numeric matched/deleted counts | Pass |
| TC-REG-ALERT-001 | Alerts | Create alert rule | Rule id returned | Pass |
| TC-REG-ALERT-002 | Alerts | Insert regression alert event | Alert event id returned | Pass |
| TC-REG-ALERT-003 | Alerts | List alert history | Regression alert appears | Pass |
| TC-REG-ALERT-004 | Alerts | Acknowledge alert | Status becomes `acknowledged` | Pass |
| TC-REG-ALERT-005 | Alerts | Resolve alert | Status becomes `resolved` | Pass |

## 6. Implementation Notes

Script behavior:

```text
Fails fast on unexpected HTTP status.
Uses unique timestamp suffix for all generated data.
Avoids overwriting existing self-hosted provider/model deployment IDs.
Verifies invalid inference node status remains HTTP 400.
Verifies audit CSV by response header and header row.
```

Known tradeoff:

```text
The script is intended for local/dev regression because it directly promotes a generated user in Postgres and inserts a test alert event. CI can use the same pattern with an ephemeral database.
```

## 7. Conclusion

P1 control-plane API smoke coverage is now executable from the repository:

```text
scripts/regression/admin-foundation.sh
```

Guardrails、Benchmark、Private Inference、Audit Retention/Export 和 Alert ack/resolve 均已完成可重复回归验证。
