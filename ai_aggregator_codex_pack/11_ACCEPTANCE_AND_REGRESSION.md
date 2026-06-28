# Acceptance and Regression Checklist

## Universal Checks After Every Sprint

Run the project-specific equivalents of:

```bash
go test ./...
docker compose up -d --build
./scripts/smoke-test.sh
```

If frontend tests exist:

```bash
npm test
npm run lint
npm run build
```

## v0.1 Regression Checklist

| Check | Expected |
|---|---|
| User can register/login | Pass |
| User can create API Key | API key shown once |
| API key is not stored in plaintext | Pass |
| GET /v1/models works | Returns enabled models |
| POST /v1/chat/completions works | Returns normalized response |
| Usage log is written | Pass |
| Billing transaction is written | Pass |
| Balance is deducted | Pass |
| Dashboard renders | Pass |
| Billing page renders | Pass |
| Playground works | Pass |

## v0.2 Acceptance Checklist

### Request Logs

| Check | Expected |
|---|---|
| request_id generated | Every chat request has request_id |
| request_logs success path | Success request stored |
| request_logs failure path | Failure request stored when possible |
| no API key logged | Pass |
| request preview truncated | Pass |
| response preview truncated | Pass |
| user ownership enforced | User sees only own logs |

### Provider Health

| Check | Expected |
|---|---|
| Health check interface exists | Provider supports HealthCheck |
| health records stored | provider_health_checks has rows |
| manual check API works | Admin can trigger |
| status page works | UI displays provider health |

### Failover

| Check | Expected |
|---|---|
| mock failure mode works | Primary provider can be forced to fail |
| fallback works | Request succeeds via secondary provider |
| fallback log written | fallback_logs has row |
| fallback smoke-test script | `RUN_FALLBACK_SMOKE=true` validates request_logs fallback_count |
| no provider available handled | normalized routing_error |

### Admin

| Check | Expected |
|---|---|
| Non-admin blocked | 403 |
| Admin can create model | Pass |
| Admin can disable model | Disabled model not listed |
| Admin can create provider | Pass |
| Admin can disable provider | Router avoids it |
| Admin can map model-provider | Pass |

## Normalized Error Format

All errors should look like:

```json
{
  "error": {
    "code": "some_code",
    "message": "Human readable message",
    "type": "some_error_type",
    "request_id": "req_xxx"
  }
}
```

## Security Regression

- No API keys in logs.
- No provider secrets returned in API responses.
- No secrets committed to repository.
- SQL queries are parameterized.
- User endpoints enforce ownership.
- Admin endpoints enforce admin role.

## Rollback Plan

Every Sprint should include:

1. Migration rollback note.
2. Feature flag or safe disable path when applicable.
3. How to restore previous routing behavior.
4. How to disable new provider health filtering if it causes outage.
