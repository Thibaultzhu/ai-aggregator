# Provider Credentials Regression - 2026-06-26

## Scope

This regression verifies the first commercial BYOK/provider credential baseline:

- Admin can create an OpenAI-compatible provider.
- Admin can save a provider API key without returning the raw secret.
- Runtime router resolves the latest active DB credential before falling back to environment variables.
- Admin can list masked credentials and revoke credentials.
- Provider credential create/revoke actions are written to audit logs.

## Changed Files

- `backend/internal/storage/store.go`
  - Added `ProviderKey` lifecycle methods.
  - Added active provider credential resolution.
- `backend/internal/router/model_router.go`
  - Provider key resolution now checks DB active credential first, then env vars.
- `backend/internal/gateway/router.go`
  - Added Admin provider credential endpoints.
- `frontend/src/lib/api.ts`
  - Added provider credential API client types and calls.
- `frontend/src/pages/Admin.tsx`
  - Added Provider Credentials panel in Admin Models.
- `scripts/regression/provider-credentials.sh`
  - Added executable regression.

## Admin API

```text
GET    /api/admin/providers/:id/keys
POST   /api/admin/providers/:id/keys
DELETE /api/admin/providers/:id/keys/:key_id
```

## Test Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/provider-credentials.sh
```

## Result

```text
Total: 11
Passed: 11
Failed: 0
```

## Covered Test Cases

| Case | Target | Expected | Result |
|---|---|---|---|
| TC-REG-PROVIDER-CRED-001 | Health | `/health` returns `status=ok` | Pass |
| TC-REG-PROVIDER-CRED-002 | Admin setup | Register, promote, and login admin | Pass |
| TC-REG-PROVIDER-CRED-003 | Create provider | OpenAI-compatible provider created | Pass |
| TC-REG-PROVIDER-CRED-004 | Create credential | Response returns id and masked key only | Pass |
| TC-REG-PROVIDER-CRED-005 | List credential | List response hides raw secret | Pass |
| TC-REG-PROVIDER-CRED-006 | DB persistence | `provider_keys.key_ref` stored as `local:v1:*`, not raw secret | Pass |
| TC-REG-PROVIDER-CRED-007 | Revoke credential | Revoke API returns success | Pass |
| TC-REG-PROVIDER-CRED-008 | DB revoke | credential `is_active=false` | Pass |
| TC-REG-PROVIDER-CRED-009 | Audit | create and revoke audit events recorded | Pass |

## Additional Verification

```bash
source scripts/dev-env.sh && cd backend && go test ./...
npm run build
source scripts/dev-env.sh && ./scripts/dev-check.sh
BASE_URL=http://localhost:8081 scripts/regression/admin-foundation.sh
```

All commands passed on 2026-06-26.

## Remaining Enhancements

- This baseline is platform/admin-managed provider credentials, not full per-customer BYOK yet.
- `local:v1` secret sealing is suitable only for local baseline; production should move to KMS/Vault envelope encryption.
- Anthropic native API still needs a dedicated adapter because it is not fully OpenAI-compatible.
- OpenAI/Grok can be registered through the OpenAI-compatible provider path once their base URLs and model bindings are configured.
