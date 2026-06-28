# Provider Health Stats Regression Results - 2026-06-26

## 1. Scope

目标：补齐 v0.2 provider error-rate 聚合缺口，使 Admin Provider Health API 和 Provider Status 页面不仅展示最新 health check，还能展示最近 24 小时的请求量、错误数、错误率、fallback 数和平均请求延迟。

新增脚本：

```text
scripts/regression/provider-health-stats.sh
```

覆盖范围：

```text
GET /api/admin/provider-health
request_logs.final_provider_id aggregation
request_count_24h
error_count_24h
error_rate_24h
fallback_count_24h
avg_request_latency_ms_24h
```

## 2. Environment

```text
BASE_URL=http://localhost:8081
backend=aag-backend
postgres=aag-postgres
redis=aag-redis
frontend=http://localhost:5175
```

## 3. Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/provider-health-stats.sh
```

## 4. Result Summary

```text
Total: 6
Passed: 6
Failed: 0
```

## 5. Test Cases

| Test ID | Area | Target | Expected | Result |
|---|---|---|---|---:|
| TC-REG-PH-STATS-001 | Health | Backend health | HTTP 200, `status=ok` | Pass |
| TC-REG-PH-STATS-002 | Auth | Admin regression user | User registered and promoted to admin | Pass |
| TC-REG-PH-STATS-003 | Auth | Admin login | JWT returned | Pass |
| TC-REG-PH-STATS-004 | Data setup | Controlled provider/request logs | Provider and 3 request_logs inserted | Pass |
| TC-REG-PH-STATS-005 | API shape | Provider stats row | `/api/admin/provider-health` returns target provider | Pass |
| TC-REG-PH-STATS-006 | Aggregation | 24h stats | requests=3, errors=1, error_rate=0.3333, fallback=3, avg latency=200 | Pass |

## 6. Implementation Notes

Backend changes:

```text
storage.ProviderHealthCheck now includes 24h aggregate fields.
ListLatestProviderHealth joins request_logs by final_provider_id over the last 24 hours.
Admin Provider Health API returns aggregate stats without changing provider_health_checks schema.
```

Frontend changes:

```text
ProviderHealth type includes 24h aggregate fields.
Admin Provider Status table displays 24h requests, error rate, and fallback count.
```

## 7. Conclusion

Provider error-rate 聚合已从后续增强推进为已实现并脚本化验证：

```text
scripts/regression/provider-health-stats.sh
```

当前结果为 6 pass / 0 fail。
